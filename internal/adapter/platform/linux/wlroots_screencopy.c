#include "wlroots_screencopy.h"

#include "common_defs.h"
#include "screencapture.h"
#include "wlr_protocol/screencopy.h"
#include "wlr_protocol/xdg-output.h"

#include <errno.h>
#include <poll.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/syscall.h>
#include <time.h>
#include <unistd.h>
#include <wayland-client.h>

// The manager is bound at version 1 on purpose. Versions 2 and 3 add damage
// tracking and dmabuf negotiation, which a one-shot shm grab has no use for,
// and version 1 guarantees the "buffer" event without the buffer_done handshake
// — a smaller state machine for the same pixels.
#define NERU_SCREENCOPY_MANAGER_VERSION 1

// Only reached when a caller passes a non-positive budget; the Go wrapper
// always passes screenCaptureTimeoutMS.
#define NERU_SCREENCOPY_FALLBACK_TIMEOUT_MS 2000

// Largest frame dimension accepted from the compositor. Far above any real
// display, and small enough that width * height * 4 cannot overflow.
#define NERU_SCREENCOPY_MAX_DIMENSION 32768

typedef struct {
	struct wl_output *output;
	struct zxdg_output_v1 *xdg_output;
	int x;
	int y;
	int w;
	int h;
	int has_position;
	int has_size;
} NeruScreencopyOutput;

typedef struct {
	struct wl_display *display;
	struct wl_registry *registry;
	struct wl_shm *shm;
	struct zxdg_output_manager_v1 *xdg_output_mgr;
	struct zwlr_screencopy_manager_v1 *screencopy_mgr;

	NeruScreencopyOutput outputs[NERU_MAX_OUTPUTS];
	int nr_outputs;

	// Frame state, all written from listener callbacks on this thread.
	uint32_t format;
	uint32_t width;
	uint32_t height;
	uint32_t stride;
	int has_buffer;
	int y_invert;
	int ready;
	int failed;
} NeruScreencopyCtx;

static int64_t neru_screencopy_now_ms(void) {
	struct timespec ts;

	clock_gettime(CLOCK_MONOTONIC, &ts);

	return (int64_t)ts.tv_sec * 1000 + (int64_t)ts.tv_nsec / 1000000;
}

// wl_output, zxdg_output_v1 and the frame all send events this file has no use
// for, but a listener with a NULL slot is a crash waiting for the compositor
// that sends it. Every event therefore gets a correctly typed handler and the
// unused ones do nothing — casting one shared no-op into each slot would be an
// incompatible function-pointer call, which is undefined behavior.
static void neru_screencopy_xdg_done(void *data, struct zxdg_output_v1 *xdg_output) {
	(void)data;
	(void)xdg_output;
}

static void neru_screencopy_xdg_string(void *data, struct zxdg_output_v1 *xdg_output, const char *value) {
	(void)data;
	(void)xdg_output;
	(void)value;
}

static void neru_screencopy_output_geometry(
    void *data, struct wl_output *output, int32_t x, int32_t y, int32_t physical_width, int32_t physical_height,
    int32_t subpixel, const char *make, const char *model, int32_t transform) {
	(void)data;
	(void)output;
	(void)x;
	(void)y;
	(void)physical_width;
	(void)physical_height;
	(void)subpixel;
	(void)make;
	(void)model;
	(void)transform;
}

static void neru_screencopy_output_mode(
    void *data, struct wl_output *output, uint32_t flags, int32_t width, int32_t height, int32_t refresh) {
	(void)data;
	(void)output;
	(void)flags;
	(void)width;
	(void)height;
	(void)refresh;
}

static void neru_screencopy_output_done(void *data, struct wl_output *output) {
	(void)data;
	(void)output;
}

static void neru_screencopy_output_scale(void *data, struct wl_output *output, int32_t factor) {
	(void)data;
	(void)output;
	(void)factor;
}

static void neru_screencopy_xdg_logical_position(void *data, struct zxdg_output_v1 *xdg_output, int32_t x, int32_t y) {
	(void)xdg_output;

	NeruScreencopyOutput *out = (NeruScreencopyOutput *)data;

	out->x = x;
	out->y = y;
	out->has_position = 1;
}

static void neru_screencopy_xdg_logical_size(void *data, struct zxdg_output_v1 *xdg_output, int32_t w, int32_t h) {
	(void)xdg_output;

	NeruScreencopyOutput *out = (NeruScreencopyOutput *)data;

	out->w = w;
	out->h = h;
	out->has_size = 1;
}

static const struct zxdg_output_v1_listener neru_screencopy_xdg_output_listener = {
    .logical_position = neru_screencopy_xdg_logical_position,
    .logical_size = neru_screencopy_xdg_logical_size,
    .done = neru_screencopy_xdg_done,
    .name = neru_screencopy_xdg_string,
    .description = neru_screencopy_xdg_string,
};

static const struct wl_output_listener neru_screencopy_output_listener = {
    .geometry = neru_screencopy_output_geometry,
    .mode = neru_screencopy_output_mode,
    .done = neru_screencopy_output_done,
    .scale = neru_screencopy_output_scale,
};

static void neru_screencopy_frame_buffer(
    void *data, struct zwlr_screencopy_frame_v1 *frame, uint32_t format, uint32_t width, uint32_t height,
    uint32_t stride) {
	(void)frame;

	NeruScreencopyCtx *ctx = (NeruScreencopyCtx *)data;

	if (ctx->has_buffer) {
		// Version 1 sends exactly one buffer event; keep the first regardless.
		return;
	}

	ctx->format = format;
	ctx->width = width;
	ctx->height = height;
	ctx->stride = stride;
	ctx->has_buffer = 1;
}

static void neru_screencopy_frame_flags(void *data, struct zwlr_screencopy_frame_v1 *frame, uint32_t flags) {
	(void)frame;

	NeruScreencopyCtx *ctx = (NeruScreencopyCtx *)data;

	ctx->y_invert = (flags & ZWLR_SCREENCOPY_FRAME_V1_FLAGS_Y_INVERT) != 0;
}

static void neru_screencopy_frame_ready(
    void *data, struct zwlr_screencopy_frame_v1 *frame, uint32_t tv_sec_hi, uint32_t tv_sec_lo, uint32_t tv_nsec) {
	(void)frame;
	(void)tv_sec_hi;
	(void)tv_sec_lo;
	(void)tv_nsec;

	((NeruScreencopyCtx *)data)->ready = 1;
}

static void neru_screencopy_frame_failed(void *data, struct zwlr_screencopy_frame_v1 *frame) {
	(void)frame;

	((NeruScreencopyCtx *)data)->failed = 1;
}

static void neru_screencopy_frame_damage(
    void *data, struct zwlr_screencopy_frame_v1 *frame, uint32_t x, uint32_t y, uint32_t width, uint32_t height) {
	(void)data;
	(void)frame;
	(void)x;
	(void)y;
	(void)width;
	(void)height;
}

static void neru_screencopy_frame_linux_dmabuf(
    void *data, struct zwlr_screencopy_frame_v1 *frame, uint32_t format, uint32_t width, uint32_t height) {
	(void)data;
	(void)frame;
	(void)format;
	(void)width;
	(void)height;
}

static void neru_screencopy_frame_buffer_done(void *data, struct zwlr_screencopy_frame_v1 *frame) {
	(void)data;
	(void)frame;
}

static const struct zwlr_screencopy_frame_v1_listener neru_screencopy_frame_listener = {
    .buffer = neru_screencopy_frame_buffer,
    .flags = neru_screencopy_frame_flags,
    .ready = neru_screencopy_frame_ready,
    .failed = neru_screencopy_frame_failed,
    .damage = neru_screencopy_frame_damage,
    .linux_dmabuf = neru_screencopy_frame_linux_dmabuf,
    .buffer_done = neru_screencopy_frame_buffer_done,
};

static void neru_screencopy_registry_global(
    void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version) {
	NeruScreencopyCtx *ctx = (NeruScreencopyCtx *)data;

	if (strcmp(interface, "wl_shm") == 0) {
		ctx->shm = wl_registry_bind(registry, name, &wl_shm_interface, 1);
	} else if (strcmp(interface, "zwlr_screencopy_manager_v1") == 0) {
		ctx->screencopy_mgr =
		    wl_registry_bind(registry, name, &zwlr_screencopy_manager_v1_interface, NERU_SCREENCOPY_MANAGER_VERSION);
	} else if (strcmp(interface, "zxdg_output_manager_v1") == 0) {
		ctx->xdg_output_mgr =
		    wl_registry_bind(registry, name, &zxdg_output_manager_v1_interface, 3 < version ? 3 : version);
	} else if (strcmp(interface, "wl_output") == 0) {
		if (ctx->nr_outputs >= NERU_MAX_OUTPUTS) {
			return;
		}

		NeruScreencopyOutput *out = &ctx->outputs[ctx->nr_outputs];

		out->output = wl_registry_bind(registry, name, &wl_output_interface, 3 < version ? 3 : version);
		wl_output_add_listener(out->output, &neru_screencopy_output_listener, out);
		ctx->nr_outputs++;
	}
}

static void neru_screencopy_registry_global_remove(void *data, struct wl_registry *registry, uint32_t name) {
	(void)data;
	(void)registry;
	(void)name;
	// The connection lives for one capture; a hotplug mid-capture surfaces as a
	// failed frame rather than needing bookkeeping here.
}

static const struct wl_registry_listener neru_screencopy_registry_listener = {
    .global = neru_screencopy_registry_global,
    .global_remove = neru_screencopy_registry_global_remove,
};

// neru_screencopy_wait dispatches until *flag is set, the frame failed, or the
// deadline passes. Single-threaded: this connection has no dispatch thread, so
// prepare_read/read_events bookkeeping is not needed.
static int neru_screencopy_wait(NeruScreencopyCtx *ctx, const int *flag, int64_t deadline) {
	struct pollfd pfd = {.fd = wl_display_get_fd(ctx->display), .events = POLLIN, .revents = 0};

	while (!*flag && !ctx->failed) {
		if (wl_display_dispatch_pending(ctx->display) < 0) {
			return 0;
		}

		if (*flag || ctx->failed) {
			break;
		}

		if (wl_display_flush(ctx->display) < 0 && errno != EAGAIN) {
			return 0;
		}

		int remaining = (int)(deadline - neru_screencopy_now_ms());
		if (remaining <= 0) {
			return 0;
		}

		int rc = poll(&pfd, 1, remaining);
		if (rc < 0) {
			if (errno == EINTR) {
				continue;
			}

			return 0;
		}

		if (rc == 0) {
			return 0;
		}

		if (wl_display_dispatch(ctx->display) < 0) {
			return 0;
		}
	}

	return *flag ? 1 : 0;
}

// The sync callback only flips a flag; the caller owns the wl_callback and
// destroys it either way, so a timed-out roundtrip cannot leave a listener
// pointing at a stack variable that has gone.
static void neru_screencopy_sync_done(void *data, struct wl_callback *callback, uint32_t serial) {
	(void)callback;
	(void)serial;

	*(int *)data = 1;
}

static const struct wl_callback_listener neru_screencopy_sync_listener = {
    .done = neru_screencopy_sync_done,
};

// neru_screencopy_roundtrip is wl_display_roundtrip with the caller's deadline
// on it. The library version blocks forever, which would let a compositor that
// stops answering during discovery hang the capture well past the budget Go
// asked for — the one thing the timeout exists to prevent.
static int neru_screencopy_roundtrip(NeruScreencopyCtx *ctx, int64_t deadline) {
	int done = 0;

	struct wl_callback *callback = wl_display_sync(ctx->display);
	if (!callback) {
		return 0;
	}

	wl_callback_add_listener(callback, &neru_screencopy_sync_listener, &done);

	int ok = neru_screencopy_wait(ctx, &done, deadline);

	wl_callback_destroy(callback);

	return ok;
}

static int neru_screencopy_shm_file(size_t size) {
	int fd = (int)syscall(__NR_memfd_create, "neru-screencopy-shm", 0);
	if (fd < 0) {
		return -1;
	}

	int rc;

	do {
		rc = ftruncate(fd, (off_t)size);
	} while (rc < 0 && errno == EINTR);

	if (rc < 0) {
		close(fd);

		return -1;
	}

	return fd;
}

// neru_screencopy_select_output finds the output that wholly contains the
// requested region and writes the region in that output's local logical
// coordinates. Returns NULL when no single output contains it.
//
// screencopy captures one output, so a region spanning two monitors could only
// be answered by cropping — which would hand back a frame whose top-left is not
// the caller's, with nothing in the result to say so. Refusing keeps the
// invariant that what comes back covers exactly what was asked for; stitching
// several outputs is work no caller needs yet.
static NeruScreencopyOutput *neru_screencopy_select_output(
    NeruScreencopyCtx *ctx, int x, int y, int w, int h, int *local_x, int *local_y) {
	for (int i = 0; i < ctx->nr_outputs; i++) {
		NeruScreencopyOutput *out = &ctx->outputs[i];

		if (!out->has_position || !out->has_size || out->w <= 0 || out->h <= 0) {
			continue;
		}

		if (x < out->x || y < out->y || x + w > out->x + out->w || y + h > out->y + out->h) {
			continue;
		}

		*local_x = x - out->x;
		*local_y = y - out->y;

		return out;
	}

	return NULL;
}

// neru_screencopy_convert turns the mapped shm buffer into packed RGBA8888.
static int neru_screencopy_convert(const NeruScreencopyCtx *ctx, const unsigned char *src, NeruCapture *out) {
	int red_offset;
	int blue_offset;

	switch (ctx->format) {
	case WL_SHM_FORMAT_ARGB8888:
	case WL_SHM_FORMAT_XRGB8888:
		// Little-endian 0xAARRGGBB: bytes are B, G, R, A.
		red_offset = 2;
		blue_offset = 0;

		break;
	case WL_SHM_FORMAT_ABGR8888:
	case WL_SHM_FORMAT_XBGR8888:
		// Little-endian 0xAABBGGRR: bytes are R, G, B, A.
		red_offset = 0;
		blue_offset = 2;

		break;
	default:
		return NERU_CAPTURE_ERR_FORMAT;
	}

	int width = (int)ctx->width;
	int height = (int)ctx->height;

	unsigned char *pixels = malloc((size_t)width * (size_t)height * 4u);
	if (!pixels) {
		return NERU_CAPTURE_ERR_ALLOC;
	}

	for (int row = 0; row < height; row++) {
		int source_row = ctx->y_invert ? (height - 1 - row) : row;
		const unsigned char *line = src + (size_t)source_row * (size_t)ctx->stride;
		unsigned char *dst = pixels + (size_t)row * (size_t)width * 4u;

		for (int col = 0; col < width; col++) {
			dst[0] = line[red_offset];
			dst[1] = line[1];
			dst[2] = line[blue_offset];
			dst[3] = 0xFF;
			dst += 4;
			line += 4;
		}
	}

	out->pixels = pixels;
	out->width = width;
	out->height = height;

	return NERU_CAPTURE_OK;
}

static void neru_screencopy_teardown(NeruScreencopyCtx *ctx) {
	for (int i = 0; i < ctx->nr_outputs; i++) {
		if (ctx->outputs[i].xdg_output) {
			zxdg_output_v1_destroy(ctx->outputs[i].xdg_output);
		}

		if (ctx->outputs[i].output) {
			wl_output_destroy(ctx->outputs[i].output);
		}
	}

	if (ctx->screencopy_mgr) {
		zwlr_screencopy_manager_v1_destroy(ctx->screencopy_mgr);
	}

	if (ctx->xdg_output_mgr) {
		zxdg_output_manager_v1_destroy(ctx->xdg_output_mgr);
	}

	if (ctx->shm) {
		wl_shm_destroy(ctx->shm);
	}

	if (ctx->registry) {
		wl_registry_destroy(ctx->registry);
	}

	if (ctx->display) {
		wl_display_disconnect(ctx->display);
	}
}

// neru_screencopy_copy_frame runs the copy half: allocate the shm buffer the
// compositor asked for, hand it over, wait for the result, convert.
static int neru_screencopy_copy_frame(
    NeruScreencopyCtx *ctx, struct zwlr_screencopy_frame_v1 *frame, int64_t deadline, NeruCapture *out) {
	// Frame metadata arrives from the compositor, so it is validated rather than
	// trusted: the dimensions are capped and every product is computed in 64
	// bits. Without that, a width near UINT32_MAX makes width * 4 wrap, the
	// stride check passes, and the conversion loop then reads past a mapping
	// sized from the wrapped value.
	if (ctx->width == 0 || ctx->height == 0 || ctx->width > NERU_SCREENCOPY_MAX_DIMENSION ||
	    ctx->height > NERU_SCREENCOPY_MAX_DIMENSION) {
		return NERU_CAPTURE_ERR_FORMAT;
	}

	if ((uint64_t)ctx->stride < (uint64_t)ctx->width * 4u) {
		return NERU_CAPTURE_ERR_FORMAT;
	}

	uint64_t mapping = (uint64_t)ctx->stride * (uint64_t)ctx->height;
	if (mapping > (uint64_t)SIZE_MAX) {
		return NERU_CAPTURE_ERR_FORMAT;
	}

	size_t size = (size_t)mapping;

	int fd = neru_screencopy_shm_file(size);
	if (fd < 0) {
		return NERU_CAPTURE_ERR_ALLOC;
	}

	unsigned char *data = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
	if (data == MAP_FAILED) {
		close(fd);

		return NERU_CAPTURE_ERR_ALLOC;
	}

	struct wl_shm_pool *pool = wl_shm_create_pool(ctx->shm, fd, (int32_t)size);
	struct wl_buffer *buffer = wl_shm_pool_create_buffer(
	    pool, 0, (int32_t)ctx->width, (int32_t)ctx->height, (int32_t)ctx->stride, ctx->format);

	wl_shm_pool_destroy(pool);
	close(fd);

	zwlr_screencopy_frame_v1_copy(frame, buffer);

	int status;

	if (!neru_screencopy_wait(ctx, &ctx->ready, deadline)) {
		status = ctx->failed ? NERU_CAPTURE_ERR_FAILED : NERU_CAPTURE_ERR_TIMEOUT;
	} else {
		status = neru_screencopy_convert(ctx, data, out);
	}

	wl_buffer_destroy(buffer);

	// The mapping held screen pixels; wipe before unmapping so the pages go
	// back to the kernel blank.
	neru_capture_wipe(data, size);
	munmap(data, size);

	return status;
}

int neru_screencopy_capture_region(int x, int y, int w, int h, int timeout_ms, NeruCapture *out) {
	int begin = neru_capture_begin(out, w, h);
	if (begin != NERU_CAPTURE_OK) {
		return begin;
	}

	if (timeout_ms <= 0) {
		// Go owns the real budget (screenCaptureTimeoutMS); this only keeps a
		// direct C caller from waiting forever.
		timeout_ms = NERU_SCREENCOPY_FALLBACK_TIMEOUT_MS;
	}

	NeruScreencopyCtx ctx;
	memset(&ctx, 0, sizeof(ctx));

	ctx.display = wl_display_connect(NULL);
	if (!ctx.display) {
		return NERU_CAPTURE_ERR_NO_DISPLAY;
	}

	int64_t deadline = neru_screencopy_now_ms() + timeout_ms;

	ctx.registry = wl_display_get_registry(ctx.display);
	wl_registry_add_listener(ctx.registry, &neru_screencopy_registry_listener, &ctx);

	if (!neru_screencopy_roundtrip(&ctx, deadline)) {
		neru_screencopy_teardown(&ctx);

		return NERU_CAPTURE_ERR_TIMEOUT;
	}

	if (!ctx.screencopy_mgr || !ctx.shm) {
		neru_screencopy_teardown(&ctx);

		return NERU_CAPTURE_ERR_NO_PROTOCOL;
	}

	if (!ctx.xdg_output_mgr || ctx.nr_outputs == 0) {
		neru_screencopy_teardown(&ctx);

		return NERU_CAPTURE_ERR_NO_OUTPUT;
	}

	for (int i = 0; i < ctx.nr_outputs; i++) {
		ctx.outputs[i].xdg_output = zxdg_output_manager_v1_get_xdg_output(ctx.xdg_output_mgr, ctx.outputs[i].output);
		zxdg_output_v1_add_listener(ctx.outputs[i].xdg_output, &neru_screencopy_xdg_output_listener, &ctx.outputs[i]);
	}

	if (!neru_screencopy_roundtrip(&ctx, deadline)) {
		neru_screencopy_teardown(&ctx);

		return NERU_CAPTURE_ERR_TIMEOUT;
	}

	int local_x = 0;
	int local_y = 0;

	NeruScreencopyOutput *target = neru_screencopy_select_output(&ctx, x, y, w, h, &local_x, &local_y);
	if (!target) {
		neru_screencopy_teardown(&ctx);

		return NERU_CAPTURE_ERR_NO_OUTPUT;
	}

	struct zwlr_screencopy_frame_v1 *frame =
	    zwlr_screencopy_manager_v1_capture_output_region(ctx.screencopy_mgr, 0, target->output, local_x, local_y, w, h);

	zwlr_screencopy_frame_v1_add_listener(frame, &neru_screencopy_frame_listener, &ctx);

	int status;

	if (!neru_screencopy_wait(&ctx, &ctx.has_buffer, deadline)) {
		status = ctx.failed ? NERU_CAPTURE_ERR_FAILED : NERU_CAPTURE_ERR_TIMEOUT;
	} else {
		status = neru_screencopy_copy_frame(&ctx, frame, deadline, out);
	}

	zwlr_screencopy_frame_v1_destroy(frame);
	neru_screencopy_teardown(&ctx);

	return status;
}

#include "pipewire_capture.h"

#include "screencapture.h"

#include <pipewire/pipewire.h>
#include <pthread.h>
#include <spa/param/video/format-utils.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

// Largest frame dimension accepted from the compositor. Far above any real
// display, and small enough that width * height * 4 cannot overflow.
#define NERU_PIPEWIRE_MAX_DIMENSION 32768

// Only reached for a non-positive budget, since the Go wrapper passes at least
// 1 ms. Never zero, because a zero value disarms a PipeWire timer.
#define NERU_PIPEWIRE_FALLBACK_TIMEOUT_MS 2000

#define NERU_PIPEWIRE_NSEC_PER_MSEC 1000000L
#define NERU_PIPEWIRE_MSEC_PER_SEC 1000

// How many values the format choice carries: the four layouts a portal
// ScreenCast stream offers, plus the leading default copy a SPA enum choice
// needs. xRGB/ARGB is not negotiated, because its green channel sits at
// another offset and nothing sends it.
#define NERU_PIPEWIRE_FORMAT_CHOICES 5

typedef struct {
	struct pw_main_loop *loop;
	struct pw_context *context;
	struct pw_core *core;
	struct pw_stream *stream;
	struct spa_hook stream_listener;

	struct spa_video_info_raw format;
	int have_format;

	const NeruPipewireRequest *request;
	NeruCapture *out;

	// status is what the capture will return, and finished says the loop has an
	// answer. Both are written only from listener callbacks, which run on the
	// loop's own thread.
	int status;
	int finished;
} NeruPipewireCtx;

// pw_init is process-global and not concurrency-safe, so it runs once. There is
// no pw_deinit, which would tear the library down under a running capture.
static pthread_once_t neru_pipewire_init_once = PTHREAD_ONCE_INIT;

static void neru_pipewire_init(void) { pw_init(NULL, NULL); }

// neru_pipewire_finish records an answer and stops the loop. The first answer
// wins. A stream error arriving after a frame was already converted must not
// turn a good capture into a failure.
static void neru_pipewire_finish(NeruPipewireCtx *ctx, int status) {
	if (ctx->finished) {
		return;
	}

	ctx->status = status;
	ctx->finished = 1;

	pw_main_loop_quit(ctx->loop);
}

// neru_pipewire_channel_offsets maps a negotiated SPA format onto the byte
// offsets of red and blue inside each 4-byte pixel. Green is always at offset 1
// for the formats accepted here, which is why it is not returned.
static int neru_pipewire_channel_offsets(uint32_t format, int *red_offset, int *blue_offset) {
	switch (format) {
	case SPA_VIDEO_FORMAT_BGRx:
	case SPA_VIDEO_FORMAT_BGRA:
		// Bytes are B, G, R, A.
		*red_offset = 2;
		*blue_offset = 0;

		return 1;
	case SPA_VIDEO_FORMAT_RGBx:
	case SPA_VIDEO_FORMAT_RGBA:
		// Bytes are R, G, B, A.
		*red_offset = 0;
		*blue_offset = 2;

		return 1;
	default:
		return 0;
	}
}

// neru_pipewire_scale maps one logical coordinate onto the physical grid,
// rounding to nearest. Coordinates are monitor-local and never negative.
static int64_t neru_pipewire_scale(int value, int physical, int logical) {
	if (logical <= 0) {
		return value;
	}

	return ((int64_t)value * (int64_t)physical + (int64_t)logical / 2) / (int64_t)logical;
}

// neru_pipewire_crop turns the caller's logical rectangle into physical pixels
// inside the frame.
//
// A scaled output streams more pixels than its logical size, so the ratio is
// the scale. Both edges are scaled and rounded, rather than origin and size, so
// a rectangle ending on the monitor's edge ends on the frame's edge. Integer
// arithmetic keeps the rounding exact. A crop that does not fit is refused
// rather than clamped, since a clipped frame cannot say where its top-left is.
static int neru_pipewire_crop(
    const NeruPipewireRequest *request, int frame_width, int frame_height, int *crop_x, int *crop_y, int *crop_width,
    int *crop_height) {
	int logical_width = request->logical_width > 0 ? request->logical_width : frame_width;
	int logical_height = request->logical_height > 0 ? request->logical_height : frame_height;

	if (request->x < 0 || request->y < 0 || request->w <= 0 || request->h <= 0) {
		return NERU_CAPTURE_ERR_REGION;
	}

	int64_t left = neru_pipewire_scale(request->x, frame_width, logical_width);
	int64_t top = neru_pipewire_scale(request->y, frame_height, logical_height);
	int64_t right = neru_pipewire_scale(request->x + request->w, frame_width, logical_width);
	int64_t bottom = neru_pipewire_scale(request->y + request->h, frame_height, logical_height);

	if (left < 0 || top < 0 || right > frame_width || bottom > frame_height) {
		return NERU_CAPTURE_ERR_REGION;
	}

	if (right <= left || bottom <= top) {
		return NERU_CAPTURE_ERR_REGION;
	}

	*crop_x = (int)left;
	*crop_y = (int)top;
	*crop_width = (int)(right - left);
	*crop_height = (int)(bottom - top);

	return NERU_CAPTURE_OK;
}

// neru_pipewire_convert copies the requested rectangle out of the mapped frame
// as packed RGBA8888.
static int neru_pipewire_convert(NeruPipewireCtx *ctx, const struct spa_buffer *buffer) {
	int frame_width = (int)ctx->format.size.width;
	int frame_height = (int)ctx->format.size.height;

	// Frame metadata comes from the compositor, so the dimensions are capped and
	// every product is computed in 64 bits, so no size can wrap.
	if (frame_width <= 0 || frame_height <= 0 || frame_width > NERU_PIPEWIRE_MAX_DIMENSION ||
	    frame_height > NERU_PIPEWIRE_MAX_DIMENSION) {
		return NERU_CAPTURE_ERR_FORMAT;
	}

	int red_offset = 0;
	int blue_offset = 0;

	if (!neru_pipewire_channel_offsets(ctx->format.format, &red_offset, &blue_offset)) {
		return NERU_CAPTURE_ERR_FORMAT;
	}

	int32_t stride = buffer->datas[0].chunk->stride;
	if ((int64_t)stride < (int64_t)frame_width * 4) {
		return NERU_CAPTURE_ERR_FORMAT;
	}

	// The pixels start at chunk->offset, and offset and stride come from the
	// compositor. Everything the loop touches must fit inside maxsize, checked
	// in 64 bits so no product can wrap into a passing check.
	int64_t offset = (int64_t)buffer->datas[0].chunk->offset;
	if (offset < 0) {
		return NERU_CAPTURE_ERR_FORMAT;
	}

	if (offset + (int64_t)stride * (int64_t)frame_height > (int64_t)buffer->datas[0].maxsize) {
		return NERU_CAPTURE_ERR_FORMAT;
	}

	int crop_x = 0;
	int crop_y = 0;
	int crop_width = 0;
	int crop_height = 0;

	int cropped =
	    neru_pipewire_crop(ctx->request, frame_width, frame_height, &crop_x, &crop_y, &crop_width, &crop_height);
	if (cropped != NERU_CAPTURE_OK) {
		return cropped;
	}

	unsigned char *pixels = malloc((size_t)crop_width * (size_t)crop_height * 4u);
	if (!pixels) {
		return NERU_CAPTURE_ERR_ALLOC;
	}

	const unsigned char *source = (const unsigned char *)buffer->datas[0].data + offset;

	for (int row = 0; row < crop_height; row++) {
		const unsigned char *line = source + (size_t)(crop_y + row) * (size_t)stride + (size_t)crop_x * 4u;
		unsigned char *destination = pixels + (size_t)row * (size_t)crop_width * 4u;

		for (int column = 0; column < crop_width; column++) {
			destination[0] = line[red_offset];
			destination[1] = line[1];
			destination[2] = line[blue_offset];
			destination[3] = 0xFF;
			destination += 4;
			line += 4;
		}
	}

	ctx->out->pixels = pixels;
	ctx->out->width = crop_width;
	ctx->out->height = crop_height;

	return NERU_CAPTURE_OK;
}

static void neru_pipewire_on_param_changed(void *data, uint32_t id, const struct spa_pod *param) {
	NeruPipewireCtx *ctx = (NeruPipewireCtx *)data;

	if (param == NULL || id != SPA_PARAM_Format) {
		return;
	}

	uint32_t media_type = 0;
	uint32_t media_subtype = 0;

	if (spa_format_parse(param, &media_type, &media_subtype) < 0) {
		return;
	}

	if (media_type != SPA_MEDIA_TYPE_video || media_subtype != SPA_MEDIA_SUBTYPE_raw) {
		return;
	}

	if (spa_format_video_raw_parse(param, &ctx->format) < 0) {
		neru_pipewire_finish(ctx, NERU_CAPTURE_ERR_FORMAT);

		return;
	}

	ctx->have_format = 1;
}

static void neru_pipewire_on_process(void *data) {
	NeruPipewireCtx *ctx = (NeruPipewireCtx *)data;

	// Enforced here, not only in neru_pipewire_finish. Converting a second frame
	// would overwrite out->pixels and strand the first allocation unwiped, the
	// one way screen content could survive in freed heap.
	if (ctx->finished) {
		return;
	}

	struct pw_buffer *pw_buf = pw_stream_dequeue_buffer(ctx->stream);
	if (pw_buf == NULL) {
		return;
	}

	struct spa_buffer *buffer = pw_buf->buffer;

	int status = NERU_CAPTURE_ERR_FAILED;

	if (!ctx->have_format) {
		// A frame before the format was parsed cannot be read. Wait for the next
		// one rather than failing, since param_changed normally precedes process.
		pw_stream_queue_buffer(ctx->stream, pw_buf);

		return;
	}

	if (buffer->n_datas == 0 || buffer->datas[0].data == NULL) {
		// PW_STREAM_FLAG_MAP_BUFFERS maps MemFd and MemPtr buffers, and the
		// negotiated dataType excludes everything else, so a null mapping means
		// the compositor sent a buffer kind this backend cannot read.
		status = NERU_CAPTURE_ERR_FORMAT;
	} else if (buffer->datas[0].chunk == NULL || buffer->datas[0].chunk->size == 0) {
		// An empty chunk is a frame the compositor had nothing new for, and the next
		// one will carry pixels.
		pw_stream_queue_buffer(ctx->stream, pw_buf);

		return;
	} else {
		status = neru_pipewire_convert(ctx, buffer);
	}

	pw_stream_queue_buffer(ctx->stream, pw_buf);

	neru_pipewire_finish(ctx, status);
}

static void neru_pipewire_on_state_changed(
    void *data, enum pw_stream_state old_state, enum pw_stream_state state, const char *error) {
	(void)old_state;
	(void)error;

	NeruPipewireCtx *ctx = (NeruPipewireCtx *)data;

	// A stream that errors or disconnects before a frame will never produce
	// one, so answer now rather than wait out the budget.
	if (state == PW_STREAM_STATE_ERROR || state == PW_STREAM_STATE_UNCONNECTED) {
		neru_pipewire_finish(ctx, NERU_CAPTURE_ERR_FAILED);
	}
}

static const struct pw_stream_events neru_pipewire_stream_events = {
    PW_VERSION_STREAM_EVENTS,
    .state_changed = neru_pipewire_on_state_changed,
    .param_changed = neru_pipewire_on_param_changed,
    .process = neru_pipewire_on_process,
};

static void neru_pipewire_on_timeout(void *data, uint64_t expirations) {
	(void)expirations;

	neru_pipewire_finish((NeruPipewireCtx *)data, NERU_CAPTURE_ERR_TIMEOUT);
}

// neru_pipewire_arm_timeout puts the caller's budget on the loop, so a
// compositor that accepts the stream and then sends nothing surfaces as a
// timed-out capture instead of hanging the caller.
static struct spa_source *neru_pipewire_arm_timeout(NeruPipewireCtx *ctx, int timeout_ms) {
	struct pw_loop *loop = pw_main_loop_get_loop(ctx->loop);

	struct spa_source *timer = pw_loop_add_timer(loop, neru_pipewire_on_timeout, ctx);
	if (timer == NULL) {
		return NULL;
	}

	struct timespec value = {
	    .tv_sec = timeout_ms / NERU_PIPEWIRE_MSEC_PER_SEC,
	    .tv_nsec = (long)(timeout_ms % NERU_PIPEWIRE_MSEC_PER_SEC) * NERU_PIPEWIRE_NSEC_PER_MSEC,
	};

	pw_loop_update_timer(loop, timer, &value, NULL, false);

	return timer;
}

// neru_pipewire_connect_stream asks for the node the portal named. The format
// enumeration lists only the layouts the conversion handles, so others are
// refused during negotiation. The buffer param restricts data to shared memory,
// keeping out dmabuf, which would need a GPU context for one memcpy.
static int neru_pipewire_connect_stream(NeruPipewireCtx *ctx, unsigned int node_id) {
	uint8_t builder_buffer[1024];
	struct spa_pod_builder builder = SPA_POD_BUILDER_INIT(builder_buffer, sizeof(builder_buffer));

	const struct spa_pod *params[2];

	params[0] = spa_pod_builder_add_object(
	    &builder, SPA_TYPE_OBJECT_Format, SPA_PARAM_EnumFormat, SPA_FORMAT_mediaType, SPA_POD_Id(SPA_MEDIA_TYPE_video),
	    SPA_FORMAT_mediaSubtype, SPA_POD_Id(SPA_MEDIA_SUBTYPE_raw), SPA_FORMAT_VIDEO_format,
	    SPA_POD_CHOICE_ENUM_Id(
	        NERU_PIPEWIRE_FORMAT_CHOICES, SPA_VIDEO_FORMAT_BGRx, SPA_VIDEO_FORMAT_BGRx, SPA_VIDEO_FORMAT_BGRA,
	        SPA_VIDEO_FORMAT_RGBx, SPA_VIDEO_FORMAT_RGBA),
	    SPA_FORMAT_VIDEO_size,
	    SPA_POD_CHOICE_RANGE_Rectangle(
	        &SPA_RECTANGLE(1920, 1080), &SPA_RECTANGLE(1, 1),
	        &SPA_RECTANGLE(NERU_PIPEWIRE_MAX_DIMENSION, NERU_PIPEWIRE_MAX_DIMENSION)),
	    SPA_FORMAT_VIDEO_framerate,
	    SPA_POD_CHOICE_RANGE_Fraction(&SPA_FRACTION(30, 1), &SPA_FRACTION(0, 1), &SPA_FRACTION(1000, 1)));

	params[1] = spa_pod_builder_add_object(
	    &builder, SPA_TYPE_OBJECT_ParamBuffers, SPA_PARAM_Buffers, SPA_PARAM_BUFFERS_dataType,
	    SPA_POD_CHOICE_FLAGS_Int((1 << SPA_DATA_MemFd) | (1 << SPA_DATA_MemPtr)));

	return pw_stream_connect(
	    ctx->stream, PW_DIRECTION_INPUT, (uint32_t)node_id, PW_STREAM_FLAG_AUTOCONNECT | PW_STREAM_FLAG_MAP_BUFFERS,
	    params, 2);
}

static void neru_pipewire_teardown(NeruPipewireCtx *ctx) {
	if (ctx->stream) {
		pw_stream_destroy(ctx->stream);
	}

	if (ctx->core) {
		pw_core_disconnect(ctx->core);
	}

	if (ctx->context) {
		pw_context_destroy(ctx->context);
	}

	if (ctx->loop) {
		pw_main_loop_destroy(ctx->loop);
	}
}

int neru_pipewire_capture(const NeruPipewireRequest *request, NeruCapture *out) {
	if (!request) {
		return NERU_CAPTURE_ERR_REGION;
	}

	int begin = neru_capture_begin(out, request->w, request->h);
	if (begin != NERU_CAPTURE_OK) {
		if (request->fd >= 0) {
			close(request->fd);
		}

		return begin;
	}

	if (request->fd < 0) {
		return NERU_CAPTURE_ERR_NO_DISPLAY;
	}

	pthread_once(&neru_pipewire_init_once, neru_pipewire_init);

	NeruPipewireCtx ctx;
	memset(&ctx, 0, sizeof(ctx));

	ctx.request = request;
	ctx.out = out;
	ctx.status = NERU_CAPTURE_ERR_FAILED;

	ctx.loop = pw_main_loop_new(NULL);
	if (!ctx.loop) {
		close(request->fd);

		return NERU_CAPTURE_ERR_ALLOC;
	}

	ctx.context = pw_context_new(pw_main_loop_get_loop(ctx.loop), NULL, 0);
	if (!ctx.context) {
		close(request->fd);
		neru_pipewire_teardown(&ctx);

		return NERU_CAPTURE_ERR_ALLOC;
	}

	// From here the descriptor belongs to PipeWire, since
	// pw_context_connect_fd closes it on disconnect and on its own failure, so
	// no path below may close it.
	ctx.core = pw_context_connect_fd(ctx.context, request->fd, NULL, 0);
	if (!ctx.core) {
		neru_pipewire_teardown(&ctx);

		return NERU_CAPTURE_ERR_NO_DISPLAY;
	}

	ctx.stream = pw_stream_new(
	    ctx.core, "neru-screen-capture",
	    pw_properties_new(
	        PW_KEY_MEDIA_TYPE, "Video", PW_KEY_MEDIA_CATEGORY, "Capture", PW_KEY_MEDIA_ROLE, "Screen", NULL));
	if (!ctx.stream) {
		neru_pipewire_teardown(&ctx);

		return NERU_CAPTURE_ERR_ALLOC;
	}

	pw_stream_add_listener(ctx.stream, &ctx.stream_listener, &neru_pipewire_stream_events, &ctx);

	int timeout_ms = request->timeout_ms > 0 ? request->timeout_ms : NERU_PIPEWIRE_FALLBACK_TIMEOUT_MS;

	if (neru_pipewire_arm_timeout(&ctx, timeout_ms) == NULL) {
		neru_pipewire_teardown(&ctx);

		return NERU_CAPTURE_ERR_ALLOC;
	}

	if (neru_pipewire_connect_stream(&ctx, request->node_id) < 0) {
		neru_pipewire_teardown(&ctx);

		return NERU_CAPTURE_ERR_NO_PROTOCOL;
	}

	pw_main_loop_run(ctx.loop);

	int status = ctx.status;

	neru_pipewire_teardown(&ctx);

	if (status != NERU_CAPTURE_OK) {
		// A partially filled capture must never reach Go beside an error code.
		neru_capture_free(out);
	}

	return status;
}

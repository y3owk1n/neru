//go:build linux && cgo && integration

package linux

// A Wayland client that records the scroll a compositor delivers to it, so the
// smooth-scroll measurement can assert "a sub-notch delta reaches an
// application" rather than argue it.
//
// It is a non-test file behind the integration tag, and not a _test.go one,
// because Go does not allow cgo in a test file at all — and not a plain file,
// because it is scaffolding and has no business in the shipped binary. The
// generated xdg-shell bindings come from the wlr_protocol package, blank
// imported below so the linker resolves them.

/*
#cgo pkg-config: wayland-client
// _GNU_SOURCE for memfd_create, which is how the shared buffer behind the
// window is allocated without touching the filesystem.
#cgo CFLAGS: -D_GNU_SOURCE

// One xdg-shell toplevel — an ordinary application window, which is the thing
// the claim is about — recording the wl_pointer axis events it is sent. The C
// is inline here rather than in a .c file because every .c file in a package
// compiles into it unconditionally, and scaffolding has no business there.

#include <poll.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <unistd.h>
#include <wayland-client.h>

#include "../../../platform/linux/wlr_protocol/xdg-shell.h"

#define NERU_PROBE_MAX_EVENTS 512

// One delivered scroll frame: the continuous distance, and whether a wheel-step
// count came with it. The second is the half that matters — a step count is the
// compositor saying "one notch", which is exactly what a sub-notch value must
// arrive without.
typedef struct {
	double value;
	int has_step;
	int source;
	int has_source;
} neru_probe_event;

typedef struct {
	struct wl_display *display;
	struct wl_registry *registry;
	struct wl_compositor *compositor;
	struct wl_shm *shm;
	struct xdg_wm_base *wm_base;
	struct wl_seat *seat;
	struct wl_pointer *pointer;
	struct wl_surface *surface;
	struct xdg_surface *xdg_surface;
	struct xdg_toplevel *toplevel;
	int configured;
	int entered;
	int width;
	int height;
	int shm_fd;
	neru_probe_event events[NERU_PROBE_MAX_EVENTS];
	int event_count;
	int pending;
} neru_probe;

static void neru_probe_stop(neru_probe *p);

static void neru_probe_record(neru_probe *p) {
	if (p->pending < 0 && p->event_count < NERU_PROBE_MAX_EVENTS) {
		p->pending = p->event_count++;
		memset(&p->events[p->pending], 0, sizeof(neru_probe_event));
	}
}

static void neru_probe_ping(void *data, struct xdg_wm_base *base, uint32_t serial) {
	(void)data;
	xdg_wm_base_pong(base, serial);
}

static const struct xdg_wm_base_listener neru_probe_wm_base_listener = {
    .ping = neru_probe_ping,
};

static void neru_probe_enter(void *data, struct wl_pointer *pointer, uint32_t serial,
                             struct wl_surface *surface, wl_fixed_t x, wl_fixed_t y) {
	(void)pointer;
	(void)serial;
	(void)surface;
	(void)x;
	(void)y;
	((neru_probe *)data)->entered = 1;
}

static void neru_probe_leave(void *data, struct wl_pointer *pointer, uint32_t serial,
                             struct wl_surface *surface) {
	(void)pointer;
	(void)serial;
	(void)surface;
	((neru_probe *)data)->entered = 0;
}

static void neru_probe_motion(void *data, struct wl_pointer *pointer, uint32_t time, wl_fixed_t x,
                              wl_fixed_t y) {
	(void)data;
	(void)pointer;
	(void)time;
	(void)x;
	(void)y;
}

static void neru_probe_button(void *data, struct wl_pointer *pointer, uint32_t serial,
                              uint32_t time, uint32_t button, uint32_t state) {
	(void)data;
	(void)pointer;
	(void)serial;
	(void)time;
	(void)button;
	(void)state;
}

static void neru_probe_axis(void *data, struct wl_pointer *pointer, uint32_t time, uint32_t axis,
                            wl_fixed_t value) {
	(void)pointer;
	(void)time;
	neru_probe *p = (neru_probe *)data;
	if (axis != WL_POINTER_AXIS_VERTICAL_SCROLL) {
		return;
	}
	neru_probe_record(p);
	if (p->pending >= 0) {
		p->events[p->pending].value = wl_fixed_to_double(value);
	}
}

static void neru_probe_frame(void *data, struct wl_pointer *pointer) {
	(void)pointer;
	((neru_probe *)data)->pending = -1;
}

static void neru_probe_axis_source(void *data, struct wl_pointer *pointer, uint32_t source) {
	(void)pointer;
	neru_probe *p = (neru_probe *)data;
	neru_probe_record(p);
	if (p->pending >= 0) {
		p->events[p->pending].source = (int)source;
		p->events[p->pending].has_source = 1;
	}
}

static void neru_probe_axis_stop(void *data, struct wl_pointer *pointer, uint32_t time,
                                 uint32_t axis) {
	(void)data;
	(void)pointer;
	(void)time;
	(void)axis;
}

static void neru_probe_axis_discrete(void *data, struct wl_pointer *pointer, uint32_t axis,
                                     int32_t discrete) {
	(void)pointer;
	(void)discrete;
	neru_probe *p = (neru_probe *)data;
	if (axis != WL_POINTER_AXIS_VERTICAL_SCROLL) {
		return;
	}
	neru_probe_record(p);
	if (p->pending >= 0) {
		p->events[p->pending].has_step = 1;
	}
}

static void neru_probe_axis_value120(void *data, struct wl_pointer *pointer, uint32_t axis,
                                     int32_t value120) {
	(void)pointer;
	(void)value120;
	neru_probe *p = (neru_probe *)data;
	if (axis != WL_POINTER_AXIS_VERTICAL_SCROLL) {
		return;
	}
	neru_probe_record(p);
	if (p->pending >= 0) {
		p->events[p->pending].has_step = 1;
	}
}

static void neru_probe_axis_relative_direction(void *data, struct wl_pointer *pointer,
                                               uint32_t axis, uint32_t direction) {
	(void)data;
	(void)pointer;
	(void)axis;
	(void)direction;
}

static const struct wl_pointer_listener neru_probe_pointer_listener = {
    .enter = neru_probe_enter,
    .leave = neru_probe_leave,
    .motion = neru_probe_motion,
    .button = neru_probe_button,
    .axis = neru_probe_axis,
    .frame = neru_probe_frame,
    .axis_source = neru_probe_axis_source,
    .axis_stop = neru_probe_axis_stop,
    .axis_discrete = neru_probe_axis_discrete,
    .axis_value120 = neru_probe_axis_value120,
    .axis_relative_direction = neru_probe_axis_relative_direction,
};

static void neru_probe_seat_caps(void *data, struct wl_seat *seat, uint32_t caps) {
	neru_probe *p = (neru_probe *)data;
	if ((caps & WL_SEAT_CAPABILITY_POINTER) && !p->pointer) {
		p->pointer = wl_seat_get_pointer(seat);
		wl_pointer_add_listener(p->pointer, &neru_probe_pointer_listener, p);
	}
}

static void neru_probe_seat_name(void *data, struct wl_seat *seat, const char *name) {
	(void)data;
	(void)seat;
	(void)name;
}

static const struct wl_seat_listener neru_probe_seat_listener = {
    .capabilities = neru_probe_seat_caps,
    .name = neru_probe_seat_name,
};

static void neru_probe_global(void *data, struct wl_registry *registry, uint32_t name,
                              const char *iface, uint32_t version) {
	neru_probe *p = (neru_probe *)data;
	if (strcmp(iface, wl_compositor_interface.name) == 0) {
		p->compositor = wl_registry_bind(registry, name, &wl_compositor_interface, 4);
	} else if (strcmp(iface, wl_shm_interface.name) == 0) {
		p->shm = wl_registry_bind(registry, name, &wl_shm_interface, 1);
	} else if (strcmp(iface, xdg_wm_base_interface.name) == 0) {
		p->wm_base = wl_registry_bind(registry, name, &xdg_wm_base_interface, 1);
		xdg_wm_base_add_listener(p->wm_base, &neru_probe_wm_base_listener, p);
	} else if (strcmp(iface, wl_seat_interface.name) == 0) {
		p->seat = wl_registry_bind(registry, name, &wl_seat_interface, version < 8 ? version : 8);
		wl_seat_add_listener(p->seat, &neru_probe_seat_listener, p);
	}
}

static void neru_probe_global_remove(void *data, struct wl_registry *registry, uint32_t name) {
	(void)data;
	(void)registry;
	(void)name;
}

static const struct wl_registry_listener neru_probe_registry_listener = {
    .global = neru_probe_global,
    .global_remove = neru_probe_global_remove,
};

static void neru_probe_surface_configure(void *data, struct xdg_surface *surface, uint32_t serial) {
	neru_probe *p = (neru_probe *)data;
	xdg_surface_ack_configure(surface, serial);
	p->configured = 1;
}

static const struct xdg_surface_listener neru_probe_surface_listener = {
    .configure = neru_probe_surface_configure,
};

static void neru_probe_toplevel_configure(void *data, struct xdg_toplevel *toplevel, int32_t width,
                                          int32_t height, struct wl_array *states) {
	(void)toplevel;
	(void)states;
	neru_probe *p = (neru_probe *)data;
	if (width > 0 && height > 0) {
		p->width = width;
		p->height = height;
	}
}

static void neru_probe_toplevel_close(void *data, struct xdg_toplevel *toplevel) {
	(void)data;
	(void)toplevel;
}

static const struct xdg_toplevel_listener neru_probe_toplevel_listener = {
    .configure = neru_probe_toplevel_configure,
    .close = neru_probe_toplevel_close,
};

// neru_probe_attach paints the surface at its current size. It is called twice:
// a window is only told the size the compositor tiled it to after it maps, and
// a surface smaller than its container leaves the pointer over the container
// rather than over the window, where no axis event would ever arrive.
static int neru_probe_attach(neru_probe *p) {
	int stride = p->width * 4;
	int size = stride * p->height;
	if (ftruncate(p->shm_fd, size) != 0) {
		return 0;
	}
	void *pixels = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, p->shm_fd, 0);
	if (pixels == MAP_FAILED) {
		return 0;
	}
	memset(pixels, 0xff, size);
	struct wl_shm_pool *pool = wl_shm_create_pool(p->shm, p->shm_fd, size);
	struct wl_buffer *buffer =
	    wl_shm_pool_create_buffer(pool, 0, p->width, p->height, stride, WL_SHM_FORMAT_XRGB8888);
	wl_shm_pool_destroy(pool);
	wl_surface_attach(p->surface, buffer, 0, 0);
	wl_surface_damage(p->surface, 0, 0, p->width, p->height);
	wl_surface_commit(p->surface);
	munmap(pixels, size);
	return 1;
}

static neru_probe *neru_probe_start(void) {
	neru_probe *p = (neru_probe *)calloc(1, sizeof(neru_probe));
	if (!p) {
		return NULL;
	}
	p->pending = -1;
	p->width = 640;
	p->height = 480;
	p->shm_fd = -1;

	p->display = wl_display_connect(NULL);
	if (!p->display) {
		free(p);
		return NULL;
	}
	p->registry = wl_display_get_registry(p->display);
	wl_registry_add_listener(p->registry, &neru_probe_registry_listener, p);
	// Twice: the first drains the registry's globals, the second the events
	// the listeners those globals installed have already produced.
	for (int i = 0; i < 2; i++) {
		wl_display_roundtrip(p->display);
	}
	if (!p->compositor || !p->shm || !p->wm_base || !p->seat) {
		wl_display_disconnect(p->display);
		free(p);
		return NULL;
	}

	p->surface = wl_compositor_create_surface(p->compositor);
	p->xdg_surface = xdg_wm_base_get_xdg_surface(p->wm_base, p->surface);
	xdg_surface_add_listener(p->xdg_surface, &neru_probe_surface_listener, p);
	p->toplevel = xdg_surface_get_toplevel(p->xdg_surface);
	xdg_toplevel_add_listener(p->toplevel, &neru_probe_toplevel_listener, p);
	xdg_toplevel_set_title(p->toplevel, "neru-scroll-probe");
	xdg_toplevel_set_app_id(p->toplevel, "neru-scroll-probe");
	wl_surface_commit(p->surface);
	for (int i = 0; i < 200 && !p->configured; i++) {
		wl_display_roundtrip(p->display);
		usleep(10000);
	}
	if (!p->configured) {
		wl_display_disconnect(p->display);
		free(p);
		return NULL;
	}

	p->shm_fd = memfd_create("neru-scroll-probe", MFD_CLOEXEC);
	if (p->shm_fd < 0 || !neru_probe_attach(p)) {
		neru_probe_stop(p);
		return NULL;
	}

	int mapped_width = p->width;
	for (int i = 0; i < 100 && p->width == mapped_width; i++) {
		wl_display_roundtrip(p->display);
		usleep(10000);
	}
	if (p->width != mapped_width && !neru_probe_attach(p)) {
		neru_probe_stop(p);
		return NULL;
	}
	wl_display_roundtrip(p->display);

	return p;
}

static void neru_probe_stop(neru_probe *p) {
	if (!p) {
		return;
	}
	if (p->shm_fd >= 0) {
		close(p->shm_fd);
	}
	wl_display_disconnect(p->display);
	free(p);
}

static int neru_probe_dispatch(neru_probe *p, int timeout_ms) {
	wl_display_flush(p->display);
	if (wl_display_dispatch_pending(p->display) > 0) {
		return 1;
	}
	struct pollfd pfd = {.fd = wl_display_get_fd(p->display), .events = POLLIN};
	if (poll(&pfd, 1, timeout_ms) > 0) {
		return wl_display_dispatch(p->display) >= 0;
	}
	return 0;
}

static int neru_probe_entered(neru_probe *p) { return p->entered; }
static int neru_probe_event_count(neru_probe *p) { return p->event_count; }
static double neru_probe_event_value(neru_probe *p, int index) { return p->events[index].value; }
static int neru_probe_event_has_step(neru_probe *p, int index) { return p->events[index].has_step; }
static int neru_probe_event_source(neru_probe *p, int index) {
	return p->events[index].has_source ? p->events[index].source : -1;
}
*/
import "C"

import (
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux/wlr_protocol"
)

// probeScrollEvent is one scroll frame the compositor delivered: the continuous
// distance, and whether a wheel-step count came with it.
type probeScrollEvent struct {
	value   float64
	hasStep bool
	// source is the wl_pointer.axis_source the frame declared, or -1 when it
	// declared none.
	source int
}

// scrollProbe is a mapped application window under the compositor's pointer.
type scrollProbe struct {
	handle *C.neru_probe
}

// startScrollProbe maps the window. The bool is false when this session cannot
// carry one, which is a skip rather than a failure: it says nothing about the
// code under test.
func startScrollProbe() (*scrollProbe, bool) {
	handle := C.neru_probe_start()
	if handle == nil {
		return nil, false
	}

	return &scrollProbe{handle: handle}, true
}

func (p *scrollProbe) stop() {
	C.neru_probe_stop(p.handle)
}

// dispatch reads whatever the compositor has sent, waiting up to timeoutMs for
// something to arrive.
func (p *scrollProbe) dispatch(timeoutMs int) {
	C.neru_probe_dispatch(p.handle, C.int(timeoutMs))
}

// entered reports whether the pointer is over the window. A client with no
// pointer focus receives no scroll however it is injected.
func (p *scrollProbe) entered() bool {
	return C.neru_probe_entered(p.handle) != 0
}

func (p *scrollProbe) axisEvents() []probeScrollEvent {
	count := int(C.neru_probe_event_count(p.handle))
	events := make([]probeScrollEvent, 0, count)

	for i := range count {
		events = append(events, probeScrollEvent{
			value:   float64(C.neru_probe_event_value(p.handle, C.int(i))),
			hasStep: C.neru_probe_event_has_step(p.handle, C.int(i)) != 0,
			source:  int(C.neru_probe_event_source(p.handle, C.int(i))),
		})
	}

	return events
}

//go:build linux && cgo

package overlay

/*
#cgo linux LDFLAGS: -lwayland-client -lcairo -lxkbcommon
#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <unistd.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <cairo/cairo.h>
#include <poll.h>

#include "../../core/infra/platform/linux/wlr_protocol/xdg-shell.h"
#include "../../core/infra/platform/linux/wlr_protocol/xdg-shell.c"
#include "../../core/infra/platform/linux/wlr_protocol/layer-shell.h"
#include "../../core/infra/platform/linux/wlr_protocol/layer-shell.c"
#include "../../core/infra/platform/linux/wlr_protocol/xdg-output.h"

// Forward declarations
#define NERU_MAX_OUTPUTS 16

typedef struct {
    int x, y, width, height;
    struct wl_output *wl_output;
    struct zxdg_output_v1 *xdg_output;

    struct wl_surface *wl_surface;
    struct zwlr_layer_surface_v1 *layer_surface;
    struct wl_buffer *buffer;

    cairo_surface_t *cairo_surface;
    cairo_t *cr;
    void *shm_data;
    size_t shm_size;
} NeruWaylandOverlayScreen;

typedef struct {
    struct wl_display *display;
    struct wl_registry *registry;
    struct wl_compositor *compositor;
    struct wl_shm *shm;
    struct zxdg_output_manager_v1 *xdg_output_mgr;
    struct zwlr_layer_shell_v1 *layer_shell;
    struct wl_seat *wl_seat;
    struct wl_keyboard *wl_keyboard;

    struct xkb_context *xkb_ctx;
    struct xkb_state *xkb_state;

    NeruWaylandOverlayScreen screens[NERU_MAX_OUTPUTS];
    int nr_screens;

    int configured;
    int keyboard_interactivity_set;

    int event_fd;
    int running;
} NeruWaylandOverlay;

// Global keyboard channel for Go callback
static char key_buffer[256];
static volatile int key_available = 0;
static volatile int keyboard_enter_received = 0;

// Modifier tracking
static uint32_t current_mods = 0;

//export neruWaylandOverlayOnKey
static void neruWaylandOverlayOnKey(const char *key) {
    // Prepend modifier prefix if shift is held
    if (current_mods & (1 << 0)) { // shift
        snprintf(key_buffer, sizeof(key_buffer), "__modifier_shift %s", key);
    } else {
        strncpy(key_buffer, key, sizeof(key_buffer) - 1);
        key_buffer[sizeof(key_buffer) - 1] = 0;
    }
    key_available = 1;
}

//export neruWaylandOverlayOnEnter
static void neruWaylandOverlayOnEnter(void) {
    keyboard_enter_received = 1;
}

// Create anonymous shared memory
static int create_shm_file(off_t size) {
    int ret, fd;

    #ifdef __NR_memfd_create
    fd = syscall(__NR_memfd_create, "neru-overlay-shm", 0);
    if (fd >= 0) {
        do {
            ret = ftruncate(fd, size);
        } while (ret < 0 && errno == EINTR);
        if (ret >= 0) return fd;
        close(fd);
    }
    #endif

    // Fallback if no memfd
    char name[] = "/tmp/neru-shm-XXXXXX";
    fd = mkstemp(name);
    if (fd < 0) return -1;
    unlink(name);
    do {
        ret = ftruncate(fd, size);
    } while (ret < 0 && errno == EINTR);
    if (ret < 0) {
        close(fd);
        return -1;
    }
    return fd;
}

static void neru_layer_surface_configure(void *data,
    struct zwlr_layer_surface_v1 *layer_surface,
    uint32_t serial, uint32_t width, uint32_t height) {
    NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;
    zwlr_layer_surface_v1_ack_configure(layer_surface, serial);

    for (int i = 0; i < overlay->nr_screens; i++) {
        if (overlay->screens[i].layer_surface == layer_surface) {
            if (width > 0) overlay->screens[i].width = width;
            if (height > 0) overlay->screens[i].height = height;
            break;
        }
    }

    overlay->configured = 1;
}

static void neru_layer_surface_closed(void *data,
    struct zwlr_layer_surface_v1 *layer_surface) {
    // No-op
}

static const struct zwlr_layer_surface_v1_listener layer_surface_listener = {
    .configure = neru_layer_surface_configure,
    .closed = neru_layer_surface_closed,
};

static void neru_overlay_registry_global(void *data,
    struct wl_registry *registry,
    uint32_t name, const char *interface, uint32_t version) {
    NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;

    if (strcmp(interface, "wl_compositor") == 0) {
        overlay->compositor = wl_registry_bind(registry, name, &wl_compositor_interface, 4);
    } else if (strcmp(interface, "wl_shm") == 0) {
        overlay->shm = wl_registry_bind(registry, name, &wl_shm_interface, 1);
    } else if (strcmp(interface, "zwlr_layer_shell_v1") == 0) {
        overlay->layer_shell = wl_registry_bind(registry, name, &zwlr_layer_shell_v1_interface, 1);
    } else if (strcmp(interface, "wl_output") == 0) {
        if (overlay->nr_screens < NERU_MAX_OUTPUTS) {
            overlay->screens[overlay->nr_screens].wl_output =
                wl_registry_bind(registry, name, &wl_output_interface, 3 < version ? 3 : version);
            overlay->nr_screens++;
        }
    } else if (strcmp(interface, "zxdg_output_manager_v1") == 0) {
        overlay->xdg_output_mgr = wl_registry_bind(registry, name, &zxdg_output_manager_v1_interface, 3 < version ? 3 : version);
    } else if (strcmp(interface, "wl_seat") == 0) {
        overlay->wl_seat = wl_registry_bind(registry, name, &wl_seat_interface, 5);
    }
}

static void neru_overlay_registry_global_remove(void *data,
    struct wl_registry *registry, uint32_t name) {
    // No-op
}

static const struct wl_registry_listener overlay_registry_listener = {
    .global = neru_overlay_registry_global,
    .global_remove = neru_overlay_registry_global_remove,
};

static void neru_xdg_output_logical_position(void *data,
    struct zxdg_output_v1 *xdg_output, int32_t x, int32_t y) {
    NeruWaylandOverlayScreen *scr = (NeruWaylandOverlayScreen *)data;
    scr->x = x;
    scr->y = y;
}

static void neru_xdg_output_logical_size(void *data,
    struct zxdg_output_v1 *xdg_output, int32_t w, int32_t h) {
    NeruWaylandOverlayScreen *scr = (NeruWaylandOverlayScreen *)data;
    scr->width = w;
    scr->height = h;
}

static void neru_xdg_output_done(void *data, struct zxdg_output_v1 *xdg_output) {}
static void neru_xdg_output_name(void *data, struct zxdg_output_v1 *xdg_output, const char *name) {}
static void neru_xdg_output_description(void *data, struct zxdg_output_v1 *xdg_output, const char *description) {}

static const struct zxdg_output_v1_listener xdg_output_listener = {
    .logical_position = neru_xdg_output_logical_position,
    .logical_size = neru_xdg_output_logical_size,
    .done = neru_xdg_output_done,
    .name = neru_xdg_output_name,
    .description = neru_xdg_output_description,
};

// Wayland keyboard listener for key events
static void neru_keyboard_keymap(void *data, struct wl_keyboard *keyboard,
    uint32_t format, int32_t fd, uint32_t size) {
    NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;
    if (format == WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1) {
        char *map = mmap(NULL, size, PROT_READ, MAP_PRIVATE, fd, 0);
        if (map != MAP_FAILED) {
            if (overlay->xkb_ctx) xkb_context_unref(overlay->xkb_ctx);
            overlay->xkb_ctx = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
            if (overlay->xkb_ctx) {
                struct xkb_keymap *keymap = xkb_keymap_new_from_string(
                    overlay->xkb_ctx, map, XKB_KEYMAP_FORMAT_TEXT_V1, XKB_KEYMAP_COMPILE_NO_FLAGS);
                if (keymap) {
                    if (overlay->xkb_state) xkb_state_unref(overlay->xkb_state);
                    overlay->xkb_state = xkb_state_new(keymap);
                    xkb_keymap_unref(keymap);
                }
            }
            munmap(map, size);
        }
        close(fd);
    }
}

static void neru_keyboard_enter(void *data, struct wl_keyboard *keyboard,
    uint32_t serial, struct wl_surface *surface, struct wl_array *keys) {
    keyboard_enter_received = 1;
}

static void neru_keyboard_leave(void *data, struct wl_keyboard *keyboard,
    uint32_t serial, struct wl_surface *surface) {}

static void neru_keyboard_key(void *data, struct wl_keyboard *keyboard,
    uint32_t serial, uint32_t time, uint32_t key, uint32_t state) {
    NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;
    if (state == WL_KEYBOARD_KEY_STATE_PRESSED) {
        char buf[64] = {0};

        if (overlay->xkb_state) {
            xkb_keysym_t keysym = xkb_state_key_get_one_sym(overlay->xkb_state, key + 8);
            xkb_keysym_get_name(keysym, buf, sizeof(buf));

            // Check modifiers - build modifier prefix
            char mod_prefix[64] = "";
            if (current_mods & (1 << 0)) strcat(mod_prefix, "__modifier_shift ");
            if (current_mods & (1 << 1)) strcat(mod_prefix, "__modifier_ctrl ");
            if (current_mods & (1 << 2)) strcat(mod_prefix, "__modifier_alt ");
            if (current_mods & (1 << 3)) strcat(mod_prefix, "__modifier_cmd ");

            // Check if it's a letter (A-Z or a-z)
            if (strlen(buf) == 1 && ((buf[0] >= 'A' && buf[0] <= 'Z') || (buf[0] >= 'a' && buf[0] <= 'z'))) {
                // Convert to lowercase for the key
                char lower = (buf[0] >= 'a') ? buf[0] : buf[0] + 32;
                if (mod_prefix[0]) {
                    snprintf(key_buffer, sizeof(key_buffer), "%s%c", mod_prefix, lower);
                } else {
                    snprintf(key_buffer, sizeof(key_buffer), "%c", lower);
                }
                key_available = 1;
            } else if (buf[0]) {
                // Non-letter keys - just send as-is
                neruWaylandOverlayOnKey(buf);
            } else {
                xkb_state_key_get_utf8(overlay->xkb_state, key + 8, buf, sizeof(buf));
                if (buf[0]) {
                    neruWaylandOverlayOnKey(buf);
                }
            }
        }
    }
}

static void neru_keyboard_modifiers(void *data, struct wl_keyboard *keyboard,
    uint32_t serial, uint32_t mods_depressed, uint32_t mods_latched, uint32_t mods_locked, uint32_t group) {
    NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;
    if (overlay->xkb_state) {
        xkb_state_update_mask(overlay->xkb_state, mods_depressed, mods_latched, mods_locked, 0, 0, group);
    }
    // Track modifiers: bit 0 = shift, bit 1 = ctrl, bit 2 = alt, bit 3 = meta
    current_mods = mods_depressed;
}

static void neru_keyboard_repeat_info(void *data, struct wl_keyboard *keyboard,
    int32_t rate, int32_t delay) {}

static const struct wl_keyboard_listener keyboard_listener = {
    .keymap = neru_keyboard_keymap,
    .enter = neru_keyboard_enter,
    .leave = neru_keyboard_leave,
    .key = neru_keyboard_key,
    .modifiers = neru_keyboard_modifiers,
    .repeat_info = neru_keyboard_repeat_info,
};

// Seat listener to detect keyboard
static void neru_seat_capabilities(void *data, struct wl_seat *seat, uint32_t capabilities) {
    NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;
    if (capabilities & WL_SEAT_CAPABILITY_KEYBOARD) {
        if (!overlay->wl_keyboard) {
            overlay->wl_keyboard = wl_seat_get_keyboard(seat);
            if (overlay->wl_keyboard) {
                wl_keyboard_add_listener(overlay->wl_keyboard, &keyboard_listener, overlay);
            }
        }
    }
}

static void neru_seat_name(void *data, struct wl_seat *seat, const char *name) {}

static const struct wl_seat_listener seat_listener = {
    .capabilities = neru_seat_capabilities,
    .name = neru_seat_name,
};

static NeruWaylandOverlay* neru_wayland_overlay_new(void) {
    NeruWaylandOverlay *overlay = calloc(1, sizeof(NeruWaylandOverlay));
    if (!overlay) return NULL;

    overlay->display = wl_display_connect(NULL);
    if (!overlay->display) {
        free(overlay);
        return NULL;
    }

    overlay->registry = wl_display_get_registry(overlay->display);
    wl_registry_add_listener(overlay->registry, &overlay_registry_listener, overlay);
    wl_display_roundtrip(overlay->display); // get globals

    if (!overlay->compositor || !overlay->layer_shell || !overlay->shm || !overlay->xdg_output_mgr) {
        wl_display_disconnect(overlay->display);
        free(overlay);
        return NULL;
    }

    for (int i = 0; i < overlay->nr_screens; i++) {
        NeruWaylandOverlayScreen *scr = &overlay->screens[i];
        scr->xdg_output = zxdg_output_manager_v1_get_xdg_output(overlay->xdg_output_mgr, scr->wl_output);
        zxdg_output_v1_add_listener(scr->xdg_output, &xdg_output_listener, scr);
    }
    wl_display_roundtrip(overlay->display); // get screen sizes

    // Setup seat listener for keyboard
    if (overlay->wl_seat) {
        wl_seat_add_listener(overlay->wl_seat, &seat_listener, overlay);
        wl_display_roundtrip(overlay->display);
    }

    // Setup xkb context
    overlay->xkb_ctx = xkb_context_new(XKB_CONTEXT_NO_FLAGS);

    // Try to get keyboard immediately and set up listener
    if (overlay->wl_seat) {
        struct wl_keyboard *kb = wl_seat_get_keyboard(overlay->wl_seat);
        if (kb) {
            wl_keyboard_add_listener(kb, &keyboard_listener, overlay);
        }
    }

    return overlay;
}

static void neru_wayland_overlay_destroy(NeruWaylandOverlay *overlay) {
    if (!overlay) return;

    for (int i = 0; i < overlay->nr_screens; i++) {
        NeruWaylandOverlayScreen *scr = &overlay->screens[i];
        if (scr->cr) cairo_destroy(scr->cr);
        if (scr->cairo_surface) cairo_surface_destroy(scr->cairo_surface);
        if (scr->buffer) wl_buffer_destroy(scr->buffer);
        if (scr->shm_data) munmap(scr->shm_data, scr->shm_size);
        if (scr->layer_surface) zwlr_layer_surface_v1_destroy(scr->layer_surface);
        if (scr->wl_surface) wl_surface_destroy(scr->wl_surface);
        if (scr->xdg_output) zxdg_output_v1_destroy(scr->xdg_output);
    }

    if (overlay->xdg_output_mgr) zxdg_output_manager_v1_destroy(overlay->xdg_output_mgr);
    if (overlay->layer_shell) zwlr_layer_shell_v1_destroy(overlay->layer_shell);
    if (overlay->registry) wl_registry_destroy(overlay->registry);
    if (overlay->display) wl_display_disconnect(overlay->display);
    free(overlay);
}

static void neru_wayland_overlay_setup_buffers(NeruWaylandOverlay *overlay) {
    for (int i = 0; i < overlay->nr_screens; i++) {
        NeruWaylandOverlayScreen *scr = &overlay->screens[i];

        if (scr->layer_surface) continue; // Already configured

        // Skip if dimensions aren't set yet
        if (scr->width <= 0 || scr->height <= 0) continue;

        scr->wl_surface = wl_compositor_create_surface(overlay->compositor);

        // NOTE: Do NOT set input region to null - that would pass input through!
        // Let the compositor handle input based on keyboard_interactivity flag.

        scr->layer_surface = zwlr_layer_shell_v1_get_layer_surface(
            overlay->layer_shell, scr->wl_surface, scr->wl_output,
            ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY, "neru"
        );

        zwlr_layer_surface_v1_set_size(scr->layer_surface, scr->width, scr->height);
        zwlr_layer_surface_v1_set_anchor(scr->layer_surface,
            ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP | ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT |
            ZWLR_LAYER_SURFACE_V1_ANCHOR_RIGHT | ZWLR_LAYER_SURFACE_V1_ANCHOR_BOTTOM);
        zwlr_layer_surface_v1_set_exclusive_zone(scr->layer_surface, -1);

        // Request exclusive keyboard interactivity when overlay is shown
        // This tells the compositor to send keyboard events to this surface
        zwlr_layer_surface_v1_set_keyboard_interactivity(scr->layer_surface,
            ZWLR_LAYER_SURFACE_V1_KEYBOARD_INTERACTIVITY_EXCLUSIVE);

        zwlr_layer_surface_v1_add_listener(scr->layer_surface, &layer_surface_listener, overlay);
        wl_surface_commit(scr->wl_surface);
    }

    // Wait for configure events
    wl_display_roundtrip(overlay->display);

    for (int i = 0; i < overlay->nr_screens; i++) {
        NeruWaylandOverlayScreen *scr = &overlay->screens[i];
        if (scr->buffer) continue;

        int stride = scr->width * 4;
        scr->shm_size = stride * scr->height;
        int fd = create_shm_file(scr->shm_size);
        if (fd < 0) continue;

        scr->shm_data = mmap(NULL, scr->shm_size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
        struct wl_shm_pool *pool = wl_shm_create_pool(overlay->shm, fd, scr->shm_size);
        scr->buffer = wl_shm_pool_create_buffer(pool, 0, scr->width, scr->height, stride, WL_SHM_FORMAT_ARGB8888);
        wl_shm_pool_destroy(pool);
        close(fd);

        scr->cairo_surface = cairo_image_surface_create_for_data(scr->shm_data, CAIRO_FORMAT_ARGB32, scr->width, scr->height, stride);
        scr->cr = cairo_create(scr->cairo_surface);
    }
}

static void neru_wayland_overlay_show(NeruWaylandOverlay *overlay) {
    if (!overlay) return;
    for (int i = 0; i < overlay->nr_screens; i++) {
        NeruWaylandOverlayScreen *scr = &overlay->screens[i];
        if (scr->wl_surface && scr->buffer) {
            wl_surface_attach(scr->wl_surface, scr->buffer, 0, 0);
            wl_surface_damage_buffer(scr->wl_surface, 0, 0, INT32_MAX, INT32_MAX);
            wl_surface_commit(scr->wl_surface);
        }
    }
    wl_display_flush(overlay->display);
}

static void neru_wayland_overlay_hide(NeruWaylandOverlay *overlay) {
    if (!overlay) return;
    for (int i = 0; i < overlay->nr_screens; i++) {
        NeruWaylandOverlayScreen *scr = &overlay->screens[i];
        if (scr->wl_surface) {
            wl_surface_attach(scr->wl_surface, NULL, 0, 0);
            wl_surface_commit(scr->wl_surface);
        }
        // Destroy layer surface to allow proper recreation on next show
        if (scr->layer_surface) {
            zwlr_layer_surface_v1_destroy(scr->layer_surface);
            scr->layer_surface = NULL;
        }
        // Also destroy the surface
        if (scr->wl_surface) {
            wl_surface_destroy(scr->wl_surface);
            scr->wl_surface = NULL;
        }
        // Destroy buffer and cairo
        if (scr->buffer) {
            wl_buffer_destroy(scr->buffer);
            scr->buffer = NULL;
        }
        if (scr->cairo_surface) {
            cairo_surface_destroy(scr->cairo_surface);
            scr->cairo_surface = NULL;
        }
        if (scr->cr) {
            cairo_destroy(scr->cr);
            scr->cr = NULL;
        }
        if (scr->shm_data) {
            munmap(scr->shm_data, scr->shm_size);
            scr->shm_data = NULL;
        }
    }
    wl_display_flush(overlay->display);
}

static void neru_wayland_overlay_clear(NeruWaylandOverlay *overlay) {
    if (!overlay) return;
    for (int i = 0; i < overlay->nr_screens; i++) {
        NeruWaylandOverlayScreen *scr = &overlay->screens[i];
        if (scr->cr) {
            cairo_save(scr->cr);
            cairo_set_operator(scr->cr, CAIRO_OPERATOR_CLEAR);
            cairo_paint(scr->cr);
            cairo_restore(scr->cr);
        }
    }
}

static void neru_wayland_overlay_flush(NeruWaylandOverlay *overlay) {
    neru_wayland_overlay_show(overlay);
}

static void neru_wayland_overlay_color(cairo_t *cr, unsigned int color) {
    double a = ((color >> 24) & 0xFF) / 255.0;
    double r = ((color >> 16) & 0xFF) / 255.0;
    double g = ((color >> 8) & 0xFF) / 255.0;
    double b = (color & 0xFF) / 255.0;
    cairo_set_source_rgba(cr, r, g, b, a);
}

static void neru_wayland_overlay_rect(
    NeruWaylandOverlay *overlay,
    double x, double y, double width, double height,
    unsigned int fill, unsigned int stroke, double stroke_width
) {
    for (int i = 0; i < overlay->nr_screens; i++) {
        NeruWaylandOverlayScreen *scr = &overlay->screens[i];
        if (!scr->cr) continue;

        // Convert global coordinates to screen-local
        double scr_x = x - scr->x;
        double scr_y = y - scr->y;

        cairo_t *cr = scr->cr;
        cairo_save(cr);
        cairo_rectangle(cr, scr_x, scr_y, width, height);
        neru_wayland_overlay_color(cr, fill);
        cairo_fill_preserve(cr);
        neru_wayland_overlay_color(cr, stroke);
        cairo_set_line_width(cr, stroke_width);
        cairo_stroke(cr);
        cairo_restore(cr);
    }
}

static void neru_wayland_overlay_text(
    NeruWaylandOverlay *overlay,
    const char *text,
    double x, double y,
    double font_size,
    unsigned int color
) {
    for (int i = 0; i < overlay->nr_screens; i++) {
        NeruWaylandOverlayScreen *scr = &overlay->screens[i];
        if (!scr->cr) continue;

        // Convert global coordinates to screen-local
        double scr_x = x - scr->x;
        double scr_y = y - scr->y;

        cairo_t *cr = scr->cr;
        cairo_text_extents_t extents;
        cairo_save(cr);
        cairo_select_font_face(cr, "Sans", CAIRO_FONT_SLANT_NORMAL, CAIRO_FONT_WEIGHT_BOLD);
        cairo_set_font_size(cr, font_size);
        cairo_text_extents(cr, text, &extents);
        neru_wayland_overlay_color(cr, color);
        cairo_move_to(cr, scr_x - (extents.width / 2.0) - extents.x_bearing, scr_y - (extents.height / 2.0) - extents.y_bearing);
        cairo_show_text(cr, text);
        cairo_restore(cr);
    }
}

// Poll for Wayland events without blocking
static int neru_wayland_overlay_poll(NeruWaylandOverlay *overlay) {
    if (!overlay || !overlay->display) return -1;

    struct pollfd pfd = {
        .fd = wl_display_get_fd(overlay->display),
        .events = POLLIN,
        .revents = 0
    };

    int ret = poll(&pfd, 1, 0);
    if (ret > 0 && (pfd.revents & POLLIN)) {
        wl_display_dispatch(overlay->display);
    } else {
        wl_display_dispatch_pending(overlay->display);
    }
    return ret;
}

// Get pending key from buffer (non-blocking)
static const char* neru_wayland_overlay_get_key(NeruWaylandOverlay *overlay) {
    if (!key_available) return NULL;
    key_available = 0;
    return key_buffer;
}

//export neruWaylandOverlayPoll
static void neruWaylandOverlayPoll(NeruWaylandOverlay *overlay) {
    neru_wayland_overlay_poll(overlay);
}

//export neruWaylandOverlayGetKey
static const char* neruWaylandOverlayGetKey(NeruWaylandOverlay *overlay) {
    return neru_wayland_overlay_get_key(overlay);
}

//export neruWaylandOverlayCheckEnter
static int neruWaylandOverlayCheckEnter(NeruWaylandOverlay *overlay) {
    (void)overlay;
    if (keyboard_enter_received) {
        keyboard_enter_received = 0;
        return 1;
    }
    return 0;
}
*/
import "C"

import (
	"image"
	"strings"
	"unsafe"

	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"go.uber.org/zap"
)

type wlrootsOverlay struct {
	raw            *C.NeruWaylandOverlay
	logger         *zap.Logger
	currentPrefix  string
	hideUnmatched  bool
	currentSubgrid *domainGrid.Cell
}

func init() {
	wlrootsKeyboardCh = make(chan string, 64)
}

func newWlrootsOverlay(logger *zap.Logger) *wlrootsOverlay {
	raw := C.neru_wayland_overlay_new()
	if raw == nil {
		return nil
	}

	C.neru_wayland_overlay_setup_buffers(raw)

	o := &wlrootsOverlay{raw: raw, logger: logger}

	go o.keyboardPoller()

	return o
}

func (o *wlrootsOverlay) keyboardPoller() {
	for o.raw != nil {
		C.neruWaylandOverlayPoll(o.raw)
		key := C.neruWaylandOverlayGetKey(o.raw)
		if key != nil {
			select {
			case wlrootsKeyboardCh <- C.GoString(key):
			default:
			}
		}
	}
}

func (o *wlrootsOverlay) Healthy() bool {
	return o != nil && o.raw != nil
}

func (o *wlrootsOverlay) WindowPtr() unsafe.Pointer {
	if o == nil {
		return nil
	}
	return unsafe.Pointer(o.raw)
}

func (o *wlrootsOverlay) Show() {
	if o != nil && o.raw != nil {
		// Re-setup buffers in case dimensions changed
		C.neru_wayland_overlay_setup_buffers(o.raw)
		C.neru_wayland_overlay_show(o.raw)
	}
}

func (o *wlrootsOverlay) Hide() {
	if o != nil && o.raw != nil {
		C.neru_wayland_overlay_hide(o.raw)
	}
}

func (o *wlrootsOverlay) Clear() {
	if o != nil && o.raw != nil {
		C.neru_wayland_overlay_clear(o.raw)
	}
}

func (o *wlrootsOverlay) Resize() {
	// Wayland layer shells auto-resize
}

func (o *wlrootsOverlay) Destroy() {
	if o != nil && o.raw != nil {
		C.neru_wayland_overlay_destroy(o.raw)
		o.raw = nil
	}
}

func (o *wlrootsOverlay) UpdateGridMatches(prefix string) {
	o.currentPrefix = strings.ToUpper(prefix)
}

func (o *wlrootsOverlay) ShowSubgrid(cell *domainGrid.Cell, _ gridcomponent.Style) {
	o.currentSubgrid = cell
}

func (o *wlrootsOverlay) SetHideUnmatched(hide bool) {
	o.hideUnmatched = hide
}

func (o *wlrootsOverlay) DrawGrid(g *domainGrid.Grid, input string, style gridcomponent.Style) {
	if o == nil || o.raw == nil || g == nil {
		return
	}
	o.Clear()

	prefix := strings.ToUpper(input)
	for _, cell := range g.AllCells() {
		label := strings.ToUpper(cell.Coordinate())
		matched := strings.HasPrefix(label, prefix)
		if o.hideUnmatched && prefix != "" && !matched {
			continue
		}

		fill := uint32(0x18000000)
		text := style.LabelFontColor
		border := style.LineColor
		if matched && prefix != "" {
			fill = 0x66465FBC
			text = 0xFFF8FAFF
		}
		o.drawRect(cell.Bounds(), fill, border, style.LineWidth)
		o.drawTextCentered(label, cell.Bounds(), style.LabelFontSize, text)
	}

	if o.currentSubgrid != nil {
		o.drawSubgrid(o.currentSubgrid.Bounds(), style)
	}
	C.neru_wayland_overlay_flush(o.raw)
}

func (o *wlrootsOverlay) DrawRecursiveGrid(
	bounds image.Rectangle,
	_ int,
	keys string,
	gridCols int,
	gridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	if o == nil || o.raw == nil || bounds.Empty() || gridCols <= 0 || gridRows <= 0 {
		return
	}
	o.Clear()

	keyRunes := []rune(strings.ToUpper(keys))
	cellWidth := bounds.Dx() / gridCols
	cellHeight := bounds.Dy() / gridRows
	index := 0
	for row := range gridRows {
		for col := range gridCols {
			cell := image.Rect(
				bounds.Min.X+col*cellWidth,
				bounds.Min.Y+row*cellHeight,
				bounds.Min.X+(col+1)*cellWidth,
				bounds.Min.Y+(row+1)*cellHeight,
			)
			if col == gridCols-1 {
				cell.Max.X = bounds.Max.X
			}
			if row == gridRows-1 {
				cell.Max.Y = bounds.Max.Y
			}

			o.drawRect(cell, 0x10000000, style.LineColor, style.LineWidth)
			if index < len(keyRunes) {
				o.drawTextCentered(string(keyRunes[index]), cell, style.LabelFontSize, style.LabelFontColor)
			}
			index++
		}
	}

	if virtualPointer.Visible {
		vpBounds := image.Rect(
			virtualPointer.Position.X-virtualPointer.Size/2,
			virtualPointer.Position.Y-virtualPointer.Size/2,
			virtualPointer.Position.X+virtualPointer.Size/2,
			virtualPointer.Position.Y+virtualPointer.Size/2,
		)
		o.drawRect(vpBounds, parseHexColor(virtualPointer.FillColor), style.LineColor, 1)
	}

	C.neru_wayland_overlay_flush(o.raw)
}

func (o *wlrootsOverlay) DrawBadge(x, y int, text string, colors overlayColors) {
	if o == nil || o.raw == nil || text == "" {
		return
	}

	paddingX := 10
	fontSize := 14.0
	width := len(text)*9 + paddingX*2
	height := 24
	rect := image.Rect(x, y, x+width, y+height)

	o.drawRect(rect, colors.background, colors.border, 1)
	o.drawTextCentered(text, rect, fontSize, colors.text)
	C.neru_wayland_overlay_flush(o.raw)
}

func (o *wlrootsOverlay) drawSubgrid(bounds image.Rectangle, style gridcomponent.Style) {
	cols, rows := 3, 3
	cellWidth := bounds.Dx() / cols
	cellHeight := bounds.Dy() / rows
	keys := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}
	index := 0
	for row := range rows {
		for col := range cols {
			cell := image.Rect(
				bounds.Min.X+col*cellWidth,
				bounds.Min.Y+row*cellHeight,
				bounds.Min.X+(col+1)*cellWidth,
				bounds.Min.Y+(row+1)*cellHeight,
			)
			o.drawRect(cell, 0x14000000, style.LineColor, 1)
			o.drawTextCentered(keys[index], cell, style.LabelFontSize*0.7, style.LabelFontColor)
			index++
		}
	}
}

func (o *wlrootsOverlay) drawRect(bounds image.Rectangle, fill uint32, border uint32, lineWidth float64) {
	C.neru_wayland_overlay_rect(
		o.raw,
		C.double(bounds.Min.X),
		C.double(bounds.Min.Y),
		C.double(bounds.Dx()),
		C.double(bounds.Dy()),
		C.uint(fill),
		C.uint(border),
		C.double(lineWidth),
	)
}

func (o *wlrootsOverlay) drawTextCentered(text string, bounds image.Rectangle, fontSize float64, color uint32) {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText)) //nolint:nlreturn

	C.neru_wayland_overlay_text(
		o.raw,
		cText,
		C.double(bounds.Min.X+bounds.Dx()/2),
		C.double(bounds.Min.Y+bounds.Dy()/2),
		C.double(fontSize),
		C.uint(color),
	)
}

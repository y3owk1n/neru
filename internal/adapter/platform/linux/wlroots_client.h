#ifndef WLROOTS_CLIENT_H
#define WLROOTS_CLIENT_H

#include "common_defs.h"

#include <pthread.h>
#include <stdatomic.h>
#include <stdint.h>
#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>

typedef struct {
	int x;
	int y;
	int w;
	int h;
	int state;
	char name[128];
	char name_valid;
	uint32_t registry_name;  // wl_registry global id, so global_remove can match on hotplug
	struct wl_output *wl_output;
	struct zxdg_output_v1 *xdg_output;
	struct wl_surface *discovery_surface;
} NeruWaylandScreen;

// app_id / title buffer length. Reverse-DNS desktop IDs (e.g.
// "org.kde.konsole") and freedesktop app-ids fit comfortably in 256 bytes.
#define NERU_APP_ID_LEN 256

// Window-title buffer length. Titles can be longer than app-ids; 512 bytes
// avoids truncating most window titles so they match the AT-SPI frame name.
#define NERU_TITLE_LEN 512

// NeruToplevel mirrors one zwlr_foreign_toplevel_handle_v1. Nodes are heap
// allocated and threaded onto NeruWlrootsClient.toplevels via `link`, so there
// is no fixed cap on the number of windows tracked. app_id and the activated
// flag are committed atomically on the handle's `done` event; the `pending_*`
// fields buffer values arriving between `done` events so a half-applied state
// is never observed.
typedef struct {
	struct wl_list link;  // links into NeruWlrootsClient.toplevels
	struct zwlr_foreign_toplevel_handle_v1 *handle;
	char app_id[NERU_APP_ID_LEN];
	char pending_app_id[NERU_APP_ID_LEN];
	int has_pending_app_id;
	char title[NERU_TITLE_LEN];
	char pending_title[NERU_TITLE_LEN];
	int has_pending_title;
	int activated;
	int pending_activated;
} NeruToplevel;

typedef struct NeruWlrootsClient {
	struct wl_display *display;
	struct wl_registry *registry;
	struct wl_compositor *compositor;
	struct wl_shm *shm;
	struct zwlr_layer_shell_v1 *layer_shell;
	struct wl_seat *seat;
	// Atomic because the seat's capabilities event is what sets it, and that
	// event can arrive on the dispatch thread long after connect — a mouse
	// plugged into a session that started without one. Everything else here
	// reads it.
	_Atomic(struct wl_pointer *) pointer;

	struct zwp_relative_pointer_manager_v1 *rel_ptr_mgr;
	struct zwp_relative_pointer_v1 *rel_ptr;

	struct zwlr_virtual_pointer_manager_v1 *vptr_mgr;
	struct zwlr_virtual_pointer_v1 *vptr;
	struct zwp_virtual_keyboard_manager_v1 *vkeyboard_mgr;
	struct zwp_virtual_keyboard_v1 *vkeyboard;
	int vkeyboard_ready;
	struct zxdg_output_manager_v1 *xdg_output_mgr;

	// Foreign-toplevel management: tracks every toplevel and which one holds
	// the "activated" (focused) state so Neru can resolve the focused app_id
	// on wlroots/KWin Wayland sessions (GNOME/Mutter does not implement it).
	struct zwlr_foreign_toplevel_manager_v1 *toplevel_mgr;
	struct wl_list toplevels;              // list of NeruToplevel, guarded by toplevel_mutex
	char focused_app_id[NERU_APP_ID_LEN];  // guarded by toplevel_mutex
	char focused_title[NERU_TITLE_LEN];    // guarded by toplevel_mutex
	pthread_mutex_t toplevel_mutex;
	int toplevel_mutex_ready;

	// focus_pipe is a self-pipe that pushes focus-change notifications to Go
	// without a cross-thread callback: neru_wlr_recompute_focused writes one
	// byte to focus_pipe[1] (the write end) whenever focused_app_id changes, and
	// the Go app watcher blocks reading focus_pipe[0] (the read end, exposed via
	// neru_wlr_focus_event_fd). Both ends are non-blocking so the dispatch
	// thread never stalls under toplevel_mutex. Both are -1 until initialized.
	int focus_pipe[2];
	int focus_pipe_ready;

	// screen_pipe is a self-pipe that pushes display-configuration changes
	// (outputs added/removed via wl_registry global/global_remove) to Go, the
	// same way focus_pipe pushes focus changes. Both ends are non-blocking so the
	// dispatch thread never stalls; both are -1 until initialized.
	int screen_pipe[2];
	int screen_pipe_ready;

	struct xkb_context *xkb_ctx;
	struct xkb_keymap *xkb_keymap;
	uint32_t mod_shift;
	uint32_t mod_ctrl;
	uint32_t mod_alt;
	uint32_t mod_logo;
	uint32_t depressed_mods;

	NeruWaylandScreen screens[NERU_MAX_OUTPUTS];
	int nr_screens;

	atomic_int cursor_x;
	atomic_int cursor_y;
	_Atomic int cursor_initialized;
	wl_fixed_t cursor_x_frac;  // guarded by display_mutex — fractional
	wl_fixed_t cursor_y_frac;  // remainder for sub-pixel accumulation

	struct wl_surface *entered_discovery_surface;
	int forwarding;

	pthread_t dispatch_thread;
	pthread_mutex_t display_mutex;
	_Atomic int dispatch_running;

	// Barrier bookkeeping for neru_wlr_sync. sync_issued is the id of the last
	// wl_display.sync request sent, stamped under display_mutex; sync_completed
	// is the id of the last one the compositor answered, stored by the callback
	// on the dispatch thread. Callbacks complete in the order they were issued,
	// so a waiter watching for its own id cannot be satisfied by an earlier one.
	unsigned int sync_issued;  // guarded by display_mutex
	_Atomic unsigned int sync_completed;

	int connected;
} NeruWlrootsClient;

NeruWlrootsClient *neru_wlr_connect(void);
void neru_wlr_disconnect(NeruWlrootsClient *c);
int neru_wlr_start_dispatch(NeruWlrootsClient *c);
void neru_wlr_init_cursor(NeruWlrootsClient *c);
int neru_wlr_refresh_cursor(NeruWlrootsClient *c);
int neru_wlr_move_absolute(NeruWlrootsClient *c, int x, int y);
int neru_wlr_move_relative(NeruWlrootsClient *c, int dx, int dy);
int neru_wlr_button(NeruWlrootsClient *c, int button, int pressed);
int neru_wlr_click(NeruWlrootsClient *c, int button);
int neru_wlr_scroll(NeruWlrootsClient *c, int axis, int delta, int discrete);
int neru_wlr_scroll_batch(NeruWlrootsClient *c, int axis, int *deltas, int *discretes, int count);
// Emit one continuous (sub-notch capable) scroll step: an axis event carrying a
// fractional wl_fixed value and no discrete step count.  With no discrete step
// the compositor forwards the fraction to the focused client as a plain
// wl_pointer.axis, which is what makes an animated scroll finer than a wheel
// notch possible.  value is in the same units neru_wlr_scroll's delta uses.
int neru_wlr_scroll_continuous(NeruWlrootsClient *c, int axis, double value);
int neru_wlr_modifier_event(NeruWlrootsClient *c, const char *modifier, int is_down);

// neru_wlr_sync waits until the compositor has processed every request issued
// on this connection so far, or until timeout_ms elapses. Returns 1 when the
// compositor answered, 0 on timeout or with no connection to wait on.
//
// It is how a caller that has just injected a virtual-keyboard modifier learns
// the modifier is applied, rather than guessing with a sleep.
int neru_wlr_sync(NeruWlrootsClient *c, int timeout_ms);
int neru_wlr_get_cursor(NeruWlrootsClient *c, int *x, int *y);
void neru_wlr_set_cursor(NeruWlrootsClient *c, int x, int y);
int neru_wlr_screen_count(NeruWlrootsClient *c);
int neru_wlr_screen_info(NeruWlrootsClient *c, int idx, int *x, int *y, int *w, int *h, char *name_out, int name_len);
int neru_wlr_has_virtual_pointer(NeruWlrootsClient *c);
int neru_wlr_has_virtual_keyboard(NeruWlrootsClient *c);
int neru_wlr_key(NeruWlrootsClient *c, uint32_t keycode, int pressed);

// neru_wlr_has_toplevel_manager reports whether the compositor advertised the
// zwlr_foreign_toplevel_manager_v1 global (true on wlroots and KWin/KDE).
int neru_wlr_has_toplevel_manager(NeruWlrootsClient *c);

// neru_wlr_focused_app_id copies the app_id of the currently-activated
// toplevel into out (NUL-terminated, capped at out_len). Returns 1 when a
// non-empty focused app_id is available, 0 otherwise (no manager, nothing
// focused, or the focused toplevel has no app_id yet).
int neru_wlr_focused_app_id(NeruWlrootsClient *c, char *out, int out_len);

// neru_wlr_focused_app_identity copies the app_id and title of the currently-
// activated toplevel under a single lock, so the two always describe the same
// window (no focus commit can interleave). Returns 1 when a focused app_id is
// available, 0 otherwise. The title disambiguates multiple windows of the
// focused application, which share an app_id.
int neru_wlr_focused_app_identity(NeruWlrootsClient *c, char *app_out, int app_len, char *title_out, int title_len);

// neru_wlr_focus_event_fd returns a readable file descriptor that becomes
// readable whenever the focused app_id changes. Callers poll it, drain the
// pending byte(s), then re-query neru_wlr_focused_app_id. Returns -1 when no
// pipe is available. The fd is owned by the client and closed by
// neru_wlr_disconnect; callers must not close it.
int neru_wlr_focus_event_fd(NeruWlrootsClient *c);

// neru_wlr_screen_event_fd returns a readable file descriptor that becomes
// readable whenever the display configuration changes (an output is added or
// removed). Callers poll it, drain the pending byte(s), then re-read the screen
// list via neru_wlr_screen_count / neru_wlr_screen_info. Returns -1 when no pipe
// is available. The fd is owned by the client and closed by neru_wlr_disconnect;
// callers must not close it.
int neru_wlr_screen_event_fd(NeruWlrootsClient *c);

#endif /* WLROOTS_CLIENT_H */

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

// app_id buffer length. Reverse-DNS desktop IDs fit comfortably.
#define NERU_APP_ID_LEN 256

// Window-title buffer length, long enough that most titles are not truncated
// and still match the AT-SPI frame name.
#define NERU_TITLE_LEN 512

// NeruToplevel mirrors one window. Nodes are heap allocated on
// NeruWlrootsClient.toplevels, so there is no cap. State commits atomically on
// `done`, and the `pending_*` fields buffer values in between so a half-applied
// state is never observed.
// On wlroots and KWin a node holds a zwlr_foreign_toplevel_handle_v1. On
// cosmic-comp it holds an ext_foreign_toplevel_handle_v1 (app_id, title) paired
// with the zcosmic_toplevel_handle_v1 that carries the activated state and the
// global geometry. Geometry exists only on the cosmic pair.
typedef struct {
	struct wl_list link;  // links into NeruWlrootsClient.toplevels
	struct zwlr_foreign_toplevel_handle_v1 *handle;
	struct ext_foreign_toplevel_handle_v1 *ext_handle;
	struct zcosmic_toplevel_handle_v1 *cosmic_handle;
	int has_geometry;
	int pending_has_geometry;
	int32_t geometry[4];          // x, y, w, h in global logical pixels
	int32_t pending_geometry[4];  // buffered until the next commit
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
	// Atomic because the capabilities event sets it on the dispatch thread,
	// possibly long after connect, as when a mouse is plugged in later.
	_Atomic(struct wl_pointer *) pointer;

	struct zwp_relative_pointer_manager_v1 *rel_ptr_mgr;
	struct zwp_relative_pointer_v1 *rel_ptr;

	struct zwlr_virtual_pointer_manager_v1 *vptr_mgr;
	struct zwlr_virtual_pointer_v1 *vptr;
	struct zwp_virtual_keyboard_manager_v1 *vkeyboard_mgr;
	struct zwp_virtual_keyboard_v1 *vkeyboard;
	int vkeyboard_ready;
	struct zxdg_output_manager_v1 *xdg_output_mgr;

	// Tracks every toplevel and the activated one, to resolve the focused app_id
	// on wlroots and KWin (GNOME/Mutter does not implement it).
	struct zwlr_foreign_toplevel_manager_v1 *toplevel_mgr;
	// cosmic-comp's replacement for the wlr manager. The ext list names every
	// toplevel, the cosmic info global says which is activated and where it is.
	// Both are bound only when the wlr manager is absent. Neither alone is
	// enough to resolve focus, so the pair is used together or not at all.
	struct ext_foreign_toplevel_list_v1 *ext_toplevel_list;
	struct zcosmic_toplevel_info_v1 *cosmic_toplevel_info;
	// The registry only records these two globals. They are bound later, with
	// their listeners attached in the same breath, so the replay of existing
	// windows a bind triggers cannot land on a listenerless proxy and leak.
	uint32_t ext_toplevel_list_name;
	uint32_t cosmic_toplevel_info_name;
	uint32_t cosmic_toplevel_info_version;
	struct wl_list toplevels;              // list of NeruToplevel, guarded by toplevel_mutex
	char focused_app_id[NERU_APP_ID_LEN];  // guarded by toplevel_mutex
	char focused_title[NERU_TITLE_LEN];    // guarded by toplevel_mutex
	int focused_has_geometry;              // guarded by toplevel_mutex
	int32_t focused_geometry[4];           // guarded by toplevel_mutex
	pthread_mutex_t toplevel_mutex;
	int toplevel_mutex_ready;

	// focus_pipe pushes focus changes to Go without a cross-thread callback.
	// neru_wlr_recompute_focused writes a byte to [1] when focused_app_id
	// changes, and Go reads [0] via neru_wlr_focus_event_fd. Both ends are
	// non-blocking so the dispatch thread never stalls under toplevel_mutex.
	// Both are -1 until initialized.
	int focus_pipe[2];
	int focus_pipe_ready;

	// screen_pipe does the same for outputs added and removed.
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
	wl_fixed_t cursor_x_frac;  // guarded by display_mutex, fractional
	wl_fixed_t cursor_y_frac;  // remainder for sub-pixel accumulation

	struct wl_surface *entered_discovery_surface;
	int forwarding;

	pthread_t dispatch_thread;
	pthread_mutex_t display_mutex;
	_Atomic int dispatch_running;

	// Barrier bookkeeping for neru_wlr_sync. sync_issued is the id of the last
	// wl_display.sync request sent, stamped under display_mutex, and sync_completed
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
// Emit one continuous scroll step, a fractional wl_fixed axis value with no
// discrete count. The compositor forwards it to the focused client as a plain
// wl_pointer.axis, which lets an animated scroll go finer than a notch. value
// uses neru_wlr_scroll's delta units.
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

// neru_wlr_has_toplevel_manager reports whether the compositor advertised a
// toplevel source Neru resolves focus from: zwlr_foreign_toplevel_manager_v1
// (wlroots, KWin/KDE) or the ext-foreign-toplevel-list plus
// zcosmic_toplevel_info_v1 pair (cosmic-comp).
int neru_wlr_has_toplevel_manager(NeruWlrootsClient *c);

// neru_wlr_has_toplevel_geometry reports whether the toplevel source also
// carries window geometry, which only the cosmic pair does.
int neru_wlr_has_toplevel_geometry(NeruWlrootsClient *c);

// neru_wlr_focused_toplevel_geometry copies the activated toplevel's global
// geometry (top-left origin, logical pixels) into x, y, w, h. Returns 1 when a
// toplevel is activated and the compositor has reported where it is, 0
// otherwise (no geometry source, nothing focused, or no geometry event yet).
int neru_wlr_focused_toplevel_geometry(NeruWlrootsClient *c, int32_t *x, int32_t *y, int32_t *w, int32_t *h);

// neru_wlr_focused_app_id copies the app_id of the currently-activated
// toplevel into out (NUL-terminated, capped at out_len). Returns 1 when a
// non-empty focused app_id is available, 0 otherwise (no manager, nothing
// focused, or the focused toplevel has no app_id yet).
int neru_wlr_focused_app_id(NeruWlrootsClient *c, char *out, int out_len);

// neru_wlr_focused_app_identity copies the focused toplevel's app_id and title
// under one lock, so both describe the same window. Returns 1 when a focused
// app_id is available, 0 otherwise. The title tells one app's windows apart.
int neru_wlr_focused_app_identity(NeruWlrootsClient *c, char *app_out, int app_len, char *title_out, int title_len);

// neru_wlr_focus_event_fd returns a readable file descriptor that becomes
// readable whenever the focused app_id changes. Callers poll it, drain the
// pending byte(s), then re-query neru_wlr_focused_app_id. Returns -1 when no
// pipe is available. The fd is owned by the client and closed by
// neru_wlr_disconnect, and callers must not close it.
int neru_wlr_focus_event_fd(NeruWlrootsClient *c);

// neru_wlr_screen_event_fd is the same for display-configuration changes,
// after which callers re-read neru_wlr_screen_count / neru_wlr_screen_info.
// Same ownership rule as the focus fd.
int neru_wlr_screen_event_fd(NeruWlrootsClient *c);

#endif /* WLROOTS_CLIENT_H */

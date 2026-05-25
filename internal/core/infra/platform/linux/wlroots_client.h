#ifndef WLROOTS_CLIENT_H
#define WLROOTS_CLIENT_H

#include <stdint.h>
#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>

#define NERU_MAX_OUTPUTS 16

typedef struct {
	int x;
	int y;
	int w;
	int h;
	int state;
	char name[128];
	char name_valid;
	struct wl_output *wl_output;
	struct zxdg_output_v1 *xdg_output;
	struct wl_surface *discovery_surface;
} NeruWaylandScreen;

typedef struct NeruWlrootsClient {
	struct wl_display *display;
	struct wl_registry *registry;
	struct wl_compositor *compositor;
	struct wl_shm *shm;
	struct zwlr_layer_shell_v1 *layer_shell;
	struct wl_seat *seat;
	struct wl_pointer *pointer;

	struct zwlr_virtual_pointer_manager_v1 *vptr_mgr;
	struct zwlr_virtual_pointer_v1 *vptr;
	struct zwp_virtual_keyboard_manager_v1 *vkeyboard_mgr;
	struct zwp_virtual_keyboard_v1 *vkeyboard;
	int vkeyboard_ready;
	struct zxdg_output_manager_v1 *xdg_output_mgr;

	struct xkb_context *xkb_ctx;
	struct xkb_keymap *xkb_keymap;
	uint32_t mod_shift;
	uint32_t mod_ctrl;
	uint32_t mod_alt;
	uint32_t mod_logo;
	uint32_t depressed_mods;

	NeruWaylandScreen screens[NERU_MAX_OUTPUTS];
	int nr_screens;

	int cursor_x;
	int cursor_y;
	int cursor_initialized;

	int connected;
} NeruWlrootsClient;

NeruWlrootsClient *neru_wlr_connect(void);
void neru_wlr_disconnect(NeruWlrootsClient *c);
void neru_wlr_init_cursor(NeruWlrootsClient *c);
int neru_wlr_move_absolute(NeruWlrootsClient *c, int x, int y);
int neru_wlr_button(NeruWlrootsClient *c, int button, int pressed);
int neru_wlr_click(NeruWlrootsClient *c, int button);
int neru_wlr_scroll(NeruWlrootsClient *c, int axis, int delta, int discrete);
int neru_wlr_modifier_event(NeruWlrootsClient *c, const char *modifier, int is_down);
int neru_wlr_get_cursor(NeruWlrootsClient *c, int *x, int *y);
int neru_wlr_screen_count(NeruWlrootsClient *c);
int neru_wlr_screen_info(
	NeruWlrootsClient *c, int idx, int *x, int *y, int *w, int *h, char *name_out, int name_len);
int neru_wlr_has_virtual_pointer(NeruWlrootsClient *c);
int neru_wlr_has_virtual_keyboard(NeruWlrootsClient *c);

#endif /* WLROOTS_CLIENT_H */

#ifndef EVDEV_H
#define EVDEV_H

#include <linux/input.h>
#include <stddef.h>
#include <sys/types.h>

int neru_evdev_grab(int fd, int grab);
int neru_evdev_key_down(int fd, unsigned int keycode);
int neru_evdev_led_is_on(int fd, unsigned int led);
int neru_evdev_is_keyboard(int fd);
int neru_evdev_get_name(int fd, char *name, size_t name_size);
int neru_evdev_get_bustype(int fd);
int neru_evdev_is_neru_device(int fd);
ssize_t neru_evdev_read_event(int fd, struct input_event *event);
int neru_evdev_get_pressed_keys(int fd, unsigned int *out_keys, int max_keys);
int neru_uinput_create_scroll(int *out_fd);
int neru_uinput_probe_scroll(void);
int neru_uinput_scroll(int fd, int axis, int value);
int neru_uinput_scroll_batch(int fd, int axis, int *values, int count);
int neru_uinput_create_keyboard(int *out_fd);
int neru_uinput_key(int fd, int keycode, int pressed);

/* The keyboard proxy: a uinput keyboard every grabbed physical keyboard is
 * re-emitted through, so the compositor's libinput only ever sees one device
 * that Neru controls. */
int neru_evdev_has_pointer_axes(int fd);
int neru_evdev_get_key_state(int fd, unsigned long *bits, size_t bits_size);
int neru_uinput_create_proxy_keyboard(int *out_fd);
int neru_uinput_destroy(int fd);
ssize_t neru_evdev_write_event(int fd, const struct input_event *event);
ssize_t neru_evdev_write_events(int fd, const struct input_event *events, int count);

#endif /* EVDEV_H */

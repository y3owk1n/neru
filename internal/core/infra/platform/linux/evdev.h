#ifndef EVDEV_H
#define EVDEV_H

#include <linux/input.h>
#include <stddef.h>
#include <sys/types.h>

int neru_evdev_grab(int fd, int grab);
int neru_evdev_key_down(int fd, unsigned int keycode);
int neru_evdev_is_keyboard(int fd);
int neru_evdev_get_name(int fd, char *name, size_t name_size);
int neru_evdev_get_bustype(int fd);
ssize_t neru_evdev_read_event(int fd, struct input_event *event);
int neru_uinput_create_scroll(int *out_fd);
int neru_uinput_scroll(int fd, int axis, int value);
int neru_uinput_scroll_batch(int fd, int axis, int *values, int count);

#endif /* EVDEV_H */

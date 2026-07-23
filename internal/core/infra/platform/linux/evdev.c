#include "evdev.h"

#include <errno.h>
#include <fcntl.h>
#include <linux/input.h>
#include <linux/uinput.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <unistd.h>

int neru_evdev_grab(int fd, int grab) { return ioctl(fd, EVIOCGRAB, grab); }

int neru_evdev_key_down(int fd, unsigned int keycode) {
	unsigned long key_bits[(KEY_MAX + 8 * sizeof(unsigned long)) / (8 * sizeof(unsigned long))];
	memset(key_bits, 0, sizeof(key_bits));

	if (ioctl(fd, EVIOCGKEY(sizeof(key_bits)), key_bits) < 0) {
		return 0;
	}

	return (key_bits[keycode / (8 * sizeof(unsigned long))] >> (keycode % (8 * sizeof(unsigned long)))) & 1UL;
}

int neru_evdev_is_keyboard(int fd) {
	unsigned long key_bits[(KEY_MAX + 8 * sizeof(unsigned long)) / (8 * sizeof(unsigned long))];
	memset(key_bits, 0, sizeof(key_bits));

	if (ioctl(fd, EVIOCGBIT(EV_KEY, sizeof(key_bits)), key_bits) < 0) {
		return 0;
	}

#define NERU_TEST_KEY(bits, key)                                                                                       \
	((bits[(key) / (8 * sizeof(unsigned long))] >> ((key) % (8 * sizeof(unsigned long)))) & 1UL)

	return NERU_TEST_KEY(key_bits, KEY_Q) && NERU_TEST_KEY(key_bits, KEY_W) && NERU_TEST_KEY(key_bits, KEY_E) &&
	       NERU_TEST_KEY(key_bits, KEY_R) && NERU_TEST_KEY(key_bits, KEY_SPACE) && NERU_TEST_KEY(key_bits, KEY_ENTER);
}

int neru_evdev_get_name(int fd, char *name, size_t name_size) {
	int r = ioctl(fd, EVIOCGNAME(name_size), name);
	if (r < 0)
		return -1;
	return r;
}

int neru_evdev_get_bustype(int fd) {
	struct input_id id;
	if (ioctl(fd, EVIOCGID, &id) < 0) {
		return -1;
	}
	return id.bustype;
}

ssize_t neru_evdev_read_event(int fd, struct input_event *event) {
	ssize_t n;
	do {
		n = read(fd, event, sizeof(struct input_event));
	} while (n < 0 && errno == EINTR);
	return n;
}

int neru_evdev_get_pressed_keys(int fd, unsigned int *out_keys, int max_keys) {
	unsigned long key_bits[(KEY_MAX + 8 * sizeof(unsigned long)) / (8 * sizeof(unsigned long))];
	memset(key_bits, 0, sizeof(key_bits));

	if (ioctl(fd, EVIOCGKEY(sizeof(key_bits)), key_bits) < 0) {
		return -1;
	}

	int count = 0;
	for (unsigned int i = 0; i < KEY_MAX; i++) {
		int idx = i / (8 * (int)sizeof(unsigned long));
		int bit = i % (8 * (int)sizeof(unsigned long));
		if ((key_bits[idx] >> bit) & 1UL) {
			if (count < max_keys) {
				out_keys[count] = i;
			}
			count++;
		}
	}
	return count;
}

int neru_uinput_create_scroll(int *out_fd) {
	int fd = open("/dev/uinput", O_RDWR);
	if (fd < 0) {
		fd = open("/dev/input/uinput", O_RDWR);
	}
	if (fd < 0) {
		return 0;
	}

	if (ioctl(fd, UI_SET_EVBIT, EV_REL) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_WHEEL) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_HWHEEL) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_WHEEL_HI_RES) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_HWHEEL_HI_RES) < 0) {
		close(fd);
		return 0;
	}

	struct uinput_setup usetup;
	memset(&usetup, 0, sizeof(usetup));
	usetup.id.bustype = BUS_USB;
	usetup.id.vendor = 0x1234;
	usetup.id.product = 0x5678;
	strcpy(usetup.name, "neru-scroll");
	if (ioctl(fd, UI_DEV_SETUP, &usetup) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_DEV_CREATE) < 0) {
		close(fd);
		return 0;
	}

	*out_fd = fd;
	return 1;
}

int neru_uinput_scroll(int fd, int axis, int value) {
	struct input_event ev;
	memset(&ev, 0, sizeof(ev));

	ev.type = EV_REL;
	ev.code = (axis == 0) ? REL_WHEEL_HI_RES : REL_HWHEEL_HI_RES;
	ev.value = value * 120;
	ssize_t w1 = write(fd, &ev, sizeof(ev));

	memset(&ev, 0, sizeof(ev));
	ev.type = EV_REL;
	ev.code = (axis == 0) ? REL_WHEEL : REL_HWHEEL;
	ev.value = value;
	ssize_t w2 = write(fd, &ev, sizeof(ev));

	memset(&ev, 0, sizeof(ev));
	ev.type = EV_SYN;
	ev.code = SYN_REPORT;
	ev.value = 0;
	ssize_t w3 = write(fd, &ev, sizeof(ev));

	return (w1 == sizeof(ev) && w2 == sizeof(ev) && w3 == sizeof(ev)) ? 1 : 0;
}

int neru_uinput_scroll_batch(int fd, int axis, int *values, int count) {
	if (fd < 0 || !values || count <= 0)
		return 0;

	size_t event_size = sizeof(struct input_event);
	size_t total = (size_t)count * 3 * event_size;
	struct input_event *events = (struct input_event *)malloc(total);
	if (!events)
		return 0;

	int rel_code = (axis == 0) ? REL_WHEEL : REL_HWHEEL;
	int rel_hi_code = (axis == 0) ? REL_WHEEL_HI_RES : REL_HWHEEL_HI_RES;

	for (int i = 0; i < count; i++) {
		int base = i * 3;
		int v = values[i];

		memset(&events[base], 0, event_size);
		events[base].type = EV_REL;
		events[base].code = rel_hi_code;
		events[base].value = v * 120;

		memset(&events[base + 1], 0, event_size);
		events[base + 1].type = EV_REL;
		events[base + 1].code = rel_code;
		events[base + 1].value = v;

		memset(&events[base + 2], 0, event_size);
		events[base + 2].type = EV_SYN;
		events[base + 2].code = SYN_REPORT;
		events[base + 2].value = 0;
	}

	ssize_t written = write(fd, events, total);
	free(events);
	return (written == (ssize_t)total) ? 1 : 0;
}

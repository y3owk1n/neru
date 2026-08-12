#include "shm_file.h"

#include <errno.h>
#include <stdlib.h>
#include <sys/syscall.h>
#include <sys/types.h>
#include <unistd.h>

// ftruncate is interruptible, and a signal landing mid-call leaves the
// descriptor at its old length rather than the size the caller asked for —
// which surfaces later as a short mapping, not as an error. Retrying on EINTR
// is what every caller wants, so it lives here once.
//
// A size above OFF_T_MAX wraps to a negative length, which ftruncate rejects
// with EINVAL; no caller can reach it, and the failure is loud if one ever
// does.
static int neru_shm_file_truncate(int fd, size_t size) {
	int rc;

	do {
		rc = ftruncate(fd, (off_t)size);
	} while (rc < 0 && errno == EINTR);

	return rc;
}

int neru_shm_file_create(const char *name, size_t size) {
#ifdef __NR_memfd_create
	int memfd = (int)syscall(__NR_memfd_create, name, 0);
	if (memfd >= 0) {
		if (neru_shm_file_truncate(memfd, size) >= 0) {
			return memfd;
		}

		close(memfd);
	}
#else
	(void)name;
#endif

	// The file is unlinked the moment it exists, so this path is never visible
	// to anything but the mkstemp collision check — `name` labels the memfd
	// above, and there is nothing here for it to label.
	char path[] = "/tmp/neru-shm-XXXXXX";

	int fd = mkstemp(path);
	if (fd < 0) {
		return -1;
	}

	unlink(path);

	if (neru_shm_file_truncate(fd, size) < 0) {
		close(fd);

		return -1;
	}

	return fd;
}

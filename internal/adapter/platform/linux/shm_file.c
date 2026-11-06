#include "shm_file.h"

#include <errno.h>
#include <stdlib.h>
#include <sys/syscall.h>
#include <sys/types.h>
#include <unistd.h>

// ftruncate is interruptible, and a signal mid-call leaves the old length,
// which surfaces later as a short mapping rather than an error, so retry on
// EINTR. A size above OFF_T_MAX wraps negative and fails loudly with EINVAL.
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

	// The file is unlinked the moment it exists, so only mkstemp's collision
	// check ever sees this path.
	char path[] = "/tmp/neru-shm-XXXXXX";

	int fd = mkstemp(path);
	if (fd < 0) {
		return -1;
	}

	// Refusing on a failed unlink keeps a named file of screen pixels out of
	// /tmp. Callers degrade on -1, and a /tmp that can be written but not
	// unlinked is broken enough to fail on.
	if (unlink(path) < 0) {
		close(fd);

		return -1;
	}

	if (neru_shm_file_truncate(fd, size) < 0) {
		close(fd);

		return -1;
	}

	return fd;
}

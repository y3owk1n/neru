#ifndef SHM_FILE_H
#define SHM_FILE_H

#include <stddef.h>

// neru_shm_file_create returns a file descriptor onto `size` bytes of
// anonymous shared memory, ready to hand to wl_shm_create_pool. Returns -1 on
// failure. The caller owns the descriptor and closes it.
//
// The buffer comes from memfd_create where the kernel headers know it, else
// from a file under /tmp unlinked at once. That fallback is unreachable by path
// but backed by whatever /tmp is, so a caller that must never let screen
// content reach a disk needs its own path. None does today, and memfd has been
// in kernel headers since 3.17.
//
// `name` labels the memfd as /memfd:<name> in /proc/<pid>/fd for debugging,
// and must not be NULL.
int neru_shm_file_create(const char *name, size_t size);

#endif /* SHM_FILE_H */

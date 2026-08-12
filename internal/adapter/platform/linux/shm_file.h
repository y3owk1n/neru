#ifndef SHM_FILE_H
#define SHM_FILE_H

#include <stddef.h>

// neru_shm_file_create returns a file descriptor onto `size` bytes of
// anonymous shared memory, ready to hand to wl_shm_create_pool. Returns -1 on
// failure; the caller owns the descriptor and closes it.
//
// The buffer comes from memfd_create where the kernel headers know the
// syscall, and from an immediately unlinked file under /tmp where they do not,
// so every backend gets a buffer on targets that predate memfd. The fallback
// is a real file: unreachable by path the moment it exists, but backed by
// whatever /tmp is, which anonymous memory is not. Every caller writes screen
// content into it, so a caller that must never let those bytes reach a disk
// wants its own path rather than this one — no caller has that constraint
// today, and memfd has been in kernel headers since 3.17.
//
// `name` labels the memfd for debugging — it is what shows up as /memfd:<name>
// in /proc/<pid>/fd — and must not be NULL. The fallback has nothing to label:
// its file is unlinked before anyone could look.
int neru_shm_file_create(const char *name, size_t size);

#endif /* SHM_FILE_H */

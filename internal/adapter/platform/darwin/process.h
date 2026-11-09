//
//  process.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef PROCESS_H
#define PROCESS_H

/// Start path with one argument through posix_spawn, with stdinFd as its
/// stdin, stdoutFd as its stdout and /dev/null as its stderr. The child gets
/// no other descriptor, and starts with no signal blocked or caught.
/// posix_spawn shares none of this process's memory with the child, where
/// fork shares all of it copy-on-write, and macOS goes on counting a page
/// shared that way after it is given back.
/// @param path Executable to start
/// @param arg Its one argument
/// @param stdinFd Descriptor the child reads as stdin
/// @param stdoutFd Descriptor the child writes as stdout
/// @param pid Receives the child's process ID
/// @return 0 on success, or the errno posix_spawn failed with
int NeruSpawnPiped(const char *path, const char *arg, int stdinFd, int stdoutFd, int *pid);

#endif /* PROCESS_H */

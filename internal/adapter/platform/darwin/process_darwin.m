//
//  process_darwin.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "process.h"

#import <fcntl.h>
#import <signal.h>
#import <spawn.h>
#import <unistd.h>

extern char **environ;

int NeruSpawnPiped(const char *path, const char *arg, int stdinFd, int stdoutFd, int *pid) {
	posix_spawn_file_actions_t actions;
	posix_spawn_file_actions_init(&actions);
	posix_spawn_file_actions_adddup2(&actions, stdinFd, STDIN_FILENO);
	posix_spawn_file_actions_adddup2(&actions, stdoutFd, STDOUT_FILENO);
	posix_spawn_file_actions_addopen(&actions, STDERR_FILENO, "/dev/null", O_WRONLY, 0);

	sigset_t none;
	sigset_t all;
	sigemptyset(&none);
	sigfillset(&all);

	posix_spawnattr_t attributes;
	posix_spawnattr_init(&attributes);
	posix_spawnattr_setsigmask(&attributes, &none);
	posix_spawnattr_setsigdefault(&attributes, &all);
	posix_spawnattr_setflags(&attributes, POSIX_SPAWN_CLOEXEC_DEFAULT | POSIX_SPAWN_SETSIGMASK | POSIX_SPAWN_SETSIGDEF);

	char *argv[] = {(char *)path, (char *)arg, NULL};
	pid_t child = 0;
	int err = posix_spawn(&child, path, &actions, &attributes, argv, environ);

	posix_spawn_file_actions_destroy(&actions);
	posix_spawnattr_destroy(&attributes);

	if (err == 0) {
		*pid = child;
	}

	return err;
}

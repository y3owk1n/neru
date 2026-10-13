#include "locale_guard.h"

#include <pthread.h>
#include <time.h>

static pthread_mutex_t neru_locale_guard_mutex = PTHREAD_MUTEX_INITIALIZER;

void neru_locale_guard_lock(void) { pthread_mutex_lock(&neru_locale_guard_mutex); }

void neru_locale_guard_unlock(void) { pthread_mutex_unlock(&neru_locale_guard_mutex); }

int neru_locale_guard_timedlock(int timeout_ms) {
	struct timespec deadline;
	clock_gettime(CLOCK_REALTIME, &deadline);
	deadline.tv_sec += timeout_ms / 1000;
	deadline.tv_nsec += (long)(timeout_ms % 1000) * 1000000L;
	deadline.tv_sec += deadline.tv_nsec / 1000000000L;
	deadline.tv_nsec %= 1000000000L;

	return pthread_mutex_timedlock(&neru_locale_guard_mutex, &deadline);
}

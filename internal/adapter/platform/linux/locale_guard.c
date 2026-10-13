#include "locale_guard.h"

#include <pthread.h>

static pthread_mutex_t neru_locale_guard_mutex = PTHREAD_MUTEX_INITIALIZER;

void neru_locale_guard_lock(void) { pthread_mutex_lock(&neru_locale_guard_mutex); }

void neru_locale_guard_unlock(void) { pthread_mutex_unlock(&neru_locale_guard_mutex); }

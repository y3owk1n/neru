#ifndef LOCALE_GUARD_H
#define LOCALE_GUARD_H

// The process runs in the C locale, and two native pieces depend on that from
// opposite sides. Tesseract asserts the C locale when an engine is created, and
// XIM opens an input method in whatever locale the process has at that moment,
// which has to be the user's for compose rules and for IBus and fcitx. The X11
// hint-search field therefore switches LC_CTYPE for the open and back, and this
// lock keeps an engine from being created inside that window. setlocale is
// process-global, and Xlib reads the global one, so a thread-local locale
// would not help.
void neru_locale_guard_lock(void);
void neru_locale_guard_unlock(void);

// neru_locale_guard_timedlock waits at most timeout_ms for the guard and
// returns 0 once it holds it. The field holds the guard across XOpenIM, which
// waits on an input-method server with no timeout, so a caller that must not
// stall behind a server that stopped answering uses this.
int neru_locale_guard_timedlock(int timeout_ms);

#endif /* LOCALE_GUARD_H */

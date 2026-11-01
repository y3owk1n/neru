//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"testing"
)

// storeUnderTempState builds a store rooted in a throwaway XDG_STATE_HOME, so
// the permission assertions below measure what the store creates rather than
// whatever the developer's real state directory happens to carry.
func storeUnderTempState(t *testing.T) (*fileRestoreTokenStore, string) {
	t.Helper()

	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	store, err := newFileRestoreTokenStore(remoteDesktopTokenBase)
	if err != nil {
		t.Fatalf("newFileRestoreTokenStore(remoteDesktopTokenBase) error = %v", err)
	}

	return store, filepath.Join(state, "neru")
}

// TestFileRestoreTokenStore_SaveWritesATokenOnlyItsOwnerCanRead is the reason
// this store exists rather than a plain os.WriteFile call. A RemoteDesktop
// restore token is a credential that replays a granted input session, so any
// other user on the machine being able to read it would hand them the grant.
func TestFileRestoreTokenStore_SaveWritesATokenOnlyItsOwnerCanRead(t *testing.T) {
	store, dir := storeUnderTempState(t)

	err := store.save("token-abc")
	if err != nil {
		t.Fatalf("save() error = %v", err)
	}

	info, err := os.Stat(store.path)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}

	if got := info.Mode().Perm(); got != restoreTokenFileMode {
		t.Errorf("token file mode = %v, want %v", got, restoreTokenFileMode)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat token directory: %v", err)
	}

	if got := dirInfo.Mode().Perm(); got != restoreTokenDirMode {
		t.Errorf("token directory mode = %v, want %v", got, restoreTokenDirMode)
	}

	if got := store.load(); got != "token-abc" {
		t.Errorf("load() = %q, want %q", got, "token-abc")
	}
}

// TestFileRestoreTokenStore_SaveTightensPermissionsOnALooseFile covers the
// upgrade path. A token file left world-readable by an earlier write (or by a
// user's own tinkering) must not stay that way just because the path already
// exists.
func TestFileRestoreTokenStore_SaveTightensPermissionsOnALooseFile(t *testing.T) {
	store, dir := storeUnderTempState(t)

	err := os.MkdirAll(dir, restoreTokenDirMode)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err = os.WriteFile(store.path, []byte("stale"), 0o644)
	if err != nil {
		t.Fatalf("seed loose token file: %v", err)
	}

	err = store.save("token-xyz")
	if err != nil {
		t.Fatalf("save() error = %v", err)
	}

	info, err := os.Stat(store.path)
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}

	if got := info.Mode().Perm(); got != restoreTokenFileMode {
		t.Errorf("token file mode = %v, want %v", got, restoreTokenFileMode)
	}

	if got := store.load(); got != "token-xyz" {
		t.Errorf("load() = %q, want %q", got, "token-xyz")
	}
}

// TestFileRestoreTokenStore_LoadReportsNoTokenWhenNoneIsStored pins the
// first-run answer. An absent file is the ordinary state before the first
// grant, not a failure, and the caller reads it as "prompt the user".
func TestFileRestoreTokenStore_LoadReportsNoTokenWhenNoneIsStored(t *testing.T) {
	store, _ := storeUnderTempState(t)

	if got := store.load(); got != "" {
		t.Errorf("load() = %q, want empty", got)
	}
}

// TestFileRestoreTokenStore_ClearRemovesTheStoredToken pins both halves of the
// refusal path: the dead token goes away, and clearing again is not an error.
// The caller clears on every refusal without first asking whether it stored
// anything.
func TestFileRestoreTokenStore_ClearRemovesTheStoredToken(t *testing.T) {
	store, _ := storeUnderTempState(t)

	err := store.save("token-abc")
	if err != nil {
		t.Fatalf("save() error = %v", err)
	}

	err = store.clear()
	if err != nil {
		t.Fatalf("clear() error = %v", err)
	}

	if got := store.load(); got != "" {
		t.Errorf("load() after clear = %q, want empty", got)
	}

	err = store.clear()
	if err != nil {
		t.Errorf("clear() on an already-cleared store error = %v, want nil", err)
	}
}

// TestFileRestoreTokenStore_LoadIgnoresSurroundingWhitespace keeps a token
// written with a trailing newline, by us or by a user inspecting the file,
// from being replayed with the newline attached, which the portal would refuse.
func TestFileRestoreTokenStore_LoadIgnoresSurroundingWhitespace(t *testing.T) {
	store, dir := storeUnderTempState(t)

	err := os.MkdirAll(dir, restoreTokenDirMode)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err = os.WriteFile(store.path, []byte("  token-abc\n"), restoreTokenFileMode)
	if err != nil {
		t.Fatalf("seed token file: %v", err)
	}

	if got := store.load(); got != "token-abc" {
		t.Errorf("load() = %q, want %q", got, "token-abc")
	}
}

// onDesktop records backend as the desktop the stored grants belong to for the
// length of the test.
func onDesktop(t *testing.T, backend string) {
	t.Helper()

	previous := tokenDesktop.Load()

	setTokenDesktop(backend)
	t.Cleanup(func() { tokenDesktop.Store(previous) })
}

// storeOn builds the screen-cast token store backend's desktop uses.
func storeOn(t *testing.T, backend string) *fileRestoreTokenStore {
	t.Helper()

	onDesktop(t, backend)

	store, err := newFileRestoreTokenStore(screenCastTokenBase)
	if err != nil {
		t.Fatalf("newFileRestoreTokenStore(screenCastTokenBase) error = %v", err)
	}

	return store
}

// TestFileRestoreTokenStore_KeepsEachDesktopsGrantApart pins that a grant one
// desktop's portal issued is never presented to another's. KDE and GNOME each
// keep their own token, so switching between them restores each grant without
// a prompt instead of overwriting one with the other.
func TestFileRestoreTokenStore_KeepsEachDesktopsGrantApart(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	err := storeOn(t, backendWaylandKDE).save("kde-grant")
	if err != nil {
		t.Fatalf("save() on KDE error = %v", err)
	}

	gnome := storeOn(t, backendWaylandGNOME)
	if got := gnome.load(); got != "" {
		t.Fatalf("load() on GNOME = %q, want no token before GNOME granted one", got)
	}

	err = gnome.save("gnome-grant")
	if err != nil {
		t.Fatalf("save() on GNOME error = %v", err)
	}

	if got := storeOn(t, backendWaylandKDE).load(); got != "kde-grant" {
		t.Errorf("load() on KDE = %q, want %q", got, "kde-grant")
	}

	if got := storeOn(t, backendWaylandGNOME).load(); got != "gnome-grant" {
		t.Errorf("load() on GNOME = %q, want %q", got, "gnome-grant")
	}
}

// TestFileRestoreTokenStore_ReadsTheSharedTokenFromBeforeUntilItIsRefused pins
// the upgrade. A desktop with no token of its own presents the one every
// desktop shared before, so the grant a user already has is restored instead
// of prompting, and clearing a refused grant removes that shared token too.
func TestFileRestoreTokenStore_ReadsTheSharedTokenFromBeforeUntilItIsRefused(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	dir := filepath.Join(state, "neru")

	err := os.MkdirAll(dir, restoreTokenDirMode)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	shared := filepath.Join(dir, screenCastTokenBase+restoreTokenSuffix)

	err = os.WriteFile(shared, []byte("shared-grant"), restoreTokenFileMode)
	if err != nil {
		t.Fatalf("write shared token: %v", err)
	}

	store := storeOn(t, backendWaylandKDE)
	if got := store.load(); got != "shared-grant" {
		t.Fatalf("load() = %q, want the shared token %q", got, "shared-grant")
	}

	err = store.clear()
	if err != nil {
		t.Fatalf("clear() error = %v", err)
	}

	_, err = os.Stat(shared)
	if !os.IsNotExist(err) {
		t.Errorf("shared token still on disk after clear(), stat error = %v", err)
	}

	if got := store.load(); got != "" {
		t.Errorf("load() after clear() = %q, want no token", got)
	}
}

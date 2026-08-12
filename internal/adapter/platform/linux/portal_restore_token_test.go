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

	store, err := newFileRestoreTokenStore()
	if err != nil {
		t.Fatalf("newFileRestoreTokenStore() error = %v", err)
	}

	return store, filepath.Join(state, "neru")
}

// TestFileRestoreTokenStore_SaveWritesATokenOnlyItsOwnerCanRead is the reason
// this store exists rather than a plain os.WriteFile call: a RemoteDesktop
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
// upgrade path: a token file left world-readable by an earlier write (or by a
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
// first-run answer: an absent file is the ordinary state before the first
// grant, not a failure, and the caller reads it as "prompt the user".
func TestFileRestoreTokenStore_LoadReportsNoTokenWhenNoneIsStored(t *testing.T) {
	store, _ := storeUnderTempState(t)

	if got := store.load(); got != "" {
		t.Errorf("load() = %q, want empty", got)
	}
}

// TestFileRestoreTokenStore_ClearRemovesTheStoredToken pins both halves of the
// refusal path: the dead token goes away, and clearing again is not an error —
// the caller clears on every refusal without first asking whether it stored
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
// written with a trailing newline — by us, or by a user inspecting the file —
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

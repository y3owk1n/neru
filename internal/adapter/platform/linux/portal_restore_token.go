//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// Persistence for portal restore tokens, which turn KDE's consent prompt into
// a one-time grant. The portal returns one for a session started with
// persist_mode, and presenting it later restores the session without asking.
//
// RemoteDesktop (input) and ScreenCast (capture) are separate consents, so each
// gets its own file. One file would make revoking one silently lose the other.
//
// A token is a credential. It replays a grant over this user's session, so it
// is stored owner-only under the user's state directory and is never logged,
// put in an error message, or reported by `neru doctor`.
const (
	// restoreTokenFileMode keeps the token readable by its owner alone.
	restoreTokenFileMode os.FileMode = 0o600
	// restoreTokenDirMode keeps the token's directory owner-only too.
	restoreTokenDirMode os.FileMode = 0o700
	// remoteDesktopTokenBase and screenCastTokenBase start the token file names
	// under XDG_STATE_HOME/neru. A token is machine-specific state that should
	// survive restarts, not configuration.
	remoteDesktopTokenBase = "remote-desktop"
	screenCastTokenBase    = "screen-cast"
	// restoreTokenSuffix ends every token file's name.
	restoreTokenSuffix = ".token"
)

// tokenDesktop is the backend label (wayland-kde, wayland-gnome) whose portal
// issued the stored grants. Each desktop's portal honors only its own tokens,
// so each gets its own file, and a desktop switch never presents KDE's grant
// to GNOME. NewSystemAdapter records it once, since it is fixed per process.
var tokenDesktop atomic.Pointer[string]

// setTokenDesktop records the backend whose portal the stored grants belong
// to.
func setTokenDesktop(backend string) {
	tokenDesktop.Store(&backend)
}

// restoreTokenStore holds the restore token across daemon restarts. It is an
// interface so the establish policy is testable without a filesystem.
type restoreTokenStore interface {
	// load returns the stored token, or "" when none is stored or it cannot be
	// read. Both mean "ask the portal to prompt".
	load() string
	// save replaces the stored token.
	save(token string) error
	// clear removes the stored token. Clearing when nothing is stored is not an
	// error, so callers clear on refusal without first asking what they hold.
	clear() error
}

// fileRestoreTokenStore stores the token in a file under Neru's state
// directory.
type fileRestoreTokenStore struct {
	path string
	// legacyPath is the file all desktops shared before each got its own. load
	// falls back to it so an upgrade keeps its grant, and clear removes it too.
	// It equals path when no desktop was recorded.
	legacyPath string
}

var _ restoreTokenStore = (*fileRestoreTokenStore)(nil)

// newFileRestoreTokenStore resolves grant's token path for the recorded
// desktop under XDG_STATE_HOME, as LogDir does. It creates nothing, so a
// session that never reaches the portal leaves no trace.
func newFileRestoreTokenStore(grant string) (*fileRestoreTokenStore, error) {
	base, err := xdgDir("XDG_STATE_HOME", ".local", "state")
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(base, "neru")
	legacy := filepath.Join(dir, grant+restoreTokenSuffix)

	path := legacy
	if desktop := tokenDesktop.Load(); desktop != nil && *desktop != "" {
		path = filepath.Join(dir, grant+"."+*desktop+restoreTokenSuffix)
	}

	return &fileRestoreTokenStore{path: path, legacyPath: legacy}, nil
}

// load reads the stored token, trimmed so a trailing newline never reaches the
// portal, falling back to the legacy shared file.
func (s *fileRestoreTokenStore) load() string {
	contents, err := os.ReadFile(s.path)
	if err != nil {
		contents, err = os.ReadFile(s.legacyPath)
	}

	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(contents))
}

// save writes the token owner-readable only, via a temp file renamed over the
// target. The new file gets restoreTokenFileMode whatever the old one had, and
// a crash mid-write leaves the previous token intact rather than truncated.
func (s *fileRestoreTokenStore) save(token string) error {
	dir := filepath.Dir(s.path)

	err := os.MkdirAll(dir, restoreTokenDirMode)
	if err != nil {
		return err
	}

	// The temporary file is named after the token it will become, so two stores
	// saving at once cannot collide and a leftover is traceable to its grant.
	temp, err := os.CreateTemp(dir, "."+filepath.Base(s.path)+".*")
	if err != nil {
		return err
	}

	// From here on the temporary file must not be left behind on any path.
	defer func() { _ = os.Remove(temp.Name()) }()

	err = temp.Chmod(restoreTokenFileMode)
	if err != nil {
		_ = temp.Close()

		return err
	}

	_, err = temp.WriteString(token)
	if err != nil {
		_ = temp.Close()

		return err
	}

	err = temp.Close()
	if err != nil {
		return err
	}

	return os.Rename(temp.Name(), s.path)
}

// clear removes the stored token and the shared one from before, treating an
// already-absent file as success.
func (s *fileRestoreTokenStore) clear() error {
	for _, path := range []string{s.path, s.legacyPath} {
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

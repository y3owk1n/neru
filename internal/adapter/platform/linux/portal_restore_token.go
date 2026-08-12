//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"strings"
)

// Persistence for a portal session's restore token.
//
// The token is what turns KDE's consent prompt from a per-start interruption
// into a one-time grant: the portal hands one back when a session is started
// with persist_mode set, and a later session that presents it is restored
// without asking the user again.
//
// There are two of them, one per grant, because there are two consents. The
// RemoteDesktop token replays permission to move the pointer and press keys;
// the ScreenCast token replays permission to read the screen. Storing them in
// one file would mean a user who revokes one silently losing the other, and
// would tie two dialogs together that KDE deliberately asks separately.
//
// A token is a credential, and it is treated as one. Presenting one replays a
// grant over this user's session, so it is written owner-readable only, under
// the user's own state directory, and it is never logged, never put in an error
// message, and never reported through `neru doctor`. Nothing in this file or
// its callers formats the value into anything a human or a log file will see.
const (
	// restoreTokenFileMode keeps the token readable by its owner alone.
	restoreTokenFileMode os.FileMode = 0o600
	// restoreTokenDirMode keeps the directory holding it owner-only too, so a
	// token is not discoverable by listing the directory either.
	restoreTokenDirMode os.FileMode = 0o700
	// remoteDesktopTokenFileName holds the input grant's token, and
	// screenCastTokenFileName the capture grant's, inside Neru's state
	// directory. XDG_STATE_HOME is the right base: a token is state that should
	// survive a restart, is specific to this machine's portal, and is neither
	// configuration the user edits nor data worth carrying to another host.
	remoteDesktopTokenFileName = "remote-desktop.token"
	screenCastTokenFileName    = "screen-cast.token"
)

// restoreTokenStore holds the portal grant's restore token across daemon
// restarts. It is an interface so the establish policy — restore, and prompt
// once when the stored token is refused — is testable without a filesystem.
type restoreTokenStore interface {
	// load returns the stored token, or "" when none is stored or it cannot be
	// read. An unreadable token is indistinguishable from an absent one to the
	// caller by design: both mean "ask the portal to prompt".
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
}

var _ restoreTokenStore = (*fileRestoreTokenStore)(nil)

// newFileRestoreTokenStore resolves fileName's path from the XDG base
// directories, honoring XDG_STATE_HOME exactly as LogDir does. It creates
// nothing: the directory is made on the first save, so a session that never
// reaches the portal leaves no trace.
func newFileRestoreTokenStore(fileName string) (*fileRestoreTokenStore, error) {
	base, err := xdgDir("XDG_STATE_HOME", ".local", "state")
	if err != nil {
		return nil, err
	}

	return &fileRestoreTokenStore{
		path: filepath.Join(base, "neru", fileName),
	}, nil
}

// load reads the stored token, trimming surrounding whitespace so a trailing
// newline never reaches the portal as part of the value.
func (s *fileRestoreTokenStore) load() string {
	contents, err := os.ReadFile(s.path)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(contents))
}

// save writes the token owner-readable only.
//
// It writes a fresh temporary file and renames it over the target rather than
// truncating in place, for two reasons: the new file carries this package's
// mode regardless of what the old one had, so a token left loose by an earlier
// version or by a user's editor is tightened rather than inherited; and a
// crash mid-write leaves the previous token intact instead of a truncated one
// the portal would refuse.
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

// clear removes the stored token, treating an already-absent file as success.
func (s *fileRestoreTokenStore) clear() error {
	err := os.Remove(s.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

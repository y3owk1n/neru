//go:build linux

package gnomeshell

import (
	"bytes"
	"context"
	"embed"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// UUID is the extension's identity in GNOME Shell, the directory it lives in
// and the argument gnome-extensions takes.
const UUID = "neru@y3owk1n.github.io"

const (
	extensionDirMode  = 0o755
	extensionFileMode = 0o644
	enableTimeout     = 3 * time.Second
	// The shell enables at login whatever this setting lists, whether or not
	// it has seen the extension yet, which gnome-extensions enable refuses to
	// do for an extension the running shell has not loaded.
	settingsCommand = "gsettings"
	settingsSchema  = "org.gnome.shell"
	settingsKey     = "enabled-extensions"
)

// extensionFiles is the extension as shipped: the files under extension/ are
// written verbatim into the user's extensions directory.
//
//go:embed extension/extension.js extension/metadata.json
var extensionFiles embed.FS

var extensionFileNames = []string{"extension.js", "metadata.json"}

// extensionDir is where the shell looks for a user-installed extension with
// this UUID: $XDG_DATA_HOME/gnome-shell/extensions/<uuid>, with the XDG
// default of ~/.local/share when the variable is unset.
func extensionDir() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}

		dataHome = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(dataHome, "gnome-shell", "extensions", UUID)
}

// Install writes the extension into dir, replacing files whose contents
// differ, and reports whether anything was written. An up-to-date install
// writes nothing, so a daemon restart touches nothing on disk.
func Install(dir string) (bool, error) {
	err := os.MkdirAll(dir, extensionDirMode)
	if err != nil {
		return false, err
	}

	changed := false

	for _, name := range extensionFileNames {
		want, err := extensionFiles.ReadFile("extension/" + name)
		if err != nil {
			return changed, err
		}

		path := filepath.Join(dir, name)

		have, err := os.ReadFile(path)
		if err == nil && bytes.Equal(have, want) {
			continue
		}

		err = os.WriteFile(path, want, extensionFileMode)
		if err != nil {
			return changed, err
		}

		changed = true
	}

	return changed, nil
}

// installIfShellIsUp puts the extension files in place when GNOME Shell is on
// the bus and the extension is not, once per process, and lists it in the
// shell's enabled extensions so the next login loads it. It is reached only
// after the extension was found absent, so it never touches an install that
// is serving. What it did comes back as a sentence for the reason the caller
// records, so the one warning and the doctor detail both carry the next step.
//
// A session with no shell on the bus is left alone: a GNOME-labeled session
// with nothing to load the files would gain a directory for nobody to read.
func (b *Bridge) installIfShellIsUp(conn *dbus.Conn) string {
	b.startMu.Lock()
	dir, done := b.dataDir, b.installed
	b.installed = true
	b.startMu.Unlock()

	if done || dir == "" {
		return ""
	}

	shellUp, err := nameHasOwner(conn, shellName)
	if err != nil || !shellUp {
		return ""
	}

	// Only a first install is enabled here. A copy that is already on disk
	// and not on the bus is one the user disabled, or one the next login
	// has not loaded yet, and neither is Neru's to switch back on.
	fresh := !Installed(dir)

	changed, err := Install(dir)
	if err != nil {
		return "it could not be installed into " + dir + ": " + err.Error()
	}

	if !fresh {
		verb := "it is installed in "
		if changed {
			verb = "it has been updated in "
		}

		return verb + dir + "; if it is enabled, log out and back in to load it, " +
			"otherwise run `gnome-extensions enable " + UUID + "`"
	}

	if !enable() {
		return "it has been installed into " + dir + "; log out and back in, then run " +
			"`gnome-extensions enable " + UUID + "` to load it"
	}

	return "it has been installed into " + dir + " and enabled; log out and back in to load it"
}

// Installed reports whether an extension is on disk at dir, whatever its
// version: the file the shell requires to list it is there.
func Installed(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "metadata.json"))

	return err == nil
}

// enable lists the extension in the shell's enabled-extensions setting so the
// next login loads it, and reports whether it is listed afterwards. Best
// effort: the setting is the shell's, and a session without gsettings is told
// to log out and back in either way.
func enable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), enableTimeout)
	defer cancel()

	current, err := exec.CommandContext(ctx, settingsCommand, "get", settingsSchema, settingsKey).
		Output()
	if err != nil {
		return false
	}

	uuids := parseStringArray(string(current))
	if slices.Contains(uuids, UUID) {
		return true
	}

	next := formatStringArray(append(uuids, UUID))

	return exec.CommandContext(ctx, settingsCommand, "set", settingsSchema, settingsKey, next).
		Run() ==
		nil
}

// parseStringArray reads the GVariant text gsettings prints for an array of
// strings: ['a', 'b'], or @as [] when empty. Quotes inside a UUID do not
// occur, so a quote is always a boundary.
func parseStringArray(text string) []string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "@as")
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "[")
	text = strings.TrimSuffix(text, "]")

	var items []string

	for item := range strings.SplitSeq(text, ",") {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, "'\"")

		if item != "" {
			items = append(items, item)
		}
	}

	return items
}

func formatStringArray(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = "'" + item + "'"
	}

	return "[" + strings.Join(quoted, ", ") + "]"
}

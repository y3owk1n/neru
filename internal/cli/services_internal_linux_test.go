//go:build linux

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	// testHome and testDefaultUnitPath stand in for a machine that never set
	// $XDG_CONFIG_HOME.
	testHome            = "/home/tester"
	testDefaultUnitPath = testHome + "/.config/systemd/user/neru.service"
)

func TestSystemdIsInit_ReadsTheRuntimeMarker(t *testing.T) {
	runtimeDir := t.TempDir()

	regularFile := filepath.Join(runtimeDir, "file")

	err := os.WriteFile(regularFile, nil, 0o600)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	testCases := []struct {
		name   string
		marker string
		want   bool
	}{
		{name: "marker directory present", marker: runtimeDir, want: true},
		{name: "marker absent", marker: filepath.Join(runtimeDir, "absent"), want: false},
		{name: "marker is a regular file", marker: regularFile, want: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := systemdIsInit(testCase.marker); got != testCase.want {
				t.Errorf("systemdIsInit(%q) = %v, want %v", testCase.marker, got, testCase.want)
			}
		})
	}
}

func TestErrNotSystemd_NamesSystemdAndTheDocs(t *testing.T) {
	err := errNotSystemd("install")

	if !derrors.IsNotSupported(err) {
		t.Fatalf("errNotSystemd() code = %v, want %v", err, derrors.CodeNotSupported)
	}

	message := err.Error()
	for _, want := range []string{"install", "systemd", "runit", "docs/LINUX_SETUP.md"} {
		if !strings.Contains(message, want) {
			t.Errorf("errNotSystemd() message = %q, want it to mention %q", message, want)
		}
	}
}

func TestServiceUnitPath_HonorsXDGConfigHome(t *testing.T) {
	testCases := []struct {
		name          string
		xdgConfigHome string
		home          string
		want          string
	}{
		{
			name:          "absolute XDG_CONFIG_HOME wins",
			xdgConfigHome: "/var/tmp/xdg",
			home:          testHome,
			want:          "/var/tmp/xdg/systemd/user/neru.service",
		},
		{
			name:          "unset falls back to ~/.config",
			xdgConfigHome: "",
			home:          testHome,
			want:          testDefaultUnitPath,
		},
		{
			name:          "relative XDG_CONFIG_HOME is ignored per the spec",
			xdgConfigHome: "relative/config",
			home:          testHome,
			want:          testDefaultUnitPath,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", testCase.xdgConfigHome)
			t.Setenv("HOME", testCase.home)

			got, err := serviceUnitPath()
			if err != nil {
				t.Fatalf("serviceUnitPath() error = %v", err)
			}

			if got != testCase.want {
				t.Errorf("serviceUnitPath() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestRequireOwnUnit_RefusesAUnitNeruDidNotWrite(t *testing.T) {
	dir := t.TempDir()

	ownUnit := filepath.Join(dir, "own.service")

	err := os.WriteFile(ownUnit, []byte("[Unit]\n"), 0o600)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// What nix and home-manager leave at the same path: a link into the store.
	linkedUnit := filepath.Join(dir, "linked.service")

	err = os.Symlink("/nix/store/whatever/neru.service", linkedUnit)
	if err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	testCases := []struct {
		name           string
		unitPath       string
		knownToSystemd bool
		wantErr        bool
	}{
		{name: "a file Neru wrote", unitPath: ownUnit, knownToSystemd: true},
		{
			name:           "a package manager's symlink",
			unitPath:       linkedUnit,
			knownToSystemd: true,
			wantErr:        true,
		},
		{
			name:           "absent here, but systemd knows one elsewhere",
			unitPath:       filepath.Join(dir, "absent.service"),
			knownToSystemd: true,
			wantErr:        true,
		},
		{
			name:     "nothing installed anywhere",
			unitPath: filepath.Join(dir, "absent.service"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotErr := requireOwnUnit(testCase.unitPath, testCase.knownToSystemd)
			if (gotErr != nil) != testCase.wantErr {
				t.Errorf(
					"requireOwnUnit(%q, %v) error = %v, wantErr %v",
					testCase.unitPath,
					testCase.knownToSystemd,
					gotErr,
					testCase.wantErr,
				)
			}
		})
	}
}

func TestUnitDirIsScanned_MatchesWholeDirectories(t *testing.T) {
	// One real `systemctl --user show -p UnitPath --value` answer, trimmed.
	const searchPath = "/home/tester/.config/systemd/user.control " +
		"/home/tester/.config/systemd/user /etc/systemd/user " +
		"/usr/local/share/systemd/user /usr/lib/systemd/user"

	testCases := []struct {
		name    string
		unitDir string
		want    bool
	}{
		{name: "listed directory", unitDir: "/home/tester/.config/systemd/user", want: true},
		{
			name:    "listed, written unclean",
			unitDir: "/home/tester/.config/systemd/user/",
			want:    true,
		},
		{
			name:    "relocated XDG the manager never saw",
			unitDir: "/srv/cfg/systemd/user",
			want:    false,
		},
		{
			name:    "prefix of a listed directory is not a match",
			unitDir: "/home/tester/.config",
			want:    false,
		},
		{
			// A space in the name makes the search path ambiguous, and an
			// ambiguous answer must not refuse an install that would work.
			name:    "directory with a space gets the benefit of the doubt",
			unitDir: "/srv/my configs/systemd/user",
			want:    false,
		},
		{
			name:    "directory with a space that the search path does contain",
			unitDir: "/home/tester/.config/systemd/user /etc",
			want:    true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := unitDirIsScanned(testCase.unitDir, searchPath)
			if got != testCase.want {
				t.Errorf("unitDirIsScanned(%q) = %v, want %v", testCase.unitDir, got, testCase.want)
			}
		})
	}
}

func TestDescribeServiceStatus_NamesEveryState(t *testing.T) {
	const docsURL = "https://example.invalid/LINUX_SETUP.md"

	unitPath := testDefaultUnitPath

	testCases := []struct {
		name  string
		state serviceUnitState
		want  string
	}{
		{
			name:  "no systemd on the machine",
			state: serviceUnitState{unitPath: unitPath},
			want: "Service management requires systemd; this machine was not " +
				"booted by systemd, so there is no user unit to report on — see " +
				docsURL,
		},
		{
			name:  "systemd, but the unit was never installed",
			state: serviceUnitState{systemdBooted: true, unitPath: unitPath},
			want: "Service not installed: no unit at " + unitPath +
				" — run `neru services install` to create it",
		},
		{
			name: "installed, running and enabled",
			state: serviceUnitState{
				systemdBooted: true,
				installed:     true,
				unitPath:      unitPath,
				active:        "active",
				enabled:       "enabled",
			},
			want: "Service installed: active, enabled at login",
		},
		{
			name: "installed, stopped and not enabled",
			state: serviceUnitState{
				systemdBooted: true,
				installed:     true,
				unitPath:      unitPath,
				active:        "inactive",
				enabled:       "disabled",
			},
			want: "Service installed: inactive, disabled at login",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := describeServiceStatus(testCase.state, docsURL)
			if got != testCase.want {
				t.Errorf("describeServiceStatus() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestRenderServiceUnit_AnchorsOnTheGraphicalSession(t *testing.T) {
	unit := renderServiceUnit("/usr/local/bin/neru")

	for _, want := range []string{
		"After=graphical-session.target",
		"PartOf=graphical-session.target",
		"WantedBy=graphical-session.target",
		`ExecStart="/usr/local/bin/neru" launch`,
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("rendered unit is missing %q:\n%s", want, unit)
		}
	}
}

// TestRenderServiceUnit_EscapesTheExecutablePath pins that the path systemd
// executes is the path Neru was run from, byte for byte, whatever is in it.
//
// The two characters that matter are not exotic: a space makes systemd read one
// path as a command plus an argument, and a percent makes it resolve a
// specifier — %h expands, and a specifier systemd does not know fails the whole
// unit load.
func TestRenderServiceUnit_EscapesTheExecutablePath(t *testing.T) {
	testCases := []struct {
		name       string
		binaryPath string
		want       string
	}{
		{
			name:       "an ordinary path is quoted and otherwise untouched",
			binaryPath: "/usr/local/bin/neru",
			want:       `ExecStart="/usr/local/bin/neru" launch`,
		},
		{
			name:       "a space stays one path instead of becoming two words",
			binaryPath: "/opt/my apps/neru",
			want:       `ExecStart="/opt/my apps/neru" launch`,
		},
		{
			name:       "a percent is written as the literal systemd reads back",
			binaryPath: "/opt/100%/neru",
			want:       `ExecStart="/opt/100%%/neru" launch`,
		},
		{
			name:       "a specifier systemd knows is not left for it to expand",
			binaryPath: "/home/%h/bin/neru",
			want:       `ExecStart="/home/%%h/bin/neru" launch`,
		},
		{
			name:       "a backslash is escaped, since quoting turns it into one",
			binaryPath: `/opt/we\ird/neru`,
			want:       `ExecStart="/opt/we\\ird/neru" launch`,
		},
		{
			name:       "a double quote is escaped rather than closing the quoting",
			binaryPath: `/opt/"quoted"/neru`,
			want:       `ExecStart="/opt/\"quoted\"/neru" launch`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			unit := renderServiceUnit(testCase.binaryPath)
			if !strings.Contains(unit, testCase.want) {
				t.Errorf(
					"renderServiceUnit(%q) is missing %q:\n%s",
					testCase.binaryPath,
					testCase.want,
					unit,
				)
			}
		})
	}
}

package describe_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/describe"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

func TestBuild_Here_FollowsThePlatformAsked(t *testing.T) {
	tests := []struct {
		name     string
		platform parity.Platform
		action   string
		want     bool
	}{
		{"a darwin-only action works on darwin", parity.Darwin, "hide_cursor", true},
		{"a darwin-only action does not work on linux", parity.Linux, "hide_cursor", false},
		{"an action declared everywhere works on windows", parity.Windows, "left_click", true},
		{"nothing works on a platform Neru declares nothing about", "", "left_click", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := describe.Build(test.platform)

			for _, described := range manifest.Actions {
				if described.Name != test.action {
					continue
				}

				if described.Here != test.want {
					t.Errorf("%s here = %v, want %v", test.action, described.Here, test.want)
				}

				return
			}

			t.Fatalf("%s is not in the manifest", test.action)
		})
	}
}

func TestBuild_Events_TakeTheirPlatformsFromTheHookKey(t *testing.T) {
	manifest := describe.Build(parity.Linux)

	for _, described := range manifest.Events {
		if described.Name != "mission_control_activated" {
			continue
		}

		if described.Hook != "on_mission_control_activated" {
			t.Errorf("hook = %q", described.Hook)
		}

		if len(described.Variables) == 0 || described.Variables[0] != "NERU_EVENT" {
			t.Errorf("variables = %v, want NERU_EVENT first, since every hook sees it",
				described.Variables)
		}

		if described.Here || described.Note == "" {
			t.Errorf(
				"a macOS-only event reads here=%v note=%q on linux",
				described.Here,
				described.Note,
			)
		}

		return
	}

	t.Fatal("mission_control_activated is not in the manifest")
}

package event_test

import (
	"image"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/event"
)

func TestEvent_Env_SetsWhatTheHooksReferencePromises(t *testing.T) {
	tests := []struct {
		name string
		evt  event.Event
		want []string
	}{
		{
			name: "a canceled exit carries no action",
			evt: event.Event{
				Name: event.ModeExit, Mode: "scroll", Reason: event.ExitCancelled,
			},
			want: []string{"NERU_EVENT=mode_exit", "NERU_MODE=scroll", "NERU_REASON=canceled"},
		},
		{
			name: "a select carries its point",
			evt: event.Event{
				Name: event.Select, Mode: "grid", Action: "left_click", Point: image.Pt(3, 4),
			},
			want: []string{
				"NERU_EVENT=select", "NERU_MODE=grid", "NERU_ACTION=left_click",
				"NERU_X=3", "NERU_Y=4",
			},
		},
		{
			name: "a failed reload says false",
			evt:  event.Event{Name: event.ConfigReload},
			want: []string{"NERU_EVENT=config_reload", "NERU_OK=false"},
		},
		{
			name: "a release reads as no modifiers",
			evt:  event.Event{Name: event.StickyModifiers},
			want: []string{"NERU_EVENT=sticky_modifiers", "NERU_MODIFIERS="},
		},
		{
			name: "a field the event does not carry stays out",
			evt:  event.Event{Name: event.AppFocus, BundleID: "com.example", Mode: "bisect"},
			want: []string{"NERU_EVENT=app_focus", "NERU_BUNDLE_ID=com.example"},
		},
		{
			name: "an event with no variables carries only its name",
			evt:  event.Event{Name: event.Ready},
			want: []string{"NERU_EVENT=ready"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.evt.Env()
			if !slices.Equal(got, test.want) {
				t.Errorf("Env() = %v, want %v", got, test.want)
			}
		})
	}
}

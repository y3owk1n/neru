package services_test

import (
	"context"
	"fmt"
	"image"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports"
	"github.com/y3owk1n/neru/internal/ports/mocks"
)

func TestHintService_RefreshHints(t *testing.T) {
	tests := []struct {
		name           string
		overlayVisible bool
		expectRefresh  bool
		refreshError   error
		wantErr        bool
	}{
		{
			name:           "refresh when visible",
			overlayVisible: true,
			expectRefresh:  true,
			refreshError:   nil,
			wantErr:        false,
		},
		{
			name:           "skip refresh when not visible",
			overlayVisible: false,
			expectRefresh:  false,
			refreshError:   nil,
			wantErr:        false,
		},
		{
			name:           "refresh error when visible",
			overlayVisible: true,
			expectRefresh:  true,
			refreshError:   derrors.New(derrors.CodeOverlayFailed, "overlay refresh failed"),
			wantErr:        true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}

			refreshCalled := false
			mockOverlay.IsVisibleFunc = func() bool {
				return testCase.overlayVisible
			}
			mockOverlay.RefreshFunc = func(_ context.Context) error {
				refreshCalled = true

				return testCase.refreshError
			}

			generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)
			logger := logger.Get()

			service := services.NewHintService(
				mockAcc,
				mockOverlay,
				&mocks.MockSystemPort{},
				generator,
				config.HintsConfig{},
				logger,
				nil,
			)

			ctx := context.Background()
			refreshHintsErr := service.RefreshHints(ctx)

			if (refreshHintsErr != nil) != testCase.wantErr {
				t.Errorf("RefreshHints() error = %v, wantErr %v", refreshHintsErr, testCase.wantErr)
			}

			if refreshCalled != testCase.expectRefresh {
				t.Errorf("Refresh called = %v, want %v", refreshCalled, testCase.expectRefresh)
			}
		})
	}
}

func TestHintService_GenerateHintsVisionCombinesSupplementaryAndWindowElements(
	t *testing.T,
) {
	supplementElement := mustNewElement("menubar", image.Rect(10, 0, 60, 20))
	windowElement := mustNewElement("window", image.Rect(10, 40, 60, 90))

	mockAcc := &mocks.MockAccessibilityPort{}
	mockAcc.ClickableElementsFunc = func(
		_ context.Context,
		filter ports.ElementFilter,
	) ([]*element.Element, error) {
		if !filter.SkipWindowElements {
			t.Error("accessibility should not collect window elements when using vision strategy")

			return nil, nil
		}

		return []*element.Element{supplementElement}, nil
	}

	mockSystem := &mocks.MockSystemPort{}
	mockSystem.FocusedWindowBoundsFunc = func(context.Context) (image.Rectangle, bool, error) {
		return image.Rect(0, 0, 200, 200), true, nil
	}

	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)
	service := services.NewHintService(
		mockAcc,
		&mocks.MockOverlayPort{},
		mockSystem,
		generator,
		config.HintsConfig{
			ClickableRoles:                []string{string(element.SemanticButton)},
			IncludeMenubarHints:           true,
			AdditionalMenubarHintsTargets: []string{"Clock"},
			IncludeDockHints:              true,
			IncludeNCHints:                true,
			IncludeStageManagerHints:      true,
			IncludePIPHints:               true,
			IncludeScreenCaptureHints:     true,
		},
		logger.Get(),
		&mockVisionPort{
			detectedElements: []*element.Element{windowElement},
		},
	)

	hints, err := service.GenerateHints(
		context.Background(),
		nil,
		nil,
		"com.example.app",
		domain.StrategyVision,
		"",
		false,
	)
	if err != nil {
		t.Fatalf("GenerateHints() unexpected error: %v", err)
	}

	if len(hints) != 2 {
		t.Fatalf("GenerateHints() returned %d hints, want 2", len(hints))
	}

	seen := map[element.ID]int{}
	for _, generatedHint := range hints {
		seen[generatedHint.Element().ID()]++
	}

	if seen[supplementElement.ID()] != 1 {
		t.Errorf("supplementary element count = %d, want 1", seen[supplementElement.ID()])
	}

	if seen[windowElement.ID()] != 1 {
		t.Errorf("window element count = %d, want 1", seen[windowElement.ID()])
	}
}

func TestHintService_GenerateHintsVisionWithNilPortReturnsSupplementaryElements(
	t *testing.T,
) {
	supplementElement := mustNewElement("menubar", image.Rect(10, 0, 60, 20))

	mockAcc := &mocks.MockAccessibilityPort{}
	mockAcc.ClickableElementsFunc = func(
		_ context.Context,
		filter ports.ElementFilter,
	) ([]*element.Element, error) {
		if !filter.SkipWindowElements {
			t.Error("nil vision port should not trigger window AX collection")
		}

		return []*element.Element{supplementElement}, nil
	}

	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)
	service := services.NewHintService(
		mockAcc,
		&mocks.MockOverlayPort{},
		&mocks.MockSystemPort{},
		generator,
		config.HintsConfig{
			IncludeMenubarHints: true,
		},
		logger.Get(),
		nil,
	)

	hints, err := service.GenerateHints(
		context.Background(),
		nil,
		nil,
		"com.example.app",
		domain.StrategyVision,
		"",
		false,
	)
	if err != nil {
		t.Fatalf("GenerateHints() unexpected error: %v", err)
	}

	if len(hints) != 1 {
		t.Fatalf("GenerateHints() returned %d hints, want 1", len(hints))
	}

	if hints[0].Element().ID() != supplementElement.ID() {
		t.Errorf("hint element = %q, want %q", hints[0].Element().ID(), supplementElement.ID())
	}
}

// TestHintService_GenerateHintsVisionNotifiesWhenTheStrategyIsUnavailable is
// about a failure a log line cannot fix.
//
// When vision detection reports CodeNotSupported the machine cannot run the
// strategy at all — no tesseract language data, no capture backend, a build
// with no engine in it — and the error names what to install. Swallowing that
// leaves the user pressing the hotkey and getting an overlay with nothing on
// it, since the supplementary elements the pipeline keeps are macOS surfaces
// with no counterpart elsewhere. The message has to reach a person, which is
// what ADR 0002 says a log line does not do.
func TestHintService_GenerateHintsVisionNotifiesWhenTheStrategyIsUnavailable(t *testing.T) {
	const missing = "install the tesseract eng language data"

	notified := make(chan string, 4)

	mockSystem := &mocks.MockSystemPort{}
	mockSystem.FocusedWindowBoundsFunc = func(context.Context) (image.Rectangle, bool, error) {
		return image.Rect(0, 0, 200, 200), true, nil
	}
	mockSystem.ShowNotificationFunc = func(_ context.Context, _, message string) error {
		notified <- message

		return nil
	}

	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	service := services.NewHintService(
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		mockSystem,
		generator,
		config.HintsConfig{},
		logger.Get(),
		&mockVisionPort{detectErr: derrors.New(derrors.CodeNotSupported, missing)},
	)

	for range 3 {
		_, err := service.GenerateHints(
			context.Background(), nil, nil, "com.example.app", domain.StrategyVision, "", false,
		)
		if err != nil {
			t.Fatalf("GenerateHints() unexpected error: %v", err)
		}
	}

	select {
	case message := <-notified:
		if !strings.Contains(message, missing) {
			t.Errorf("notification %q does not carry what the error named", message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("vision reported CodeNotSupported and the user was never told")
	}

	// Three activations, one notification: a user who keeps pressing the hotkey
	// is told once, not once per press.
	select {
	case extra := <-notified:
		t.Errorf("the same failure notified twice: %q", extra)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestHintService_GenerateHintsVisionNoticeSurvivesTheActivationContext is the
// regression test for the way this notification is easiest to lose.
//
// Every caller that reaches vision detection under the mode handler's lock
// builds its context with a hint timeout and cancels it on return, microseconds
// after the notification is queued — and the Linux notification path honors a
// caller's deadline. A notice that carried that context would be canceled
// before the first send, which is also the send that dials the session bus, and
// the user would be told nothing.
func TestHintService_GenerateHintsVisionNoticeSurvivesTheActivationContext(t *testing.T) {
	// proceed holds the send until the activation context has been canceled,
	// so this observes the hazard rather than racing it.
	proceed := make(chan struct{})
	sent := make(chan error, 1)

	mockSystem := &mocks.MockSystemPort{}
	mockSystem.FocusedWindowBoundsFunc = func(context.Context) (image.Rectangle, bool, error) {
		return image.Rect(0, 0, 200, 200), true, nil
	}
	mockSystem.ShowNotificationFunc = func(ctx context.Context, _, _ string) error {
		<-proceed

		sent <- ctx.Err()

		return nil
	}

	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	service := services.NewHintService(
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		mockSystem,
		generator,
		config.HintsConfig{},
		logger.Get(),
		&mockVisionPort{detectErr: derrors.New(derrors.CodeNotSupported, "no language data")},
	)

	ctx, cancel := context.WithCancel(context.Background())

	_, err := service.GenerateHints(
		ctx, nil, nil, "com.example.app", domain.StrategyVision, "", false,
	)
	if err != nil {
		t.Fatalf("GenerateHints() unexpected error: %v", err)
	}

	// What every locked caller does on return.
	cancel()
	close(proceed)

	select {
	case ctxErr := <-sent:
		if ctxErr != nil {
			t.Errorf("the notification was sent on a canceled context: %v", ctxErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the notification was never sent")
	}
}

// TestHintService_GenerateHintsVisionRetriesANoticeThatFailedToSend pins that
// the dedupe remembers a *telling*, not an attempt. A session whose
// notification daemon was not up at the first activation would otherwise be
// silenced for the life of the daemon, which is the same silence the
// notification exists to break.
func TestHintService_GenerateHintsVisionRetriesANoticeThatFailedToSend(t *testing.T) {
	attempts := make(chan struct{}, 4)

	mockSystem := &mocks.MockSystemPort{}
	mockSystem.FocusedWindowBoundsFunc = func(context.Context) (image.Rectangle, bool, error) {
		return image.Rect(0, 0, 200, 200), true, nil
	}
	mockSystem.ShowNotificationFunc = func(context.Context, string, string) error {
		attempts <- struct{}{}

		return derrors.New(derrors.CodeActionFailed, "no notification daemon on the bus")
	}

	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	service := services.NewHintService(
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		mockSystem,
		generator,
		config.HintsConfig{},
		logger.Get(),
		&mockVisionPort{detectErr: derrors.New(derrors.CodeNotSupported, "no language data")},
	)

	// The release of a failed notice happens on the sending goroutine, so which
	// later activation retries is a scheduling detail rather than a promise.
	// What is asserted is that one of them does: a dedupe that remembered the
	// attempt would let none of them.
	seen := 0
	deadline := time.After(5 * time.Second)

	for seen < 2 {
		_, err := service.GenerateHints(
			context.Background(), nil, nil, "com.example.app", domain.StrategyVision, "", false,
		)
		if err != nil {
			t.Fatalf("GenerateHints() unexpected error: %v", err)
		}

		select {
		case <-attempts:
			seen++
		case <-deadline:
			t.Fatalf("a failed notice was never retried: %d attempts", seen)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// TestHintService_GenerateHintsVisionStaysQuietForAnOrdinaryFailure keeps the
// notification for the case a user can act on. A capture that timed out or an
// engine that failed one frame is a transient fault, and a toast on every
// hotkey press for those would be noise rather than help.
func TestHintService_GenerateHintsVisionStaysQuietForAnOrdinaryFailure(t *testing.T) {
	notified := make(chan string, 2)

	mockSystem := &mocks.MockSystemPort{}
	mockSystem.FocusedWindowBoundsFunc = func(context.Context) (image.Rectangle, bool, error) {
		return image.Rect(0, 0, 200, 200), true, nil
	}
	mockSystem.ShowNotificationFunc = func(_ context.Context, _, message string) error {
		notified <- message

		return nil
	}

	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	service := services.NewHintService(
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		mockSystem,
		generator,
		config.HintsConfig{},
		logger.Get(),
		&mockVisionPort{
			detectErr: derrors.New(
				derrors.CodeActionFailed,
				"the compositor did not answer in time",
			),
		},
	)

	_, err := service.GenerateHints(
		context.Background(), nil, nil, "com.example.app", domain.StrategyVision, "", false,
	)
	if err != nil {
		t.Fatalf("GenerateHints() unexpected error: %v", err)
	}

	select {
	case message := <-notified:
		t.Errorf("a transient vision failure notified the user: %q", message)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestHintService_UpdateGenerator(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	log := logger.Get()

	initialGen, err := hint.NewAlphabetGenerator("abcd", hint.LabelDirectionReverse)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error = %v", err)
	}

	service := services.NewHintService(
		mockAcc,
		mockOverlay,
		&mocks.MockSystemPort{},
		initialGen,
		config.HintsConfig{},
		log,
		nil,
	)

	normal, err := hint.NewAlphabetGenerator("efgh", hint.LabelDirectionNormal)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error = %v", err)
	}

	service.UpdateGenerator(context.Background(), normal)

	// The registered generator must be retrievable under its own direction —
	// this is what the per-activation `hints --label-direction` override reads.
	got := service.Generator(hint.LabelDirectionNormal.String())
	if got == nil {
		t.Fatal("Generator(normal) = nil after UpdateGenerator")
	}

	if got.LabelDirection() != hint.LabelDirectionNormal {
		t.Errorf(
			"Generator(normal).LabelDirection() = %v, want %v",
			got.LabelDirection(),
			hint.LabelDirectionNormal,
		)
	}

	// A nil generator must be ignored rather than wiping a live one.
	service.UpdateGenerator(context.Background(), nil)

	if service.Generator(hint.LabelDirectionNormal.String()) == nil {
		t.Error("UpdateGenerator(nil) cleared the previously registered generator")
	}
}

func TestHintService_GeneratorReturnsDirectionSpecificInstance(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()

	reverseGen, _ := hint.NewAlphabetGenerator("abcd", hint.LabelDirectionReverse)
	normalGen, _ := hint.NewAlphabetGenerator("abcd", hint.LabelDirectionNormal)

	service := services.NewHintService(
		mockAcc,
		mockOverlay,
		&mocks.MockSystemPort{},
		reverseGen,
		config.HintsConfig{},
		logger,
		nil,
	)

	// Register a normal-direction generator on top of the reverse default.
	ctx := context.Background()
	service.UpdateGenerator(ctx, normalGen)

	// Each direction must resolve to its own generator instance, not the
	// shared default.
	gotReverse := service.Generator(domain.LabelDirectionReverse)
	if gotReverse == nil {
		t.Fatal("Generator(reverse) returned nil")
	}

	if gotReverse.LabelDirection() != hint.LabelDirectionReverse {
		t.Errorf(
			"Generator(reverse).LabelDirection() = %v, want %v",
			gotReverse.LabelDirection(),
			hint.LabelDirectionReverse,
		)
	}

	gotNormal := service.Generator(domain.LabelDirectionNormal)
	if gotNormal == nil {
		t.Fatal("Generator(normal) returned nil")
	}

	if gotNormal.LabelDirection() != hint.LabelDirectionNormal {
		t.Errorf(
			"Generator(normal).LabelDirection() = %v, want %v",
			gotNormal.LabelDirection(),
			hint.LabelDirectionNormal,
		)
	}

	if gotReverse == gotNormal {
		t.Error("reverse and normal resolved to the same generator instance")
	}

	// Empty direction falls back to the default (reverse) generator.
	gotDefault := service.Generator("")
	if gotDefault == nil {
		t.Fatal("Generator(\"\") returned nil")
	}

	if gotDefault.LabelDirection() != hint.LabelDirectionReverse {
		t.Errorf(
			"Generator(\"\").LabelDirection() = %v, want %v",
			gotDefault.LabelDirection(),
			hint.LabelDirectionReverse,
		)
	}

	// Unknown direction falls back to the default rather than failing.
	gotUnknown := service.Generator("made-up")
	if gotUnknown == nil {
		t.Fatal("Generator(\"made-up\") returned nil")
	}

	if gotUnknown.LabelDirection() != hint.LabelDirectionReverse {
		t.Errorf(
			"Generator(\"made-up\").LabelDirection() = %v, want %v",
			gotUnknown.LabelDirection(),
			hint.LabelDirectionReverse,
		)
	}
}

func TestHintService_GenerateHintsPicksDirectionGenerator(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()

	normalGen, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	reverseGen, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)

	// Five elements force both algorithms into the two-character tier, where
	// reverse and normal produce *different* label sequences. The exact
	// normal sequence is [A S D FA FS]; the exact reverse sequence is
	// [AA SA DA FA AS]. The 4th and 5th labels expose the difference.
	mockAcc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
		return []*element.Element{
			mustNewElement("e1", image.Rect(0, 0, 10, 10)),
			mustNewElement("e2", image.Rect(20, 20, 30, 30)),
			mustNewElement("e3", image.Rect(40, 40, 50, 50)),
			mustNewElement("e4", image.Rect(60, 60, 70, 70)),
			mustNewElement("e5", image.Rect(80, 80, 90, 90)),
		}, nil
	}

	service := services.NewHintService(
		mockAcc,
		mockOverlay,
		&mocks.MockSystemPort{},
		normalGen,
		config.HintsConfig{},
		logger,
		nil,
	)

	ctx := context.Background()
	service.UpdateGenerator(ctx, reverseGen)

	// Without an override, the configured (empty) label direction resolves
	// to the default normal generator. The normal algorithm keeps 3
	// single-char slots ([A S D]) and expands the 4th alphabet slot (F)
	// into 2-char labels starting at [FA].
	hints, err := service.GenerateHints(ctx, nil, nil, "", "", "", false)
	if err != nil {
		t.Fatalf("GenerateHints() unexpected error: %v", err)
	}

	if len(hints) != 5 {
		t.Fatalf("GenerateHints() returned %d hints, want 5", len(hints))
	}

	wantNormalLabels := []string{"A", "S", "D", "FA", "FS"}
	for i, want := range wantNormalLabels {
		if got := hints[i].Label(); got != want {
			t.Errorf("default-direction hint[%d].Label() = %q, want %q", i, got, want)
		}
	}

	// With a reverse override, the override must resolve to the registered
	// reverse generator — not silently fall back to the default normal one.
	// The reverse algorithm fills all 4 single-char slots ([AA SA DA FA])
	// before yielding a 2-char label ([AS]). The 1st and 5th labels (AA, AS)
	// prove the override actually engaged.
	hints, err = service.GenerateHints(ctx, nil, nil, "", "", domain.LabelDirectionReverse, false)
	if err != nil {
		t.Fatalf("GenerateHints() with reverse override unexpected error: %v", err)
	}

	if len(hints) != 5 {
		t.Fatalf(
			"GenerateHints() with reverse override returned %d hints, want 5",
			len(hints),
		)
	}

	wantReverseLabels := []string{"AA", "SA", "DA", "FA", "AS"}
	for i, want := range wantReverseLabels {
		if got := hints[i].Label(); got != want {
			t.Errorf(
				"reverse-override hint[%d].Label() = %q, want %q",
				i,
				got,
				want,
			)
		}
	}
}

func TestHintService_Health(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	generator, _ := hint.NewAlphabetGenerator("abcd", hint.LabelDirectionReverse)
	logger := logger.Get()

	service := services.NewHintService(
		mockAcc,
		mockOverlay,
		&mocks.MockSystemPort{},
		generator,
		config.HintsConfig{},
		logger,
		nil,
	)

	// Setup mocks
	mockAcc.HealthFunc = func(_ context.Context) error {
		return nil
	}
	mockOverlay.HealthFunc = func(_ context.Context) error {
		return derrors.New(derrors.CodeOverlayFailed, "overlay unhealthy")
	}

	ctx := context.Background()
	health := service.Health(ctx)

	if len(health) != 2 {
		t.Errorf("Health() returned %d entries, want 2", len(health))
	}

	if _, ok := health["accessibility"]; !ok {
		t.Error("Health() missing 'accessibility' key")
	}

	if _, ok := health["overlay"]; !ok {
		t.Error("Health() missing 'overlay' key")
	}

	if health["overlay"] == nil {
		t.Error("Health() overlay should have error")
	}

	if health["accessibility"] != nil {
		t.Error("Health() accessibility should not have error")
	}
}

// nativeButtonRole is the accessibility role a button reports on the platform
// running the tests. Configured roles resolve to native names, so an element
// built with a role from another platform would silently stop matching a
// config that asks for "button".
var nativeButtonRole = func() element.Role {
	native := element.ResolveRolesForCurrentPlatform(
		[]string{string(element.SemanticButton)},
	).Native
	if len(native) == 0 {
		return element.RoleButton
	}

	return element.Role(native[0])
}()

func mustNewElement(id string, bounds image.Rectangle) *element.Element {
	element, elementErr := element.NewElement(element.ID(id), bounds, nativeButtonRole)
	if elementErr != nil {
		panic(elementErr)
	}

	return element
}

type mockVisionPort struct {
	detectedElements []*element.Element
	detectErr        error
}

func (m *mockVisionPort) DetectElements(
	context.Context,
	image.Rectangle,
	config.HintsVisionConfig,
	bool,
) ([]*element.Element, error) {
	if m.detectErr != nil {
		return nil, m.detectErr
	}

	return m.detectedElements, nil
}

func (m *mockVisionPort) CaptureScreen(context.Context) (*image.RGBA, error) {
	return nil, derrors.New(derrors.CodeBridgeFailed, "capture screen not implemented")
}

func (m *mockVisionPort) Health(context.Context) error {
	return nil
}

func TestHintService_GenerateHintsRejectsSplitWordForNonVisionStrategy(t *testing.T) {
	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	service := services.NewHintService(
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		&mocks.MockSystemPort{},
		generator,
		config.HintsConfig{
			Strategy: domain.StrategyAXTree,
		},
		logger.Get(),
		nil,
	)

	ctx := context.Background()

	_, err := service.GenerateHints(
		ctx,
		nil,
		nil,
		"",
		domain.StrategyAXTree,
		"",
		true, // splitWord
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !derrors.IsCode(err, derrors.CodeInvalidInput) {
		t.Errorf("expected invalid input error, got: %v", err)
	}
}

// TestHintService_GenerateHintsRoleFilterResolvingToNothing pins the behavior
// of a role filter that is configured but resolves to no native role on this
// platform — for example a config carrying only Linux entries, run on macOS.
//
// An empty ports.ElementFilter.Roles means "match every role", so the naive
// outcome is that an unusable filter hints *everything*. It must hint nothing.
func TestHintService_GenerateHintsRoleFilterResolvingToNothing(t *testing.T) {
	testElements := []*element.Element{
		mustNewElement("elem1", image.Rect(10, 10, 50, 50)),
		mustNewElement("elem2", image.Rect(60, 10, 100, 50)),
	}

	tests := []struct {
		name        string
		roles       []string
		filterRoles []string
	}{
		{
			name:  "configured roles all belong to another platform",
			roles: foreignRolesForCurrentPlatform(),
		},
		{
			name:        "role flag entries are unresolvable",
			roles:       []string{string(element.SemanticButton)},
			filterRoles: []string{"AXButton"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if len(testCase.roles) == 0 {
				t.Skipf("no foreign role set defined for %s", runtime.GOOS)
			}

			mockAcc := &mocks.MockAccessibilityPort{}
			mockAcc.ClickableElementsFunc = func(
				_ context.Context,
				_ ports.ElementFilter,
			) ([]*element.Element, error) {
				return testElements, nil
			}

			generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)
			service := services.NewHintService(
				mockAcc,
				&mocks.MockOverlayPort{},
				&mocks.MockSystemPort{},
				generator,
				config.HintsConfig{ClickableRoles: testCase.roles},
				logger.Get(),
				nil,
			)

			hints, err := service.GenerateHints(
				context.Background(),
				testCase.filterRoles,
				nil,
				"com.example.app",
				"",
				"",
				false,
			)
			if err != nil {
				t.Fatalf("GenerateHints() unexpected error: %v", err)
			}

			if len(hints) != 0 {
				t.Errorf(
					"GenerateHints() returned %d hints for an unusable role filter, want 0",
					len(hints),
				)
			}
		})
	}
}

// TestHintService_GenerateHintsRoleFlagOverridesConfig covers the happy path of
// `neru hints --role ...`: the flag replaces the configured roles entirely and
// is resolved through the same vocabulary, so it accepts semantic names and
// vocabulary-prefixed native names alike.
func TestHintService_GenerateHintsRoleFlagOverridesConfig(t *testing.T) {
	testElements := []*element.Element{
		mustNewElement("elem1", image.Rect(10, 10, 50, 50)),
	}

	var captured []element.Role

	mockAcc := &mocks.MockAccessibilityPort{}
	mockAcc.ClickableElementsFunc = func(
		_ context.Context,
		filter ports.ElementFilter,
	) ([]*element.Element, error) {
		captured = filter.Roles

		return testElements, nil
	}

	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)
	service := services.NewHintService(
		mockAcc,
		&mocks.MockOverlayPort{},
		&mocks.MockSystemPort{},
		generator,
		config.HintsConfig{
			ClickableRoles: []string{string(element.SemanticButton)},
		},
		logger.Get(),
		nil,
	)

	_, err := service.GenerateHints(
		context.Background(),
		[]string{string(element.SemanticLink)},
		nil,
		"com.example.app",
		"",
		"",
		false,
	)
	if err != nil {
		t.Fatalf("GenerateHints() unexpected error: %v", err)
	}

	want := element.ResolveRolesForCurrentPlatform(
		[]string{string(element.SemanticLink)},
	).Native
	if len(want) == 0 {
		t.Skip("link has no native role on this platform")
	}

	for _, role := range want {
		if !slices.Contains(captured, element.Role(role)) {
			t.Errorf("filter.Roles = %v, missing overridden role %q", captured, role)
		}
	}

	// The configured role must not survive the override.
	for _, role := range element.ResolveRolesForCurrentPlatform(
		[]string{string(element.SemanticButton)},
	).Native {
		if slices.Contains(captured, element.Role(role)) {
			t.Errorf("filter.Roles = %v, configured role %q leaked past the override",
				captured, role)
		}
	}
}

// newVisionHintService builds a vision-strategy service whose focused-window
// answer is whatever the caller wants to observe the fallback for.
func newVisionHintService(
	log *zap.Logger,
	bounds func(context.Context) (image.Rectangle, bool, error),
) *services.HintService {
	mockSystem := &mocks.MockSystemPort{}
	mockSystem.FocusedWindowBoundsFunc = bounds
	mockSystem.ScreenBoundsFunc = func(context.Context) (image.Rectangle, error) {
		return image.Rect(0, 0, 1920, 1080), nil
	}

	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)

	return services.NewHintService(
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		mockSystem,
		generator,
		config.HintsConfig{},
		log,
		&mockVisionPort{},
	)
}

// TestHintService_GenerateHintsVisionSaysWhyItFellBackToTheScreen pins the
// difference between the two ways vision ends up scanning a whole monitor.
//
// A platform that cannot report focused-window geometry — a wlroots compositor
// with no IPC, a KWin bridge that never installed — is not the same event as a
// desktop with nothing focused, and it used to be logged as if it were. Scoping
// OCR to the entire screen instead of one window is slower and noisier, and the
// reason has to be readable without a debug build.
func TestHintService_GenerateHintsVisionSaysWhyItFellBackToTheScreen(t *testing.T) {
	tests := []struct {
		name      string
		bounds    func(context.Context) (image.Rectangle, bool, error)
		wantLevel zapcore.Level
		wantText  string
	}{
		{
			name: "a platform that cannot answer is a warning",
			bounds: func(context.Context) (image.Rectangle, bool, error) {
				return image.Rectangle{}, false, derrors.New(
					derrors.CodeNotSupported,
					"no focused-window geometry source on linux backend wayland-wlroots",
				)
			},
			wantLevel: zapcore.WarnLevel,
			wantText:  "wayland-wlroots",
		},
		{
			name: "nothing focused is routine",
			bounds: func(context.Context) (image.Rectangle, bool, error) {
				return image.Rectangle{}, false, nil
			},
			wantLevel: zapcore.DebugLevel,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)

			service := newVisionHintService(zap.New(core), testCase.bounds)

			_, err := service.GenerateHints(
				context.Background(), nil, nil, "com.example.app", domain.StrategyVision, "", false,
			)
			if err != nil {
				t.Fatalf("GenerateHints() unexpected error: %v", err)
			}

			entries := logs.FilterLevelExact(testCase.wantLevel).
				FilterMessageSnippet("focused window").
				All()
			if len(entries) == 0 {
				t.Fatalf("no %s entry about the focused window; logged %v",
					testCase.wantLevel, logs.All())
			}

			if testCase.wantText == "" {
				return
			}

			logged := fmt.Sprint(entries[0].Message, entries[0].ContextMap())
			if !strings.Contains(logged, testCase.wantText) {
				t.Errorf("entry %q does not carry %q, so the reason is unreadable",
					logged, testCase.wantText)
			}
		})
	}
}

// foreignRolesForCurrentPlatform returns native role entries that belong to
// platforms other than the one running the tests, so they resolve to nothing
// here. Returns nil on a platform with no accessibility backend.
func foreignRolesForCurrentPlatform() []string {
	return map[string][]string{
		"darwin":  {"atspi:push button", "uia:Button"},
		"linux":   {"ax:AXButton", "uia:Button"},
		"windows": {"ax:AXButton", "atspi:push button"},
	}[runtime.GOOS]
}

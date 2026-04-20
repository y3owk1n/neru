package recursivegrid

import "testing"

func TestTrainingSessionSecondDepthWrongAnswerAppliesPenalty(t *testing.T) {
	session := NewTrainingSession("u", "i", true, 2, 1)

	if result := session.HandleKey("u"); result != TrainingResultAdvanceDepth {
		t.Fatalf("first-step result = %v, want %v", result, TrainingResultAdvanceDepth)
	}

	if !session.InSecondDepth() {
		t.Fatal("session should be waiting for the second-depth key")
	}

	if got := session.OverlayKeys(); got != "i" {
		t.Fatalf("second-depth overlay keys = %q, want %q", got, "i")
	}

	if result := session.HandleKey("x"); result != TrainingResultWrong {
		t.Fatalf("wrong second-depth result = %v, want %v", result, TrainingResultWrong)
	}

	if !session.InSecondDepth() {
		t.Fatal("session should stay in second-depth mode after a wrong answer")
	}

	learned, total := session.Progress()
	if learned != 0 || total != 1 {
		t.Fatalf("progress after wrong second-depth key = (%d, %d), want (0, 1)", learned, total)
	}
}

func TestTrainingSessionSecondDepthCompleteAndReset(t *testing.T) {
	session := NewTrainingSession("u", "i", true, 2, 1)

	if result := session.HandleKey("u"); result != TrainingResultAdvanceDepth {
		t.Fatalf("first-step result = %v, want %v", result, TrainingResultAdvanceDepth)
	}

	if result := session.HandleKey("i"); result != TrainingResultCorrect {
		t.Fatalf("first drill completion result = %v, want %v", result, TrainingResultCorrect)
	}

	if session.InSecondDepth() {
		t.Fatal("session should return to top-level view after a completed drill")
	}

	if got := session.OverlayKeys(); got != "u" {
		t.Fatalf("top-level overlay after one learned hit = %q, want %q", got, "u")
	}

	if result := session.HandleKey("u"); result != TrainingResultAdvanceDepth {
		t.Fatalf("second first-step result = %v, want %v", result, TrainingResultAdvanceDepth)
	}

	if result := session.HandleKey("i"); result != TrainingResultCompleted {
		t.Fatalf("second drill completion result = %v, want %v", result, TrainingResultCompleted)
	}

	if session.Active() {
		t.Fatal("session should be inactive after completion")
	}

	if got := session.TargetIndex(); got != -1 {
		t.Fatalf("target index after completion = %d, want -1", got)
	}

	if got := session.OverlayKeys(); got != " " {
		t.Fatalf("top-level overlay after completion = %q, want %q", got, " ")
	}

	learned, total := session.Progress()
	if learned != 1 || total != 1 {
		t.Fatalf("progress after completion = (%d, %d), want (1, 1)", learned, total)
	}

	session.Reset()

	if !session.Active() {
		t.Fatal("session should be active after reset")
	}

	if session.InSecondDepth() {
		t.Fatal("session should restart at the top-level view")
	}

	if got := session.TargetIndex(); got != 0 {
		t.Fatalf("target index after reset = %d, want 0", got)
	}

	if got := session.OverlayKeys(); got != "u" {
		t.Fatalf("top-level overlay after reset = %q, want %q", got, "u")
	}
}

func TestTrainingSessionSingleDepthFallbackStillWorks(t *testing.T) {
	session := NewTrainingSession("u", "", false, 2, 1)

	if result := session.HandleKey("u"); result != TrainingResultCorrect {
		t.Fatalf("single-depth first result = %v, want %v", result, TrainingResultCorrect)
	}

	if session.InSecondDepth() {
		t.Fatal("single-depth fallback should never enter second-depth mode")
	}

	if result := session.HandleKey("u"); result != TrainingResultCompleted {
		t.Fatalf("single-depth completion result = %v, want %v", result, TrainingResultCompleted)
	}
}

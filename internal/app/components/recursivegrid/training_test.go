package recursivegrid

import "testing"

func TestTrainingSessionWrongAnswerAppliesPenaltyAndHidesLearnedKeys(t *testing.T) {
	session := NewTrainingSession("u", 2, 1)

	if result := session.HandleKey("u"); result != TrainingResultCorrect {
		t.Fatalf("first correct key result = %v, want %v", result, TrainingResultCorrect)
	}

	learned, total := session.Progress()
	if learned != 0 || total != 1 {
		t.Fatalf("progress after one hit = (%d, %d), want (0, 1)", learned, total)
	}

	if got := session.OverlayKeys(); got != "u" {
		t.Fatalf("overlay keys after one hit = %q, want %q", got, "u")
	}

	if result := session.HandleKey("x"); result != TrainingResultWrong {
		t.Fatalf("wrong key result = %v, want %v", result, TrainingResultWrong)
	}

	learned, total = session.Progress()
	if learned != 0 || total != 1 {
		t.Fatalf("progress after penalty = (%d, %d), want (0, 1)", learned, total)
	}

	if got := session.OverlayKeys(); got != "u" {
		t.Fatalf("overlay keys after penalty = %q, want %q", got, "u")
	}
}

func TestTrainingSessionCompleteAndReset(t *testing.T) {
	session := NewTrainingSession("u", 2, 1)

	if result := session.HandleKey("u"); result != TrainingResultCorrect {
		t.Fatalf("first correct key result = %v, want %v", result, TrainingResultCorrect)
	}

	if result := session.HandleKey("u"); result != TrainingResultCompleted {
		t.Fatalf("second correct key result = %v, want %v", result, TrainingResultCompleted)
	}

	if session.Active() {
		t.Fatal("session should be inactive after completion")
	}

	if got := session.TargetIndex(); got != -1 {
		t.Fatalf("target index after completion = %d, want -1", got)
	}

	if got := session.OverlayKeys(); got != " " {
		t.Fatalf("overlay keys after completion = %q, want %q", got, " ")
	}

	learned, total := session.Progress()
	if learned != 1 || total != 1 {
		t.Fatalf("progress after completion = (%d, %d), want (1, 1)", learned, total)
	}

	session.Reset()

	if !session.Active() {
		t.Fatal("session should be active after reset")
	}

	if got := session.TargetIndex(); got != 0 {
		t.Fatalf("target index after reset = %d, want 0", got)
	}

	if got := session.OverlayKeys(); got != "u" {
		t.Fatalf("overlay keys after reset = %q, want %q", got, "u")
	}

	learned, total = session.Progress()
	if learned != 0 || total != 1 {
		t.Fatalf("progress after reset = (%d, %d), want (0, 1)", learned, total)
	}
}

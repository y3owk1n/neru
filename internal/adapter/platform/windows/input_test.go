//go:build windows && (amd64 || arm64)

package windows

import (
	"testing"
	"unsafe"
)

func TestSendInputStructLayout(t *testing.T) {
	t.Parallel()

	if got := unsafe.Sizeof(input{}); got != 40 {
		t.Fatalf("sizeof(input) = %d, want 40", got)
	}

	if got := unsafe.Sizeof(mouseInput{}); got != 32 {
		t.Fatalf("sizeof(mouseInput) = %d, want 32", got)
	}

	if got := unsafe.Offsetof(input{}.mi); got != 8 {
		t.Fatalf("offsetof(input.mi) = %d, want 8", got)
	}

	// The keyboard arm of the union carries its own padding to reach the same
	// 40 bytes cbSize demands, so it needs the same offset check: a wVk landing
	// anywhere but byte 8 is a SendInput that silently posts the wrong key.
	if got := unsafe.Sizeof(keyInput{}); got != 40 {
		t.Fatalf("sizeof(keyInput) = %d, want 40", got)
	}

	if got := unsafe.Sizeof(keyboardInput{}); got != 24 {
		t.Fatalf("sizeof(keyboardInput) = %d, want 24", got)
	}

	if got := unsafe.Offsetof(keyInput{}.ki); got != 8 {
		t.Fatalf("offsetof(keyInput.ki) = %d, want 8", got)
	}
}

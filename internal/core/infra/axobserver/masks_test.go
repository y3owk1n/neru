package axobserver

import (
	"math/bits"
	"testing"
)

// TestDefaultMaskExcludesValueChanged pins the one deliberate omission from the
// watched set. value_changed fires on every value update, so watching it would
// wake the observer continuously.
func TestDefaultMaskExcludesValueChanged(t *testing.T) {
	if DefaultMask&notifValueChanged != 0 {
		t.Error("DefaultMask must not include value_changed")
	}
}

// TestDefaultMaskIsEveryBitExceptValueChanged guards that adding a notification
// bit to the vocabulary also adds it to the watched set (or consciously updates
// this test). Without it, a new bit could be defined and mapped to a native
// notification yet never actually observed.
func TestDefaultMaskIsEveryBitExceptValueChanged(t *testing.T) {
	allBits := (highestBit << 1) - 1 // every defined bit set

	want := allBits &^ notifValueChanged
	if DefaultMask != want {
		t.Fatalf("DefaultMask = %#x, want %#x (every bit except value_changed)", DefaultMask, want)
	}

	if got := bits.OnesCount32(uint32(DefaultMask)); got != bits.OnesCount32(uint32(allBits))-1 {
		t.Fatalf("DefaultMask sets %d bits, want %d", got, bits.OnesCount32(uint32(allBits))-1)
	}
}

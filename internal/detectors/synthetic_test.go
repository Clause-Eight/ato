package detectors

import (
	"fmt"
	"testing"
)

func TestCanonical(t *testing.T) {
	if got := Canonical("ro49 aaaa-1b.c@d"); got != "RO49AAAA1BCD" {
		t.Fatalf("got %q", got)
	}
}

func TestSyntheticMarkIsRare(t *testing.T) {
	marked := 0
	const n = 1 << 20
	for i := 0; i < n; i++ {
		if Synthetic(fmt.Sprintf("%010d", i)) {
			marked++
		}
	}
	// about 16 of 2^20 values carry a 16-bit mark by chance.
	if marked < 2 || marked > 50 {
		t.Fatalf("%d of %d values carry the mark", marked, n)
	}
	if Synthetic("") {
		t.Fatal("an empty value is never synthetic")
	}
}

package detectors

import (
	"strings"
	"testing"
)

func TestEHICFormat(t *testing.T) {
	d, _ := Get("ehic")
	if d.OnByDefault || !d.NeedsKeyword {
		t.Fatal("the EHIC detector must be off by default and keyword-gated")
	}
	for i, v := range []string{"80642123450000012345", "8027612345 0000012345", "80380 12345 0000012345"} {
		if got := d.Pattern.FindString("ehic: " + v); got != v {
			t.Errorf("valid case %d not found whole", i)
		}
		if !validEHIC(d.Normalize(v)) {
			t.Errorf("valid case %d rejected", i)
		}
	}
	for i, v := range []string{
		"81642123450000012345",  // not the health-card range
		"80999123450000012345",  // no such country
		"80840123450000012345",  // a country outside the EHIC area
		"8064212345000001234",   // 19 digits
		"806421234500000123456", // 21 digits
		"8064212345000001234A",
	} {
		if validEHIC(v) {
			t.Errorf("invalid case %d accepted", i)
		}
	}
	if d.Normalize("80642 12345 0000012345") != "80642123450000012345" {
		t.Error("normalisation must drop the spaces")
	}
	if m := d.Mask("80642123450000012345"); m != strings.Repeat("•", 20) {
		t.Errorf("mask %q", m)
	}
}

package detectors

import "testing"

func TestLUNationalID(t *testing.T) {
	d, ok := Get("lu-national-id")
	if !ok {
		t.Fatal("not registered")
	}
	cases := []struct {
		why   string
		value string
		want  bool
	}{
		{"valid", "1983071514250", true},
		{"leap day", "2004022907675", true},
		{"printed in groups", "1961 1130 981 71", true},
		{"wrong Luhn digit", "1983071514260", false},
		{"wrong Verhoeff digit", "1983071514251", false},
		{"Verhoeff over twelve digits instead of eleven", "1983071514258", false},
		{"29 February in a year that is not a leap year", "2003022911399", false},
		{"twelve digits", "198307151425", false},
	}
	for _, c := range cases {
		if got := d.Validate(d.Normalize(c.value)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.why, got, c.want)
		}
	}
	if m := d.Mask("1983071514250"); len([]rune(m)) != 13 || m == "1983071514250" {
		t.Errorf("mask does not hide the value")
	}
}

func TestVerhoeff(t *testing.T) {
	for x := 0; x < 10; x++ {
		if d5(x, 0) != x || d5(0, x) != x {
			t.Fatalf("0 is not the identity")
		}
	}
	// The two non-commuting products named in the description of the group.
	if d5(1, 5) != 6 || d5(5, 1) != 9 {
		t.Errorf("rotation and reflection products are wrong")
	}
	// Every single-digit error and every adjacent transposition changes the check digit.
	body := "73019245168"
	want := verhoeffDigit(body)
	for i := 0; i < len(body); i++ {
		for c := byte('0'); c <= '9'; c++ {
			if c == body[i] {
				continue
			}
			changed := body[:i] + string(c) + body[i+1:]
			if verhoeffDigit(changed) == want {
				t.Fatalf("a changed digit at %d keeps the check digit", i)
			}
		}
		if i+1 < len(body) && body[i] != body[i+1] {
			swapped := body[:i] + string(body[i+1]) + string(body[i]) + body[i+2:]
			if verhoeffDigit(swapped) == want {
				t.Fatalf("swapping digits %d and %d keeps the check digit", i, i+1)
			}
		}
	}
}

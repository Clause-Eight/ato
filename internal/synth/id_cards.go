package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("fr-id-card", func(r *rand.Rand) string {
		if r.Intn(2) == 0 {
			// 1995 card: year and month of issue, department, order number.
			dept := []string{"075", "092", "013", "069", "2A0", "2B1", "971", "033"}[r.Intn(8)]
			return Digits(r, 2) + Pad(1+r.Intn(12), 2) + dept + Digits(r, 5)
		}
		// 2021 card: nine letters and digits, at least one of each.
		return Letters(r, 1, "") + Letters(r, 7, alnum) + Digits(r, 1)
	})
	Register("nl-id-card", func(r *rand.Rand) string {
		return Letters(r, 2, nlLetters) + nlMiddle(r) + Digits(r, 1)
	})
	Register("be-id-card", func(r *rand.Rand) string {
		body := Digits(r, 10)
		check := Mod97(body)
		if check == 0 {
			check = 97
		}
		if r.Intn(2) == 0 {
			return fmt.Sprintf("%s-%s-%02d", body[:3], body[3:], check)
		}
		return fmt.Sprintf("%s%02d", body, check)
	})
	Register("hu-id-card", func(r *rand.Rand) string {
		if r.Intn(2) == 0 {
			return Digits(r, 6) + Letters(r, 2, "")
		}
		return Letters(r, 2, "") + Digits(r, 6)
	})
	Register("gr-id-card", func(r *rand.Rand) string {
		greek := []string{"Α", "Β", "Ε", "Ζ", "Η", "Ι", "Κ", "Μ", "Ν", "Ο", "Ρ", "Τ", "Υ", "Χ"}
		sep := []string{"-", "", " "}[r.Intn(3)]
		switch r.Intn(3) {
		case 0:
			return Letters(r, 2, "ABEZHIKMNOPTYX") + sep + Digits(r, 6)
		case 1:
			return greek[r.Intn(len(greek))] + greek[r.Intn(len(greek))] + sep + Digits(r, 6)
		}
		// Older cards: one letter of the Greek alphabet.
		old := []string{"Γ", "Δ", "Θ", "Λ", "Ξ", "Π", "Σ", "Φ", "Ψ", "Ω", "Α", "Κ"}
		return old[r.Intn(len(old))] + sep + Digits(r, 6)
	})
	Register("bg-id-card", func(r *rand.Rand) string {
		return Digits(r, 9)
	})
}

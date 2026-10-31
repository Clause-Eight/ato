package synth

import "math/rand"

func init() {
	// ICD-10 category inside a chapter, sometimes with a subcategory digit: E11, J45.9.
	Register("icd10", func(r *rand.Rand) string {
		chapters := []struct {
			letter   string
			from, to int
		}{{"A", 0, 99}, {"B", 0, 99}, {"C", 0, 97}, {"D", 0, 48}, {"D", 50, 89}, {"E", 0, 90}, {"F", 0, 99}, {"G", 0, 99},
			{"H", 0, 95}, {"I", 0, 99}, {"J", 0, 99}, {"K", 0, 93}, {"L", 0, 99}, {"M", 0, 99}, {"N", 0, 99}, {"O", 0, 99},
			{"P", 0, 96}, {"Q", 0, 99}, {"R", 0, 99}, {"S", 0, 99}, {"T", 0, 98}, {"V", 1, 99}, {"W", 0, 99}, {"X", 0, 99},
			{"Y", 0, 98}, {"Z", 0, 99}}
		c := chapters[r.Intn(len(chapters))]
		code := c.letter + Pad(c.from+r.Intn(c.to-c.from+1), 2)
		if r.Intn(2) == 0 {
			code += "." + Digits(r, 1)
		}
		return code
	})
	// ICD-11 stem code: chapter, letter, digit, letter or digit, sometimes with an extension after a point.
	Register("icd11", func(r *rand.Rand) string {
		const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		const alnum = "0123456789" + letters
		code := Letters(r, 1, "123456789ABCDEFGHJKLMNPQRS") + Letters(r, 1, letters) + Digits(r, 1) + Letters(r, 1, alnum)
		if r.Intn(2) == 0 {
			code += "." + Letters(r, 1+r.Intn(2), alnum)
		}
		return code
	})
	// ATC substance: main group, two digits from 01, two letters, two digits.
	Register("atc-code", func(r *rand.Rand) string {
		return Letters(r, 1, "ABCDGHJLMNPRSV") + Pad(1+r.Intn(99), 2) + Letters(r, 2, "") + Digits(r, 2)
	})
	// AIC medicine pack code: 0, seven digits, then the check digit: the digits in even places (from 1) are
	// doubled with nine taken off above nine, and the check is the last digit of the total.
	Register("it-aic", func(r *rand.Rand) string {
		body := "0" + Digits(r, 7)
		total := 0
		for i, c := range body {
			n := int(c - '0')
			if i%2 == 1 {
				if n *= 2; n > 9 {
					n -= 9
				}
			}
			total += n
		}
		return body + Pad(total%10, 1)
	})
}

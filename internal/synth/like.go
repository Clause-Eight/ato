package synth

import (
	"fmt"
	"math/rand"
	"strings"
)

// Shaper returns a generator of values like sample, a value as it was written, or nil when it does not
// understand sample. Generated values keep what tests commonly check (country, brand, length, the sex and
// century of a personal number) and replace the rest. Letters and digits stay in the same order as in sample,
// so the separators and case of sample can be laid over them. A value may still fail its detector (a phone
// number outside the plan, a lucky VAT format): callers check.
type Shaper func(sample string) Generator

var likes = map[string]Shaper{}

// RegisterLike adds the shaper for a detector.
func RegisterLike(detector string, s Shaper) {
	if _, dup := likes[detector]; dup {
		panic("synth: duplicate shaper for " + detector)
	}
	likes[detector] = s
}

// Like returns a generator of values like sample, for detectors whose values have a country, brand or class
// that one generator would not keep. ok is false when there is none or sample is not understood.
func Like(detector, sample string) (Generator, bool) {
	s, ok := likes[detector]
	if !ok {
		return nil, false
	}
	g := s(sample)
	return g, g != nil
}

func init() {
	RegisterLike("iban", likeIBAN)
	RegisterLike("card", likeCard)
	RegisterLike("eu-phone", likePhone)
	RegisterLike("email", likeEmail)
	RegisterLike("eu-vat", likeVAT)
	RegisterLike("ro-cnp", likeCNP)
	RegisterLike("bg-egn", likeEGN)
	RegisterLike("ip-address", likeIP)
	RegisterLike("mac-address", likeMAC)
	RegisterLike("gps-coordinates", likeGPS)
	RegisterLike("crypto-wallet", likeWallet)
	RegisterLike("advertising-id", likeAdvertisingID)
	RegisterLike("de-id-card", likeDEIDCard)
	RegisterLike("licence-plate", likePlate)
	RegisterLike("driving-licence", likePlate)
	RegisterLike("postcode", likePlate)
}

// alnums keeps the letters and digits of s, in upper case.
func alnums(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9', c >= 'A' && c <= 'Z':
			b.WriteRune(c)
		case c >= 'a' && c <= 'z':
			b.WriteRune(c - 'a' + 'A')
		}
	}
	return b.String()
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// vary replaces each digit of s with a random digit and each letter with a random letter from letters (A-Z
// when empty), in the letter's case, and keeps every other character.
func vary(r *rand.Rand, s, letters string) string {
	if letters == "" {
		letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	}
	b := []byte(s)
	for i, c := range b {
		switch {
		case c >= '0' && c <= '9':
			b[i] = byte('0' + r.Intn(10))
		case c >= 'A' && c <= 'Z':
			b[i] = letters[r.Intn(len(letters))]
		case c >= 'a' && c <= 'z':
			b[i] = letters[r.Intn(len(letters))] - 'A' + 'a'
		}
	}
	return string(b)
}

// likeIBAN keeps the country, so the length and account layout, with letters where sample has letters.
func likeIBAN(sample string) Generator {
	c := alnums(sample)
	if len(c) < 8 || c[0] < 'A' || c[1] < 'A' || !allDigits(c[2:4]) {
		return nil
	}
	country, layout := c[:2], c[4:]
	return func(r *rand.Rand) string {
		bban := vary(r, layout, "")
		check := 98 - Mod97(LettersAsNumbers(bban+country)+"00")
		return fmt.Sprintf("%s%02d%s", country, check, bban)
	}
}

// likeCard keeps the digits that make the brand (1 for Visa, up to 4 for Mastercard 2-series, Discover and
// JCB) and the length.
func likeCard(sample string) Generator {
	d := alnums(sample)
	if len(d) < 12 || !allDigits(d) {
		return nil
	}
	keep := 2
	switch {
	case d[0] == '4':
		keep = 1
	case d[0] == '2', strings.HasPrefix(d, "6011"), strings.HasPrefix(d, "35"):
		keep = 4
	case strings.HasPrefix(d, "64"):
		keep = 3
	}
	return func(r *rand.Rand) string {
		body := d[:keep] + Digits(r, len(d)-1-keep)
		return body + fmt.Sprint(LuhnDigit(body))
	}
}

// likePhone keeps the international prefix, the country code, a "(0)" trunk prefix and the first two digits of
// the national number (mobile or geographic, the area; one when the number is short, so enough values remain
// for one to carry the mark), or the first three of a national 07x/08x mobile, and the length.
func likePhone(sample string) Generator {
	d := alnums(sample)
	if len(d) < 6 || !allDigits(d) {
		return nil
	}
	var keep int
	switch {
	case strings.HasPrefix(sample, "+") || strings.HasPrefix(d, "00"):
		keep = 0
		if strings.HasPrefix(d, "00") {
			keep = 2
		}
		keep += 2
		switch d[keep-2 : keep] {
		case "35", "37", "38", "42":
			keep++
		}
		if strings.Contains(sample, "(0)") {
			keep++
		}
		keep++
		if len(d)-keep > 7 {
			keep++
		}
	default:
		keep = 3
	}
	if keep >= len(d) {
		return nil
	}
	return func(r *rand.Rand) string { return d[:keep] + Digits(r, len(d)-keep) }
}

// likeEmail keeps the shape of the local part, the number and length of the domain labels and the top-level
// domain (with a short second level such as co.uk or com.ro). The domain is made up rather than a reserved
// example domain: the email detector ignores reserved domains, so such a replacement would no longer be
// reported as a sample, and a test domain of another TLD would change what tests on the TLD see.
func likeEmail(sample string) Generator {
	at := strings.LastIndexByte(sample, '@')
	if at <= 0 {
		return nil
	}
	local, domain := sample[:at], sample[at+1:]
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return nil
	}
	kept := 1
	if len(labels) >= 3 && len(labels[len(labels)-2]) <= 3 {
		kept = 2
	}
	return func(r *rand.Rand) string {
		out := make([]string, len(labels))
		for i, l := range labels {
			if i >= len(labels)-kept {
				out[i] = l
			} else {
				out[i] = vary(r, l, "")
			}
		}
		return vary(r, local, "") + "@" + strings.Join(out, ".")
	}
}

// likeVAT keeps the country and length. Countries whose check rule is known here get a fresh number with its
// check digit; the others keep their letters, their first digit and, for Sweden, the 01 suffix.
func likeVAT(sample string) Generator {
	c := alnums(sample)
	if len(c) < 4 || c[0] < 'A' || c[1] < 'A' {
		return nil
	}
	country, body := c[:2], c[2:]
	n := len(body)
	switch {
	case country == "DE" && n == 9:
		return func(r *rand.Rand) string {
			b := fmt.Sprint(1+r.Intn(9)) + Digits(r, 7)
			return "DE" + b + fmt.Sprint(Mod1110Digit(b))
		}
	case country == "PL" && n == 10:
		return func(r *rand.Rand) string {
			for {
				b := Digits(r, 9)
				if d := WeightedSum(b, []int{6, 5, 7, 2, 3, 4, 5, 6, 7}) % 11; d != 10 {
					return "PL" + b + fmt.Sprint(d)
				}
			}
		}
	case country == "HU" && n == 8:
		return func(r *rand.Rand) string {
			b := fmt.Sprint(1+r.Intn(9)) + Digits(r, 6)
			return "HU" + b + fmt.Sprint((10-WeightedSum(b, []int{9, 7, 3, 1, 9, 7, 3})%10)%10)
		}
	case country == "RO" && n >= 2 && n <= 10:
		return func(r *rand.Rand) string {
			m := n
			if m < 8 && r.Intn(2) == 0 {
				// A short code has few values, maybe none with the mark: half are eight digits long.
				m = 8
			}
			b := fmt.Sprint(1+r.Intn(9)) + Digits(r, m-2)
			padded := strings.Repeat("0", 9-len(b)) + b
			d := WeightedSum(padded, []int{7, 5, 3, 2, 1, 7, 5, 3, 2}) * 10 % 11 % 10
			return "RO" + b + fmt.Sprint(d)
		}
	case country == "BG" && n == 9:
		return func(r *rand.Rand) string {
			b := Digits(r, 8)
			d := WeightedSum(b, []int{1, 2, 3, 4, 5, 6, 7, 8}) % 11
			if d == 10 {
				d = WeightedSum(b, []int{3, 4, 5, 6, 7, 8, 9, 10}) % 11 % 10
			}
			return "BG" + b + fmt.Sprint(d)
		}
	case country == "BG":
		return nil
	case country == "FR" && n == 11 && allDigits(body):
		// The key is (SIREN followed by 12) mod 97, and the SIREN passes Luhn.
		return func(r *rand.Rand) string {
			siren := Digits(r, 8)
			siren += fmt.Sprint(LuhnDigit(siren))
			var v int
			fmt.Sscan(siren+"12", &v)
			return "FR" + Pad(v%97, 2) + siren
		}
	case country == "IE" && (n == 8 || n == 9) && allDigits(body[:7]):
		// The check letter is WABCDEFGHIJKLMNOPQRSTUV at the sum of the digits weighted 8 down to 2, plus 9 times
		// the second letter's index, mod 23.
		const alphabet = "WABCDEFGHIJKLMNOPQRSTUV"
		return func(r *rand.Rand) string {
			b := Digits(r, 7)
			sum := WeightedSum(b, []int{8, 7, 6, 5, 4, 3, 2})
			extra := body[8:]
			if extra != "" {
				sum += 9 * strings.IndexByte(alphabet, extra[0])
			}
			return "IE" + b + string(alphabet[sum%23]) + extra
		}
	}
	head, tail := 1, 0
	switch country {
	case "EE":
		head = 2
	case "SE":
		tail = 2
	}
	if head+tail >= n {
		return nil
	}
	return func(r *rand.Rand) string {
		b := []byte(body)
		for i := head; i < n-tail; i++ {
			if b[i] >= '0' && b[i] <= '9' {
				b[i] = byte('0' + r.Intn(10))
			}
		}
		return country + string(b)
	}
}

// likeCNP keeps the first digit (sex and century) and makes a new date in that century, county and serial.
func likeCNP(sample string) Generator {
	d := alnums(sample)
	if len(d) != 13 || !allDigits(d) || d[0] == '0' {
		return nil
	}
	sex := d[0]
	return func(r *rand.Rand) string {
		_, month, day := Date(r)
		year := r.Intn(100)
		if sex == '5' || sex == '6' {
			year = r.Intn(20)
		}
		counties := []int{1, 7, 12, 22, 40, 46, 51, 52}
		body := fmt.Sprintf("%c%s%s%s%s%s", sex, Pad(year, 2), Pad(month, 2), Pad(day, 2), Pad(counties[r.Intn(len(counties))], 2), Digits(r, 3))
		c := WeightedSum(body, []int{2, 7, 9, 1, 4, 6, 3, 5, 8, 2, 7, 9}) % 11
		if c == 10 {
			c = 1
		}
		return body + fmt.Sprint(c)
	}
}

// likeEGN keeps the century (written in the month) and the sex (the parity of the ninth digit).
func likeEGN(sample string) Generator {
	d := alnums(sample)
	if len(d) != 10 || !allDigits(d) {
		return nil
	}
	offset := 0
	switch m := int(d[2]-'0')*10 + int(d[3]-'0'); {
	case m > 40:
		offset = 40
	case m > 20:
		offset = 20
	}
	parity := int(d[8]-'0') % 2
	return func(r *rand.Rand) string {
		_, month, day := Date(r)
		year := r.Intn(100)
		if offset == 40 {
			year = r.Intn(20)
		}
		body := Pad(year, 2) + Pad(month+offset, 2) + Pad(day, 2) + Digits(r, 2) + fmt.Sprint(r.Intn(5)*2+parity)
		return body + fmt.Sprint(WeightedSum(body, []int{2, 4, 8, 5, 10, 9, 7, 3, 6})%11%10)
	}
}

// likeIP keeps the version, the number of digits in each part and, for IPv6, the first group (the registry's
// block).
func likeIP(sample string) Generator {
	if strings.Contains(sample, ":") {
		first := strings.IndexByte(sample, ':')
		return func(r *rand.Rand) string { return sample[:first] + vary(r, sample[first:], "ABCDEF") }
	}
	parts := strings.Split(sample, ".")
	if len(parts) != 4 || !allDigits(strings.Join(parts, "")) {
		return nil
	}
	return func(r *rand.Rand) string {
		// Short octets leave few addresses, maybe none with the mark: half the values have octets of any length.
		free := r.Intn(2) == 0
		var out []string
		for _, p := range parts {
			n := len(p)
			if free {
				n = 1 + r.Intn(3)
			}
			low, high := 0, 9
			switch n {
			case 2:
				low, high = 10, 99
			case 3:
				low, high = 100, 255
			}
			out = append(out, fmt.Sprint(low+r.Intn(high-low+1)))
		}
		return strings.Join(out, ".")
	}
}

// likeAdvertisingID keeps the UUID's version and variant characters.
func likeAdvertisingID(sample string) Generator {
	if len(sample) != 36 {
		return nil
	}
	return func(r *rand.Rand) string {
		return vary(r, sample[:14], "ABCDEF") + sample[14:15] + vary(r, sample[15:19], "ABCDEF") + sample[19:20] + vary(r, sample[20:], "ABCDEF")
	}
}

// likeDEIDCard keeps the letter and digit positions, with the letters the card number uses, and its check digit.
func likeDEIDCard(sample string) Generator {
	c := alnums(sample)
	if len(c) != 10 || c[0] < 'A' {
		return nil
	}
	return func(r *rand.Rand) string {
		body := Letters(r, 1, "LMNPRTVWXY") + vary(r, c[1:9], "CFGHJKLMNPRTVWXYZ")
		return body + fmt.Sprint(mrzDigit(body))
	}
}

// likePlate keeps the letters a plate, licence number or postcode starts with (a county, district, authority
// or country) and the letter and digit positions of the rest.
func likePlate(sample string) Generator {
	head := 0
	for head < len(sample) && (sample[head] >= 'A' && sample[head] <= 'Z' || sample[head] >= 'a' && sample[head] <= 'z') {
		head++
	}
	if head == len(sample) {
		return nil
	}
	return func(r *rand.Rand) string { return sample[:head] + vary(r, sample[head:], "") }
}

// likeMAC keeps the first octet (unicast or multicast, universal or local).
func likeMAC(sample string) Generator {
	if len(sample) < 3 {
		return nil
	}
	return func(r *rand.Rand) string { return sample[:2] + vary(r, sample[2:], "ABCDEF") }
}

// likeGPS keeps the labels, signs, the first digit of each number and the number of digits, so a value stays
// within tens of degrees of the original and in range.
func likeGPS(sample string) Generator {
	return func(r *rand.Rand) string {
		b := []byte(sample)
		for i, c := range b {
			first := i == 0 || (b[i-1] < '0' || b[i-1] > '9') && b[i-1] != '.'
			if c >= '0' && c <= '9' && !first {
				b[i] = byte('0' + r.Intn(10))
			}
		}
		return string(b)
	}
}

// likeWallet keeps the kind of address: legacy or script Bitcoin, Bech32 or Bech32m, or Ethereum.
func likeWallet(sample string) Generator {
	lower := strings.ToLower(sample)
	switch {
	case strings.HasPrefix(lower, "0x"):
		return eip55
	case strings.HasPrefix(lower, "bc1") && len(lower) > 4:
		version, size := byte(0), 20
		if len(lower) > 50 {
			size = 32
			if lower[3] != 'q' {
				version = 1
			}
		}
		return func(r *rand.Rand) string { return segwit(version, randomBytes(r, size)) }
	case strings.HasPrefix(sample, "1"):
		return func(r *rand.Rand) string { return base58Check(0x00, randomBytes(r, 20)) }
	case strings.HasPrefix(sample, "3"):
		return func(r *rand.Rand) string { return base58Check(0x05, randomBytes(r, 20)) }
	}
	return nil
}

package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// scanDefault scans a file with the detectors that are on by default.
func scanDefault(t *testing.T, path, text string) []domain.Finding {
	t.Helper()
	var dets []detectors.Detector
	for _, d := range detectors.All() {
		if d.OnByDefault {
			dets = append(dets, d)
		}
	}
	fs, err := ScanReader(path, strings.NewReader(text), dets)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func byDetector(fs []domain.Finding, name string) []domain.Finding {
	var out []domain.Finding
	for _, f := range fs {
		if f.Detector == name {
			out = append(out, f)
		}
	}
	return out
}

func TestCardSecurityCodeAndExpiryNextToACard(t *testing.T) {
	card := synthValue(t, "card", 3)
	cases := []struct {
		name         string
		path, text   string
		code, expiry int
	}{
		{"JSON object over several lines", "pay.json", "{\n  \"number\": \"" + card + "\",\n  \"exp_month\": 12,\n  \"expiry\": \"12/27\",\n  \"cvc\": \"381\"\n}\n", 1, 1},
		{"CSV row under labelled columns", "cards.csv", "card_number,exp,cvv\n" + card + ",0927,742\n", 1, 1},
		{"log line", "app.log", "charge card=" + card + " valid_thru=12/2027 cvv2=4410\n", 1, 1},
		{"PHP array", "seed.php", "['pan' => '" + card + "', 'cardExpiry' => '03/29', 'securityCode' => '019'],\n", 1, 1},
		{"code without a card", "app.log", "lone cvv: 512 exp: 12/27\n", 0, 0},
		{"card with an unlabelled three-digit number", "app.log", "card " + card + " total 123 1227\n", 0, 0},
		{"a date is not an expiry", "app.log", "card " + card + " expires 12/27/2026\n", 0, 0},
		{"code label far from any card", "app.log", "card " + card + "\na\nb\nc\nd\ne\nf\ng\ncvc: 381\n", 0, 0},
		{"a verification code is not a card code", "app.log", "card " + card + " verification_code: 381\n", 0, 0},
	}
	for _, c := range cases {
		fs := scanDefault(t, c.path, c.text)
		codes, exps := byDetector(fs, "card-security-code"), byDetector(fs, "card-expiry")
		if len(codes) != c.code || len(exps) != c.expiry {
			t.Errorf("%s: %d codes and %d expiry dates, want %d and %d", c.name, len(codes), len(exps), c.code, c.expiry)
			continue
		}
		for _, f := range codes {
			if f.Confidence != domain.High || strings.Trim(f.Masked, "•") != "" {
				t.Errorf("%s: a security code is reported at %s, masked %v", c.name, f.Confidence, strings.Trim(f.Masked, "•") == "")
			}
		}
		if strings.Contains(c.text, card) && len(byDetector(fs, "card")) != 1 {
			t.Errorf("%s: the card itself is not reported once", c.name)
		}
	}
}

func TestSecurityCodeNextToATestCardIsSample(t *testing.T) {
	fs := byDetector(scanDefault(t, "stripe.json", `{"number": "4242424242424242", "cvc": "123"}`+"\n"), "card-security-code")
	if len(fs) != 1 || !fs[0].Sample || fs[0].Confidence != domain.Low {
		t.Fatalf("a code next to a published test card should be a low sample, got %d findings", len(fs))
	}
}

func TestHashedContactForAdMatching(t *testing.T) {
	h := func(c string) string { return strings.Repeat(c+"1b2c3d4e5f60718293a4b5c6d7e8f90", 2)[:64] }
	md5 := "9b2c3d4e5f60718293a4b5c6d7e8f90a"
	cases := []struct {
		name, path, text string
		want             int
	}{
		{"Meta user_data on one line", "capi.json", `{"user_data": {"em": ["` + h("a") + `", "` + h("b") + `"], "ph": "` + h("c") + `"}}`, 3},
		{"Meta user_data over lines", "capi.js", "const payload = {\n  user_data: {\n    em: [\n      '" + h("a") + "',\n    ],\n    external_id: '" + h("d") + "',\n  },\n};", 2},
		{"short keys side by side", "capi.py", `data = {"em": "` + h("a") + `", "ph": "` + h("b") + `", "country": "ro"}`, 2},
		{"Google enhanced conversions", "gads.py", `{"sha256_email_address": "` + h("e") + `", "sha256_phone_number": "` + h("f") + `"}`, 2},
		{"camelCase Google Ads API", "gads.ts", `userIdentifiers: [{ hashedEmail: "` + h("e") + `" }, { hashedPhoneNumber: "` + h("f") + `" }]`, 2},
		{"TikTok user object", "tiktok.json", `"user": {"email": "` + h("a") + `", "phone": "` + h("b") + `"}`, 2},
		{"MD5 under a hashed email key", "legacy.py", "hashed_email = '" + md5 + "'", 1},
		{"CSV column", "audience.csv", "email_sha256,country\n" + h("a") + ",ro\n", 1},
		{"a short key alone", "app.js", `const x = {"em": "` + h("a") + `"}`, 0},
		{"a commit", "CHANGELOG.md", "fixed in commit " + h("a")[:40] + " and " + h("b"), 0},
		{"a checksum", "go.sum", "example.org/mod v1.0.0 h1:" + h("a") + " sha256 " + h("b"), 0},
		{"a hash under an unrelated key", "lock.json", `"integrity": "` + h("a") + `", "etag": "` + md5 + `"`, 0},
		{"the hash of the empty string", "capi.json", `{"hashed_email": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}`, 0},
	}
	for _, c := range cases {
		fs := byDetector(scanDefault(t, c.path, c.text+"\n"), "hashed-contact")
		got := 0
		for _, f := range fs {
			if !f.Sample {
				got++
			}
			if !f.Sample && f.Confidence < domain.Medium {
				t.Errorf("%s: a hash is reported at %s", c.name, f.Confidence)
			}
		}
		if got != c.want {
			t.Errorf("%s: %d hashes found, want %d", c.name, got, c.want)
		}
	}
}

// jwt builds an unsigned-looking token with the given claims and a fixed made-up signature.
func jwt(claims map[string]any) string {
	seg := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	return seg(map[string]string{"alg": "HS256", "typ": "JWT"}) + "." + seg(claims) + ".c2lnbmF0dXJlLW9mLWEtbWFkZS11cC10b2tlbg"
}

func TestJWTWithPersonalClaims(t *testing.T) {
	iban := synthValue(t, "iban", 2)
	cases := []struct {
		name   string
		claims map[string]any
		want   bool
	}{
		{"email claim", map[string]any{"sub": "u-81", "email": "lina.korv7@mailbox-west.eu"}, true},
		{"name claims", map[string]any{"given_name": "Ilona", "family_name": "Varga"}, true},
		{"full name", map[string]any{"name": "Ilona Varga", "iat": 1700000000}, true},
		{"phone number", map[string]any{"phone_number": "+40 712 345 678"}, true},
		{"birthdate", map[string]any{"birthdate": "1984-03-07"}, true},
		{"address object", map[string]any{"address": map[string]any{"locality": "Cluj"}}, true},
		{"an IBAN in a custom claim", map[string]any{"https://bank/claims": map[string]any{"acct": iban}}, true},
		{"an email in another claim", map[string]any{"upn": "lina.korv7@mailbox-west.eu"}, true},
		{"service token", map[string]any{"sub": "svc-billing", "scope": "read write", "iat": 1700000000}, false},
		{"placeholder name", map[string]any{"sub": "1234567890", "name": "John Doe", "admin": true}, false},
		{"documentation email", map[string]any{"email": "someone@example.com"}, false},
		{"one word name", map[string]any{"name": "Billing"}, false},
	}
	for _, c := range cases {
		tok := jwt(c.claims)
		for _, line := range []string{"Authorization: Bearer " + tok, `{"token": "` + tok + `"}`} {
			fs := byDetector(scanDefault(t, "app.log", line+"\n"), "jwt-personal")
			if got := len(fs) == 1; got != c.want {
				t.Errorf("%s: found=%v, want %v", c.name, got, c.want)
				continue
			}
			for _, f := range fs {
				header := tok[:strings.IndexByte(tok, '.')+1]
				if !strings.HasPrefix(f.Masked, header) || strings.Trim(f.Masked[len(header):], "•") != "" || len([]rune(f.Masked)) != len(tok) {
					t.Errorf("%s: the mask must keep only the header", c.name)
				}
				if strings.Contains(fmt.Sprint(f.Reasons), "@") {
					t.Errorf("%s: a reason names a claim value", c.name)
				}
			}
		}
	}
	if fs := byDetector(scanDefault(t, "app.log", "eyJhbGciOiJIUzI1NiJ9.eyJub3QganNvbg.sig\n"), "jwt-personal"); len(fs) != 0 {
		t.Error("a token whose payload is not JSON was reported")
	}
}

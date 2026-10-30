package engine

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// group writes v in groups of the given sizes joined by sep.
func group(v, sep string, sizes ...int) string {
	var parts []string
	for _, n := range sizes {
		parts = append(parts, v[:n])
		v = v[n:]
	}
	return strings.Join(parts, sep)
}

func percentAll(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		fmt.Fprintf(&b, "%%%02X", s[i])
	}
	return b.String()
}

func TestCNPWithSeparators(t *testing.T) {
	cases := []struct {
		name  string
		write func(v string) string
		found bool
	}{
		{"plain", func(v string) string { return v }, true},
		{"sex, birth date, county and number", func(v string) string { return group(v, " ", 1, 6, 6) }, true},
		{"sex and year, month and day, rest", func(v string) string { return group(v, "-", 3, 4, 6) }, true},
		{"phone-like groups", func(v string) string { return group(v, " ", 3, 3, 3, 4) }, true},
		{"phone-like groups with dashes", func(v string) string { return group(v, "-", 3, 3, 3, 4) }, true},
		{"mixed separators", func(v string) string { return group(v, " ", 3, 4) + "-" + v[7:] }, false},
		{"groups it is not printed in", func(v string) string { return group(v, " ", 4, 4, 5) }, false},
		{"slashes", func(v string) string { return group(v, "/", 1, 6, 6) }, false},
	}
	for seed := int64(1); seed <= 20; seed++ {
		v := synthValue(t, "ro-cnp", seed)
		for _, c := range cases {
			fs := scanWith(t, "ro-cnp", "cnp: "+c.write(v))
			if got := len(fs) == 1 && fs[0].Value == v; got != c.found {
				t.Errorf("%s, seed %d: found=%v, want %v", c.name, seed, got, c.found)
			}
		}
	}
}

func TestEncodedValuesAreDecoded(t *testing.T) {
	cases := []struct {
		name    string
		encode  func(v string) string
		reason  string
		keyword bool
	}{
		{"query string after an encoded equals sign", func(v string) string { return "GET /search?q=cnp%3D" + v + "&page=2" }, "URL encoding", true},
		{"every byte escaped", func(v string) string { return "q=" + percentAll(v) }, "URL encoding", false},
		{"grouped with encoded spaces", func(v string) string { return "q=" + url.PathEscape(group(v, " ", 1, 6, 6)) }, "URL encoding", false},
		{"base64", func(v string) string { return "token=" + base64.StdEncoding.EncodeToString([]byte(v)) }, "base64", false},
		{"base64 of the value run together with letters", func(v string) string {
			return `{"ref": "` + base64.RawURLEncoding.EncodeToString([]byte("cnp"+v)) + `"}`
		}, "", false},
		{"base64 of the value in an encoded query", func(v string) string {
			return "id=" + url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(v)))
		}, "base64", false},
	}
	for seed := int64(1); seed <= 20; seed++ {
		v := synthValue(t, "ro-cnp", seed)
		for _, c := range cases {
			line := c.encode(v)
			fs := scanWith(t, "ro-cnp", line)
			if c.reason == "" {
				if len(fs) != 0 {
					t.Errorf("%s, seed %d: a token that decodes to more than the value is not a value", c.name, seed)
				}
				continue
			}
			if len(fs) != 1 || fs[0].Value != v {
				t.Errorf("%s, seed %d: want the decoded value, got %d findings", c.name, seed, len(fs))
				continue
			}
			f := fs[0]
			if !strings.Contains(strings.Join(f.Reasons, "; "), "found after decoding "+c.reason) {
				t.Errorf("%s, seed %d: the reason should name the encoding: %q", c.name, seed, f.Reasons)
			}
			if f.Keyword != c.keyword {
				t.Errorf("%s, seed %d: keyword=%v, want %v", c.name, seed, f.Keyword, c.keyword)
			}
		}
	}
}

func TestDecodedSpansCoverTheEncodedText(t *testing.T) {
	v := synthValue(t, "ro-cnp", 3)
	enc := base64.StdEncoding.EncodeToString([]byte(v))
	line := "a=1 b=" + enc + " c=" + percentAll(v) + " d=" + v + "%26e=1"
	fs := scanWith(t, "ro-cnp", line)
	if len(fs) != 3 {
		t.Fatalf("want three findings, got %d", len(fs))
	}
	want := []string{enc, percentAll(v), v}
	for i, f := range fs {
		if line[f.Span.Start:f.Span.End] != want[i] {
			t.Errorf("finding %d: span does not cover its text", i)
		}
	}
	if reasons := strings.Join(fs[2].Reasons, "; "); strings.Contains(reasons, "decoding") || strings.Contains(reasons, "also valid") {
		t.Errorf("a value the plain scan finds is reported once, as plain: %q", reasons)
	}
}

func TestBase64OnlyForWholeAlphanumericTokens(t *testing.T) {
	v := synthValue(t, "ro-cnp", 5)
	lines := []string{
		// Binary that happens to be base64: not decoded.
		"sig=" + base64.StdEncoding.EncodeToString([]byte{0xff, 0x10, 0x00, 0x7f, 0x80, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}),
		// A token longer than 64 characters: not decoded.
		"blob=" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 40)+v)),
		// Part of a longer token: not decoded.
		"t=" + base64.StdEncoding.EncodeToString([]byte(v))[:16] + "QUJD",
	}
	for i, line := range lines {
		if fs := scanWith(t, "ro-cnp", line); len(fs) != 0 {
			t.Errorf("line %d: got %d findings", i, len(fs))
		}
	}
	if _, ok := decode("ids: 123456789012 123456789012345678"); ok {
		t.Error("plain numbers must not be read as base64")
	}
}

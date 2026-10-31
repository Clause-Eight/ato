package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func testKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func terms() License {
	return License{ID: "lic-test-1", Org: "Example Org", Plan: "privacy", Seats: 25, Features: []string{"map"},
		Issued: "2026-11-01", Expires: "2027-10-31", Key: "test"}
}

func day(s string) time.Time {
	d, _ := time.Parse(time.DateOnly, s)
	return d.Add(12 * time.Hour)
}

func TestSignAndVerify(t *testing.T) {
	pub, priv := testKey(t)
	token, err := Sign(terms(), priv)
	if err != nil {
		t.Fatal(err)
	}
	keys := Keys{"test": pub}
	for _, tc := range []struct {
		on   string
		want Status
	}{
		{"2026-11-01", Valid},
		{"2027-10-31", Valid},
		{"2027-11-01", InGrace},
		{"2027-11-30", InGrace},
		{"2027-12-01", Expired},
	} {
		c := Verify(token, keys, day(tc.on))
		if c.Status != tc.want || c.Err != nil {
			t.Errorf("%s: %s (%v), want %s", tc.on, c.Status, c.Err, tc.want)
		}
	}
	c := Verify(token, keys, day("2027-10-30"))
	if c.License.Org != "Example Org" || c.License.Seats != 25 || c.DaysLeft(day("2027-10-30")) != 1 {
		t.Fatalf("terms or days left wrong: %+v, %d days", c.License, c.DaysLeft(day("2027-10-30")))
	}
}

func TestCommentsAroundTheTokenAreIgnored(t *testing.T) {
	pub, priv := testKey(t)
	token, _ := Sign(terms(), priv)
	file := "# ATO license for Another Org, plan everything, expires 2099-01-01\n" + token + "\n"
	if c := Verify(file, Keys{"test": pub}, day("2027-01-01")); c.Status != Valid || c.License.Org != "Example Org" {
		t.Fatalf("got %s %+v", c.Status, c.License)
	}
}

func TestTamperedOrForeignTokensAreInvalid(t *testing.T) {
	pub, priv := testKey(t)
	otherPub, otherPriv := testKey(t)
	token, _ := Sign(terms(), priv)
	cut := strings.LastIndexByte(token, '.')
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"id":"lic-test-1","org":"Example Org","plan":"privacy","issued":"2026-11-01","expires":"2099-12-31","key":"test"}`))
	foreign, _ := Sign(terms(), otherPriv)
	unknownKey := terms()
	unknownKey.Key = "retired"
	unknown, _ := Sign(unknownKey, priv)
	for name, text := range map[string]string{
		"terms changed":     Prefix + payload + token[cut:],
		"signature changed": token[:cut+1] + strings.Repeat("A", len(token)-cut-1),
		"other key":         foreign,
		"unknown key id":    unknown,
		"no token":          "nothing here",
		"not base64":        Prefix + "!!!.???",
	} {
		if c := Verify(text, Keys{"test": pub}, day("2027-01-01")); c.Status != Invalid || c.Err == nil {
			t.Errorf("%s: %s, want invalid with a reason", name, c.Status)
		}
	}
	if c := Verify(foreign, Keys{"test": otherPub}, day("2027-01-01")); c.Status != Valid {
		t.Fatalf("the other key's token should verify with that key: %s %v", c.Status, c.Err)
	}
}

func TestSignRefusesIncompleteTerms(t *testing.T) {
	_, priv := testKey(t)
	for name, edit := range map[string]func(*License){
		"no org":            func(l *License) { l.Org = "" },
		"bad date":          func(l *License) { l.Expires = "31/10/2027" },
		"ends before start": func(l *License) { l.Expires = "2026-01-01" },
		"negative seats":    func(l *License) { l.Seats = -1 },
	} {
		l := terms()
		edit(&l)
		if _, err := Sign(l, priv); err == nil {
			t.Errorf("%s: signed", name)
		}
	}
}

func TestLoadKeys(t *testing.T) {
	pub, _ := testKey(t)
	fsys := fstest.MapFS{
		"keys/2026-11.pub": {Data: []byte(base64.StdEncoding.EncodeToString(pub) + "\n")},
		"keys/README.md":   {Data: []byte("not a key")},
	}
	keys, err := LoadKeys(fsys, "keys")
	if err != nil || len(keys) != 1 || !keys["2026-11"].Equal(pub) {
		t.Fatalf("got %v, %v", keys, err)
	}
	fsys["keys/broken.pub"] = &fstest.MapFile{Data: []byte("short")}
	if _, err := LoadKeys(fsys, "keys"); err == nil {
		t.Fatal("a broken key file must be an error")
	}
	if _, err := Trusted(); err != nil {
		t.Fatalf("the embedded keys must load: %v", err)
	}
}

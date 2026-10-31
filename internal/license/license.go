// Package license reads and checks ATO license tokens. A token is the license terms in JSON, signed with an
// Ed25519 key that only Clause Eight holds; ATO carries the public keys and checks a token offline, without
// contacting any server (ADR 15). The package imports nothing from this module and reads no files.
package license

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"
)

// Prefix starts every token and names its format version.
const Prefix = "ato-license-v1."

// Grace is how long ATO keeps working as licensed after the expiry date, so a late renewal never breaks a build.
const Grace = 30 * 24 * time.Hour

// License is what a customer bought. Expires is a date: the license is valid through that whole day, UTC.
type License struct {
	ID       string   `json:"id"`
	Org      string   `json:"org"`
	Plan     string   `json:"plan"`
	Seats    int      `json:"seats,omitempty"`
	Features []string `json:"features,omitempty"`
	Issued   string   `json:"issued"`
	Expires  string   `json:"expires"`
	Key      string   `json:"key"`
}

// Status is the outcome of checking a token on a given day.
type Status int

const (
	// Missing means no token was found.
	Missing Status = iota
	// Invalid means a token was found but is malformed, signed by an unknown key or not signed by its key.
	Invalid
	// Expired means the expiry date and the grace period have passed.
	Expired
	// InGrace means the expiry date has passed but the grace period has not.
	InGrace
	// Valid means the token is signed by a known key and has not expired.
	Valid
)

func (s Status) String() string {
	return [...]string{"missing", "invalid", "expired", "in grace", "valid"}[s]
}

// Licensed reports whether ATO runs as licensed: a valid token, or one in its grace period.
func (s Status) Licensed() bool { return s == Valid || s == InGrace }

// Check is the result of checking a token.
type Check struct {
	Status  Status
	License License
	// End is when the license stops being valid (the end of its expiry day); zero unless the token was read.
	End time.Time
	// Err says why a token is invalid.
	Err error
}

// DaysLeft is the number of whole days until End, negative once it has passed.
func (c Check) DaysLeft(now time.Time) int {
	return int(c.End.Sub(now).Hours() / 24)
}

// GraceDaysLeft is the number of whole days until the grace period ends.
func (c Check) GraceDaysLeft(now time.Time) int {
	return int(c.End.Add(Grace).Sub(now).Hours() / 24)
}

// Keys maps key IDs to the public keys that sign tokens.
type Keys map[string]ed25519.PublicKey

// LoadKeys reads every <id>.pub file in a folder: the base64 of an Ed25519 public key.
func LoadKeys(fsys fs.FS, dir string) (Keys, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	keys := Keys{}
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".pub" {
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		pub, err := ParsePublicKey(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		keys[strings.TrimSuffix(e.Name(), ".pub")] = pub
	}
	return keys, nil
}

// ParsePublicKey reads the base64 of an Ed25519 public key.
func ParsePublicKey(s string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("not an Ed25519 public key in base64")
	}
	return ed25519.PublicKey(raw), nil
}

// ParsePrivateKey reads the base64 of an Ed25519 private key.
func ParsePrivateKey(s string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return nil, errors.New("not an Ed25519 private key in base64")
	}
	return ed25519.PrivateKey(raw), nil
}

// Sign makes a token for l with the private key whose ID is l.Key.
func Sign(l License, priv ed25519.PrivateKey) (string, error) {
	if err := l.validate(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(l)
	if err != nil {
		return "", err
	}
	signed := Prefix + base64.RawURLEncoding.EncodeToString(payload)
	return signed + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(signed))), nil
}

// Verify checks a token against the keys on the day now. Text around the token is ignored, so a license file
// may carry comment lines that say who it is for; those are not signed and never trusted.
func Verify(text string, keys Keys, now time.Time) Check {
	token := find(text)
	if token == "" {
		return Check{Status: Invalid, Err: errors.New("no " + strings.TrimSuffix(Prefix, ".") + " token in the text")}
	}
	cut := strings.LastIndexByte(token, '.')
	signed, sigText := token[:cut], token[cut+1:]
	payload, err1 := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(signed, Prefix))
	sig, err2 := base64.RawURLEncoding.DecodeString(sigText)
	if err1 != nil || err2 != nil {
		return Check{Status: Invalid, Err: errors.New("the token is not valid base64")}
	}
	var l License
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&l); err != nil {
		return Check{Status: Invalid, Err: errors.New("the token's terms cannot be read")}
	}
	pub, ok := keys[l.Key]
	if !ok {
		return Check{Status: Invalid, Err: fmt.Errorf("signed with key %q, which this build of ATO does not know", l.Key)}
	}
	if !ed25519.Verify(pub, []byte(signed), sig) {
		return Check{Status: Invalid, Err: errors.New("the signature does not match the terms")}
	}
	if err := l.validate(); err != nil {
		return Check{Status: Invalid, Err: err}
	}
	expires, _ := time.Parse(time.DateOnly, l.Expires)
	c := Check{License: l, End: expires.Add(24 * time.Hour)}
	switch {
	case now.Before(c.End):
		c.Status = Valid
	case now.Before(c.End.Add(Grace)):
		c.Status = InGrace
	default:
		c.Status = Expired
	}
	return c
}

// find returns the first token in text.
func find(text string) string {
	for _, f := range strings.Fields(text) {
		if strings.HasPrefix(f, Prefix) && strings.Count(f, ".") == 2 {
			return f
		}
	}
	return ""
}

func (l License) validate() error {
	if l.ID == "" || l.Org == "" || l.Plan == "" || l.Key == "" {
		return errors.New("a license needs an id, an organisation, a plan and a key")
	}
	issued, err1 := time.Parse(time.DateOnly, l.Issued)
	expires, err2 := time.Parse(time.DateOnly, l.Expires)
	if err1 != nil || err2 != nil {
		return errors.New("issued and expires must be dates such as 2027-11-01")
	}
	if expires.Before(issued) {
		return errors.New("the license expires before it is issued")
	}
	if l.Seats < 0 {
		return errors.New("seats cannot be negative")
	}
	return nil
}

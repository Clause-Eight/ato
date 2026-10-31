// Command ato-license makes signing keys and issues ATO licenses. It is Clause Eight's tool, not shipped to
// customers: the release builds only cmd/ato. Private keys are files you keep outside the repository; this tool
// reads one with -key and never prints it. See docs/LICENSES.md.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Clause-Eight/ato/v2/internal/license"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "ato-license:", err)
		os.Exit(1)
	}
}

const usage = `usage:
  ato-license keygen -id ID -out DIR     make a signing key pair: DIR/ID.key (private) and DIR/ID.pub
  ato-license issue -key FILE -org NAME -plan PLAN -expires DATE [-seats N] [-features a,b]
                    [-issued DATE] [-id ID] [-out FILE] [-ledger FILE]
                                         sign a license with a private key; the key's ID is its file name
  ato-license inspect FILE [-pub FILE]   check a license with the keys built into ATO, or with a public key file`

func run(args []string, stdout io.Writer, now time.Time) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "keygen":
		return keygen(args[1:], stdout)
	case "issue":
		return issue(args[1:], stdout, now)
	case "inspect":
		return inspect(args[1:], stdout, now)
	}
	return errors.New(usage)
}

func keygen(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	id := fs.String("id", "", "key ID, such as 2026-11")
	out := fs.String("out", "", "folder for the key files, outside any repository")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" || *out == "" || strings.ContainsAny(*id, `/\. `) {
		return errors.New("keygen needs -id (letters, digits and dashes) and -out")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o700); err != nil {
		return err
	}
	keyFile, pubFile := filepath.Join(*out, *id+".key"), filepath.Join(*out, *id+".pub")
	if err := writeNew(keyFile, base64.StdEncoding.EncodeToString(priv)+"\n", 0o600); err != nil {
		return err
	}
	if err := writeNew(pubFile, base64.StdEncoding.EncodeToString(pub)+"\n", 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Private key: %s (keep it secret and backed up)\nPublic key:  %s (copy it to internal/license/keys/ and release ATO)\n", keyFile, pubFile)
	return nil
}

func issue(args []string, stdout io.Writer, now time.Time) error {
	fs := flag.NewFlagSet("issue", flag.ContinueOnError)
	keyFile := fs.String("key", "", "private key file made by keygen")
	l := license.License{Issued: now.UTC().Format(time.DateOnly)}
	fs.StringVar(&l.Org, "org", "", "customer organisation, as on the contract")
	fs.StringVar(&l.Plan, "plan", "", "plan name")
	fs.StringVar(&l.Expires, "expires", "", "last day of the license, such as 2027-10-31")
	fs.StringVar(&l.Issued, "issued", l.Issued, "first day of the license")
	fs.StringVar(&l.ID, "id", "", "license ID (default: ato-<issued>-<random>)")
	fs.IntVar(&l.Seats, "seats", 0, "seats on the contract (0: not stated)")
	features := fs.String("features", "", "comma-separated features")
	out := fs.String("out", "", "license file to write (default: print it)")
	ledger := fs.String("ledger", "", "CSV file to append the license's terms to")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *keyFile == "" {
		return errors.New("issue needs -key")
	}
	data, err := os.ReadFile(*keyFile)
	if err != nil {
		return err
	}
	priv, err := license.ParsePrivateKey(string(data))
	if err != nil {
		return fmt.Errorf("%s: %w", *keyFile, err)
	}
	l.Key = strings.TrimSuffix(filepath.Base(*keyFile), ".key")
	if l.ID == "" {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		l.ID = "ato-" + l.Issued + "-" + hex.EncodeToString(b)
	}
	if *features != "" {
		for _, f := range strings.Split(*features, ",") {
			if f = strings.TrimSpace(f); f != "" {
				l.Features = append(l.Features, f)
			}
		}
	}
	token, err := license.Sign(l, priv)
	if err != nil {
		return err
	}
	file := fmt.Sprintf("# ATO license %s for %s: plan %s, %s to %s.\n# Install with `ato license install <this file>`, or put the line below in the ATO_LICENSE variable in CI.\n%s\n",
		l.ID, l.Org, l.Plan, l.Issued, l.Expires, token)
	if keys, err := license.Trusted(); err == nil {
		if _, ok := keys[l.Key]; !ok {
			fmt.Fprintf(os.Stderr, "ato-license: warning: key %q is not built into this ATO, so released builds will not accept this license yet\n", l.Key)
		}
	}
	if *ledger != "" {
		if err := appendLedger(*ledger, l); err != nil {
			return err
		}
	}
	if *out == "" {
		_, err := io.WriteString(stdout, file)
		return err
	}
	if err := writeNew(*out, file, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Wrote %s: %s for %s, %s to %s.\n", *out, l.ID, l.Org, l.Issued, l.Expires)
	return nil
}

func inspect(args []string, stdout io.Writer, now time.Time) error {
	if len(args) == 0 {
		return errors.New("inspect needs a license file")
	}
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	pubFile := fs.String("pub", "", "public key file to check with instead of the keys built into ATO")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	keys, err := license.Trusted()
	if err != nil {
		return err
	}
	if *pubFile != "" {
		raw, err := os.ReadFile(*pubFile)
		if err != nil {
			return err
		}
		pub, err := license.ParsePublicKey(string(raw))
		if err != nil {
			return err
		}
		keys = license.Keys{strings.TrimSuffix(filepath.Base(*pubFile), ".pub"): pub}
	}
	c := license.Verify(string(data), keys, now)
	if c.Err != nil {
		return fmt.Errorf("%s: %s: %w", args[0], c.Status, c.Err)
	}
	l := c.License
	fmt.Fprintf(stdout, "%s: %s\n  id %s, org %s, plan %s, seats %d, features %s\n  %s to %s, key %s\n",
		args[0], c.Status, l.ID, l.Org, l.Plan, l.Seats, strings.Join(l.Features, ","), l.Issued, l.Expires, l.Key)
	return nil
}

// appendLedger records an issued license, with a header line for a new file.
func appendLedger(path string, l license.License) error {
	_, err := os.Stat(path)
	header := errors.Is(err, os.ErrNotExist)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if header {
		if _, err := io.WriteString(f, "id,org,plan,seats,features,issued,expires,key\n"); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(f, "%s,%s,%s,%d,%s,%s,%s,%s\n", csv(l.ID), csv(l.Org), csv(l.Plan), l.Seats,
		csv(strings.Join(l.Features, " ")), l.Issued, l.Expires, csv(l.Key))
	return err
}

func csv(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// writeNew writes a file that must not exist yet, so a key or license is never overwritten by mistake.
func writeNew(path, content string, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(f, content); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

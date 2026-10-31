package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Clause-Eight/ato/v2/internal/license"
)

// licenseEnv holds a license token, or the path of a license file, for CI where nothing is installed.
const licenseEnv = "ATO_LICENSE"

// Seams for tests.
var (
	getenv      = os.Getenv
	configDir   = os.UserConfigDir
	now         = time.Now
	trustedKeys = license.Trusted
)

// installedLicense is where `ato license install` puts the license: <user config dir>/ato/license.
func installedLicense() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ato", "license"), nil
}

// findLicense returns the license text and where it came from: ATO_LICENSE first, then the installed file.
// An empty source means none was found.
func findLicense() (text, source string, err error) {
	if v := strings.TrimSpace(getenv(licenseEnv)); v != "" {
		if strings.HasPrefix(v, license.Prefix) {
			return v, licenseEnv, nil
		}
		data, err := os.ReadFile(v)
		if err != nil {
			return "", licenseEnv, fmt.Errorf("%s names a file that cannot be read: %w", licenseEnv, err)
		}
		return string(data), v, nil
	}
	file, err := installedLicense()
	if err != nil {
		return "", "", nil
	}
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", nil
	}
	if err != nil {
		return "", file, err
	}
	return string(data), file, nil
}

// checkLicense finds and checks the license.
func checkLicense() (license.Check, string) {
	text, source, err := findLicense()
	if source == "" {
		return license.Check{Status: license.Missing}, ""
	}
	if err != nil {
		return license.Check{Status: license.Invalid, Err: err}, source
	}
	keys, err := trustedKeys()
	if err != nil {
		return license.Check{Status: license.Invalid, Err: err}, source
	}
	return license.Verify(text, keys, now()), source
}

// renewSoon is how close to its expiry date a license starts to say so.
const renewSoon = 30

// licenseNotice is the one line a command prints on stderr when the license needs attention; ATO runs either
// way, so a missing or late license never breaks a build.
func licenseNotice(stderr io.Writer) {
	c, _ := checkLicense()
	t := now()
	switch c.Status {
	case license.Valid:
		if d := c.DaysLeft(t); d < renewSoon {
			fmt.Fprintf(stderr, "ato: the license for %s expires on %s (%d days left)\n", c.License.Org, c.License.Expires, d)
		}
	case license.InGrace:
		fmt.Fprintf(stderr, "ato: the license for %s expired on %s; it keeps working for %d more days\n", c.License.Org, c.License.Expires, c.GraceDaysLeft(t))
	default:
		fmt.Fprintln(stderr, "ato: running unlicensed (evaluation); `ato license` says why")
	}
}

func runLicense(args []string, stdout, stderr io.Writer) (int, error) {
	switch {
	case len(args) == 0 || len(args) == 1 && args[0] == "status":
		return licenseStatus(stdout), nil
	case len(args) == 2 && args[0] == "install":
		return licenseInstall(args[1], stdout)
	}
	return 0, usageError{"usage: ato license [status] | ato license install FILE"}
}

func licenseStatus(stdout io.Writer) int {
	c, source := checkLicense()
	switch c.Status {
	case license.Missing:
		file, _ := installedLicense()
		fmt.Fprintf(stdout, "No license found. Install one with `ato license install FILE` (it goes to %s), or set %s to a license or its path.\n", file, licenseEnv)
		return ExitFindings
	case license.Invalid:
		fmt.Fprintf(stdout, "The license in %s cannot be used: %v.\n", source, c.Err)
		return ExitFindings
	}
	l := c.License
	fmt.Fprintf(stdout, "License:      %s\nOrganisation: %s\nPlan:         %s\n", l.ID, l.Org, l.Plan)
	if l.Seats > 0 {
		fmt.Fprintf(stdout, "Seats:        %d\n", l.Seats)
	}
	if len(l.Features) > 0 {
		fmt.Fprintf(stdout, "Features:     %s\n", strings.Join(l.Features, ", "))
	}
	fmt.Fprintf(stdout, "Valid:        %s to %s\nRead from:    %s\n", l.Issued, l.Expires, source)
	d := c.DaysLeft(now())
	switch c.Status {
	case license.Valid:
		fmt.Fprintf(stdout, "Status:       valid, %d days left\n", d)
	case license.InGrace:
		fmt.Fprintf(stdout, "Status:       expired; grace period, %d days left\n", c.GraceDaysLeft(now()))
	default:
		fmt.Fprintln(stdout, "Status:       expired; ATO runs unlicensed (evaluation)")
		return ExitFindings
	}
	return ExitClean
}

// licenseInstall checks a license file and copies it to the user's config folder, readable by the user only.
func licenseInstall(file string, stdout io.Writer) (int, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, usageError{err.Error()}
	}
	keys, err := trustedKeys()
	if err != nil {
		return 0, err
	}
	c := license.Verify(string(data), keys, now())
	if !c.Status.Licensed() {
		reason := c.Status.String()
		if c.Err != nil {
			reason = c.Err.Error()
		}
		return 0, usageError{fmt.Sprintf("%s is not a license this build of ATO accepts: %s", file, reason)}
	}
	dest, err := installedLicense()
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return 0, err
	}
	if err := os.WriteFile(dest, data, 0o600); err != nil {
		return 0, err
	}
	fmt.Fprintf(stdout, "Installed the license for %s (%s, until %s) in %s.\n", c.License.Org, c.License.Plan, c.License.Expires, dest)
	return ExitClean, nil
}

package detectors

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Analytics and advertising cookies hold an ID for one browser, which is an online identifier under GDPR
// Recital 30. Each has a fixed prefix and a creation time, so a value whose time is not a plausible date is not
// reported.
//
// Google Analytics _ga: GA1.<domain depth>.<random>.<first visit>, where the random part is an unsigned 31-bit
// integer (1 to 2147483647) and the first visit is a Unix time in seconds; the last two parts are the client ID
// (https://www.bounteous.com/insights/2019/12/23/how-google-analytics-uses-cookies-identify-users/, cookie names
// at https://support.google.com/analytics/answer/11397207). The _ga cookie came with Universal Analytics in 2012.
//
// Meta Pixel _fbp and _fbc: fb.<subdomain index>.<creation time>.<random number> for _fbp, and the same with the
// fbclid parameter of the landing page in place of the random number for _fbc. The creation time is a Unix time
// in milliseconds (https://developers.facebook.com/docs/marketing-api/conversions-api/parameters/fbp-and-fbc/).
// The cookies date from the first-party pixel of 2018.
//
// TikTok's _ttp and ttclid are left out: TikTok publishes no format for them, only that they are opaque IDs.
func init() {
	Register(Detector{
		Name:        "ga-client-id",
		Label:       "Google Analytics client ID (_ga cookie)",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`GA1\.\d\.\d{1,10}\.\d{10}`),
		Validate:    validGAClientID,
		Mask:        maskAfterPrefix,
		Keywords:    []string{"ga cookie", "ga client id", "client id", "clientid", "cid"},
		Base:        domain.Medium,
		Validation:  "random part in range and a first-visit time that is a plausible date",
		Needs:       Needs{AnyOf: []string{"GA1."}},
	})
	Register(Detector{
		Name:        "meta-pixel-cookie",
		Label:       "Meta Pixel browser or click ID (_fbp, _fbc cookie)",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`fb\.\d\.\d{13}\.[A-Za-z0-9_-]{6,}`),
		Validate:    validMetaPixelCookie,
		Mask:        maskAfterPrefix,
		Keywords:    []string{"fbp", "fbc", "fbclid", "fb browser id", "fb click id"},
		Base:        domain.Medium,
		Validation:  "a creation time that is a plausible date",
		Needs:       Needs{AnyOf: []string{"fb."}},
	})
}

var (
	gaSince   = time.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC)
	metaSince = time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
)

// plausibleTime reports whether t is after since and no later than a day from now.
func plausibleTime(t, since time.Time) bool {
	return !t.Before(since) && !t.After(time.Now().Add(24*time.Hour))
}

func validGAClientID(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 || parts[1] == "0" {
		return false
	}
	random, err1 := strconv.ParseInt(parts[2], 10, 64)
	secs, err2 := strconv.ParseInt(parts[3], 10, 64)
	if err1 != nil || err2 != nil || random < 1 || random > 2147483647 {
		return false
	}
	return plausibleTime(time.Unix(secs, 0), gaSince)
}

// validMetaPixelCookie accepts a random number (_fbp) or an fbclid (_fbc), which carries letters.
func validMetaPixelCookie(s string) bool {
	parts := strings.SplitN(s, ".", 4)
	if len(parts) != 4 {
		return false
	}
	ms, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || !plausibleTime(time.UnixMilli(ms), metaSince) {
		return false
	}
	last := parts[3]
	if DigitsOnly(last) == last {
		return len(last) <= 20 && strings.Trim(last, "0") != ""
	}
	return true
}

// maskAfterPrefix keeps the fixed prefix (GA1., fb.) and hides the rest.
func maskAfterPrefix(v string) string {
	i := strings.IndexByte(v, '.')
	if i < 0 {
		return MaskAll(v)
	}
	return v[:i+1] + MaskAll(v[i+1:])
}

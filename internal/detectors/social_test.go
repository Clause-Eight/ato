package detectors

import (
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

func TestLinkedInProfile(t *testing.T) {
	d := registered(t, "linkedin-profile")
	checkCases(t, "linkedin-profile", map[string]bool{
		"https://www.linkedin.com/in/kalo-mitas":          true,
		"linkedin.com/in/renvo-dranel-3f9a0c71":           true,
		"https://ro.linkedin.com/in/sorul":                true, // a country subdomain
		"https://www.linkedin.com/in/j%C3%B3zef-kalo":     true,
		"https://www.linkedin.com/in/yourname":            false, // a template placeholder
		"https://www.linkedin.com/in/username":            false,
		"https://api.linkedin.com/in/kalo-mitas":          false,
		"https://www.linkedin.com/in/my-linkedin-profile": false, // LinkedIn does not allow its own name
		"https://www.linkedin.com/in/ab":                  false, // shorter than three characters
	})
	for _, line := range []string{
		`<a href="https://www.linkedin.com/company/acme-gmbh">LinkedIn</a>`,
		`share: "https://www.linkedin.com/sharing/share-offsite/?url=https://acme.example"`,
		`"https://www.linkedin.com/school/kalo-university/"`,
	} {
		if d.Pattern.MatchString(line) {
			t.Errorf("a company, school or share link is not a person's profile")
		}
	}
	if !d.OnByDefault || d.NeedsKeyword {
		t.Errorf("linkedin-profile always names a person: on by default, no keyword needed")
	}
	if m := d.Mask("https://www.linkedin.com/in/kalo-mitas"); m != "https://www.linkedin.com/in/••••••••••" {
		t.Errorf("the mask must keep the site and hide the account")
	}
}

func TestSocialProfile(t *testing.T) {
	d := registered(t, "social-profile")
	checkCases(t, "social-profile", map[string]bool{
		"https://www.facebook.com/kalo.mitas":                 true,
		"https://facebook.com/profile.php?id=100012345678901": true,
		"instagram.com/renvo_dranel":                          true,
		"https://x.com/sorul_bex":                             true,
		"https://twitter.com/finka42":                         true,
		"https://www.tiktok.com/@kalo.mitas":                  true,
		"https://m.facebook.com/kalo.mitas":                   true,
		"https://www.facebook.com/sharer.php":                 false, // the site's own pages
		"https://www.facebook.com/groups":                     false,
		"https://www.facebook.com/policies":                   false,
		"https://graph.facebook.com/v18.0":                    false, // an API host
		"https://www.instagram.com/explore":                   false,
		"https://www.instagram.com/accounts":                  false,
		"https://twitter.com/intent":                          false,
		"https://twitter.com/share":                           false,
		"https://twitter.com/hashtag":                         false,
		"https://x.com/home":                                  false,
		"https://api.twitter.com/oauth":                       false,
		"https://x.com/yourhandle":                            false, // a template placeholder
		"https://www.facebook.com/12345678":                   false, // digits alone are not a username
		"https://www.facebook.com/.kalo":                      false,
	})
	for _, line := range []string{
		`<a href="https://twitter.com/intent/tweet?text=hi">Share</a>`,
		`url: "https://www.facebook.com/sharer/sharer.php?u=https://acme.example"`,
		`"https://x.com/en/tos"`,
		`user@x.com`,
	} {
		if m := d.Pattern.FindString(line); m != "" && d.Validate(m) {
			t.Errorf("a share link, the site's own page or an e-mail domain is not a profile")
		}
	}
	// A footer's own accounts look like anyone's, so the detector is off by default and starts low: a link
	// such as facebook.com/acmeshop next to an "Instagram" label is not reported at the default level.
	if d.OnByDefault || d.Base != domain.Low {
		t.Errorf("social-profile must be off by default and start low")
	}
}

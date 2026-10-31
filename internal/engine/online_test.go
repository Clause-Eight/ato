package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// TestSocialProfilesInFootersStayLow: a site's own accounts in a footer look like anyone's profile, so they
// stay low; a person's record (a name field next to the link) or a profile word raises them, and a LinkedIn /in/
// link is a person wherever it is.
func TestSocialProfilesInFootersStayLow(t *testing.T) {
	text := strings.Join([]string{
		`<a href="https://www.facebook.com/kalomitasshop" aria-label="Facebook">`,
		`<a href="https://www.instagram.com/kalomitas.shop">Instagram</a>`,
		`<a href="https://twitter.com/intent/tweet?url=https://shop.example">Share</a>`,
		`<a href="https://www.linkedin.com/company/kalomitas">LinkedIn</a>`,
		`{"full_name": "Renvo Dranel", "instagram": "https://instagram.com/renvo_dranel"}`,
		`profile_url: "https://x.com/sorul_bex"`,
		`<a href="https://www.linkedin.com/in/kalo-mitas">Kalo</a>`,
		`target: "dns:///storage.example.com"`,
		`meet at ///kalo.mitas.renvo`,
	}, "\n") + "\n"
	fs, err := ScanReader("web/footer.html", strings.NewReader(text), detectors.All())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domain.Confidence{}
	for _, f := range fs {
		got[fmt.Sprintf("%d:%s", f.Location.Line, f.Detector)] = f.Confidence
	}
	for _, line := range []string{"1:social-profile", "2:social-profile"} {
		if got[line] != domain.Low {
			t.Errorf("%s: a footer link should be low, got %v", line, got[line])
		}
	}
	for _, line := range []string{"3:social-profile", "4:linkedin-profile", "8:what3words"} {
		if _, ok := got[line]; ok {
			t.Errorf("%s: a share link, a company page or a URL scheme is not personal data", line)
		}
	}
	for _, line := range []string{"5:social-profile", "6:social-profile", "7:linkedin-profile", "9:what3words"} {
		if got[line] < domain.Medium {
			t.Errorf("%s: want medium or more, got %v", line, got[line])
		}
	}
}

package detectors

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Social profile URLs name an account, and an account is usually a person.
//
// LinkedIn public profiles are linkedin.com/in/<custom URL>, where the custom part is 3 to 100 letters, digits
// and hyphens (https://www.linkedin.com/help/linkedin/answer/a542685). Companies live under /company/ and
// schools under /school/, so /in/ always names a person: the detector is on and starts at medium.
//
// Facebook, Instagram, X and TikTok put people and organisations at the same kind of address, and the usual
// such URL in code is the product's own account in a footer or a share link. That detector is off by default
// and starts low; a profile word or another personal field on the line raises it. Usernames:
//
//	facebook.com/<name>         5 to 50 letters, digits and periods, or profile.php?id=<number>
//	                            (https://www.facebook.com/help/105399436216001)
//	instagram.com/<name>        1 to 30 letters, digits, periods and underscores
//	                            (https://help.instagram.com/370452623149242)
//	x.com/<name>, twitter.com   4 to 15 letters, digits and underscores
//	                            (https://help.x.com/managing-your-account/x-username-rules)
//	tiktok.com/@<name>          2 to 24 letters, digits, periods and underscores
//	                            (https://support.tiktok.com/en/getting-started/setting-up-your-profile/changing-your-username)
//
// The sites' own pages (share, intent, login, policies, help, hashtags, ...) are not profiles, and neither are
// API hosts such as graph.facebook.com or api.twitter.com, nor placeholder names such as yourname.
func init() {
	Register(Detector{
		Name:        "linkedin-profile",
		Label:       "LinkedIn profile URL",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`(?:https?://)?(?:[A-Za-z0-9-]+\.)*linkedin\.com/in/[\p{L}\p{N}%_-]{3,100}`),
		Validate:    validLinkedInProfile,
		Mask:        maskProfile,
		Keywords:    []string{"linkedin url", "linkedin profile", "linkedinurl", "profile url", "profileurl", "social profile"},
		Base:        domain.Medium,
		Validation:  "a /in/ profile address with a custom URL of 3 to 100 characters",
		Needs:       Needs{AnyOf: []string{"linkedin.com/in/"}},
	})
	Register(Detector{
		Name:        "social-profile",
		Label:       "Social media profile URL (Facebook, Instagram, X, TikTok)",
		Country:     "EU",
		OnByDefault: false,
		Pattern: regexp.MustCompile(`(?:https?://)?(?:[A-Za-z0-9-]+\.)*(?:` +
			`facebook\.com/(?:profile\.php\?id=\d{5,20}|[A-Za-z0-9.]{5,50})` +
			`|instagram\.com/[A-Za-z0-9._]{1,30}` +
			`|(?:twitter|x)\.com/[A-Za-z0-9_]{4,15}` +
			`|tiktok\.com/@[A-Za-z0-9._]{2,24})`),
		Validate:   validSocialProfile,
		Mask:       maskProfile,
		Keywords:   []string{"social profile", "profile url", "profileurl", "social url", "social link", "facebook url", "instagram handle", "twitter handle", "tiktok handle"},
		Base:       domain.Low,
		Validation: "a profile address with a username the site allows, not one of its own pages",
		Needs:      Needs{AnyOf: []string{"facebook.com/", "instagram.com/", "twitter.com/", "x.com/", "tiktok.com/@"}},
	})
}

// profileHosts are the subdomains a profile page is served on; country subdomains (de.linkedin.com) also count
// for LinkedIn.
var profileHosts = map[string]bool{"": true, "www": true, "m": true, "mobile": true, "web": true}

// placeholderHandles are names that stand in for a real account in templates and documentation.
var placeholderHandles = map[string]bool{
	"username": true, "yourusername": true, "your_username": true, "yourname": true, "your-name": true, "yourprofile": true,
	"your-profile": true, "yourhandle": true, "handle": true, "user": true, "example": true, "johndoe": true, "john-doe": true,
	"janedoe": true, "jane-doe": true, "profile": true, "name": true, "xxx": true, "xxxxx": true, "me": true, "someone": true,
}

// socialReserved are first path segments that are the site's own pages rather than accounts.
var socialReserved = map[string]map[string]bool{
	"facebook": words(strings.Fields("pages groups events share sharer dialog plugins login logout home about privacy policies policy help " +
		"settings marketplace watch gaming business ads legal terms connect recover notes photos videos people hashtag " +
		"places search messages notifications friends bookmarks profile permalink story stories reel reels live fundraisers")...),
	"instagram": words(strings.Fields("p reel reels explore stories accounts about developer legal direct tv web static privacy terms help " +
		"challenge emails oauth api graphql press blog session login")...),
	"x": words(strings.Fields("home intent share i search hashtag explore settings login logout signup tos privacy rest download web oauth " +
		"streaming messages notifications compose about help jobs account rules policies following followers lists " +
		"status statuses widgets oauth2 personalization")...),
	"tiktok": nil,
}

// splitProfile returns the subdomain, the site's name (linkedin, facebook, ...) and the path with its query.
func splitProfile(s string) (sub, site, path string, ok bool) {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	slash := strings.IndexByte(s, '/')
	if slash < 0 {
		return "", "", "", false
	}
	host, path := strings.ToLower(s[:slash]), s[slash+1:]
	labels := strings.Split(host, ".")
	if len(labels) < 2 || labels[len(labels)-1] != "com" {
		return "", "", "", false
	}
	site = labels[len(labels)-2]
	if site == "twitter" {
		site = "x"
	}
	return strings.Join(labels[:len(labels)-2], "."), site, path, true
}

func validLinkedInProfile(s string) bool {
	sub, site, path, ok := splitProfile(s)
	if !ok || site != "linkedin" || !profileHosts[sub] && !(len(sub) == 2 && sub != "m") {
		return false
	}
	slug, err := url.PathUnescape(strings.TrimPrefix(path, "in/"))
	if err != nil || len([]rune(slug)) < 3 || placeholderHandles[strings.ToLower(slug)] {
		return false
	}
	return strings.Trim(slug, "-_") != "" && !strings.Contains(strings.ToLower(slug), "linkedin")
}

func validSocialProfile(s string) bool {
	sub, site, path, ok := splitProfile(s)
	if !ok || !profileHosts[sub] {
		return false
	}
	reserved, known := socialReserved[site]
	if !known {
		return false
	}
	name := path
	switch site {
	case "facebook":
		if strings.HasPrefix(name, "profile.php?id=") {
			return strings.Trim(name[len("profile.php?id="):], "0") != ""
		}
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") || strings.Contains(name, ".php") {
			return false
		}
	case "instagram":
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") {
			return false
		}
	case "tiktok":
		name = strings.TrimPrefix(name, "@")
	}
	lower := strings.ToLower(name)
	if reserved[lower] || placeholderHandles[lower] || strings.Trim(lower, "_.") == "" {
		return false
	}
	return strings.ContainsAny(lower, "abcdefghijklmnopqrstuvwxyz")
}

// maskProfile keeps the site and hides the account.
func maskProfile(v string) string {
	i := strings.LastIndexAny(v, "/=@")
	if i < 0 || i == len(v)-1 {
		return MaskAll(v)
	}
	return v[:i+1] + MaskAll(v[i+1:])
}

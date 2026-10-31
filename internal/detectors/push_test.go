package detectors

import (
	"strings"
	"testing"
)

func TestFCMToken(t *testing.T) {
	d := registered(t, "fcm-token")
	reg := "APA91b" + strings.Repeat("Hq3_x-Z9", 17)
	checkCases(t, "fcm-token", map[string]bool{
		"dQw4w9WgXcQ-_abcdefghi:" + reg:      true,
		"fZZZZZZZZZZZZZZZZZZZZZ:" + reg:      true,
		reg:                                  true, // an older GCM token
		"dQw4w9WgXcQ-_abcdefghi:APA91bShort": false,
		"dQw4w9WgXcQ-_abcdefghi:" + strings.Repeat("Hq3_x-Z9", 18): false, // no APA91b
	})
	if d.Pattern.FindString("aQw4w9WgXcQ-_abcdefghi:"+reg) == "aQw4w9WgXcQ-_abcdefghi:"+reg {
		t.Errorf("an installation ID starts with c, d, e or f")
	}
	if !d.NeedsKeyword || d.OnByDefault {
		t.Errorf("fcm-token must need a keyword and be off by default")
	}
}

func TestAPNSToken(t *testing.T) {
	d := registered(t, "apns-token")
	token := strings.Repeat("3f9a0c71", 8)
	checkCases(t, "apns-token", map[string]bool{
		token:                         true,
		strings.ToUpper(token):        true,
		strings.Repeat("0", 64):       false,
		strings.Repeat("f", 64):       false,
		token[:62]:                    false,
		token + "ab":                  false,
		strings.Repeat("3f9a0g71", 8): false,
	})
	if d.Pattern.FindString("3f9a0c71"+strings.ToUpper(token[8:])) != "" {
		t.Errorf("mixed case is not how a token is printed")
	}
	if !d.NeedsKeyword || d.OnByDefault {
		t.Errorf("apns-token must need a keyword and be off by default")
	}
	if !d.Needs.Check("deviceToken="+token) || d.Needs.Check("sha: 3f9a0c71") {
		t.Errorf("prefilter")
	}
}

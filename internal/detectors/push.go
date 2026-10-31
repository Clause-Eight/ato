package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Push notification tokens address one app on one device, so they are online identifiers (GDPR Recital 30).
//
// Firebase Cloud Messaging registration tokens start with the app's Firebase installation ID: 22 base64url
// characters whose first four bits are the header 0111, so the first character is c, d, e or f
// (VALID_FID_PATTERN in https://github.com/firebase/firebase-js-sdk/blob/master/packages/installations/src/helpers/generate-fid.ts;
// https://firebase.google.com/docs/cloud-messaging/manage-tokens). A colon and the registration part follow,
// which begins APA91b, as did the older GCM tokens that have no installation ID
// (https://help.pushwoosh.com/hc/en-us/articles/360000364923-What-is-a-Device-token).
//
// Apple Push Notification service device tokens are opaque data, 32 bytes on current devices, written as 64 hex
// characters (https://developer.apple.com/documentation/usernotifications/registering-your-app-with-apns). A
// SHA-256 digest has the same shape, so both detectors need a keyword.
func init() {
	Register(Detector{
		Name:         "fcm-token",
		Label:        "Firebase Cloud Messaging registration token",
		Country:      "EU",
		OnByDefault:  false,
		Pattern:      regexp.MustCompile(`[c-f][A-Za-z0-9_-]{21}:APA91b[A-Za-z0-9_-]{100,}|APA91b[A-Za-z0-9_-]{100,}`),
		Validate:     validFCMToken,
		Mask:         MaskAll,
		Keywords:     []string{"fcm token", "fcmtoken", "fcm", "gcm", "registration id", "registrationid", "registration ids", "registration token", "registrationtoken", "push token", "pushtoken", "device token", "devicetoken"},
		Base:         domain.Low,
		Validation:   "installation ID header and length of a registration token",
		NeedsKeyword: true,
		Needs:        Needs{AnyOf: []string{"APA91b"}},
	})
	Register(Detector{
		Name:         "apns-token",
		Label:        "Apple push notification device token",
		Country:      "EU",
		OnByDefault:  false,
		Pattern:      regexp.MustCompile(`[0-9a-f]{64}|[0-9A-F]{64}`),
		Normalize:    strings.ToLower,
		Validate:     validAPNSToken,
		Keywords:     []string{"apns", "apns token", "apnstoken", "apns device token", "device token", "devicetoken", "push token", "pushtoken", "voip token", "voiptoken"},
		Base:         domain.Low,
		Validation:   "64 hex characters, not a run of one character",
		NeedsKeyword: true,
		Needs:        Needs{Check: maybeHex64},
	})
}

func validFCMToken(s string) bool {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		s = s[i+1:]
	}
	return strings.HasPrefix(s, "APA91b") && len(s) >= 106 && len(s) <= 400
}

func validAPNSToken(s string) bool {
	if len(s) != 64 || strings.Trim(s, s[:1]) == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isHex(s[i]) {
			return false
		}
	}
	return true
}

// maybeHex64 looks for a run of 64 hex characters.
func maybeHex64(line string) bool {
	run := 0
	for i := 0; i < len(line); i++ {
		if isHex(line[i]) {
			run++
			if run >= 64 {
				return true
			}
		} else {
			run = 0
		}
	}
	return false
}

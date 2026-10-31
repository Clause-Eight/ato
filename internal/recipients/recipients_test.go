package recipients

import (
	"strings"
	"testing"
)

func names(path, text string) string {
	var out []string
	for _, u := range Find(path, []byte(text)) {
		out = append(out, u.Name+"/"+u.Evidence)
	}
	return strings.Join(out, ",")
}

func TestFind(t *testing.T) {
	cases := []struct{ path, text, want string }{
		{"package.json", `{"dependencies": {"@sentry/node": "^8.0.0", "stripe": "^16.0.0", "react": "18.3.1"}}`, "Sentry/dependency,Stripe/dependency"},
		{"requirements.txt", "# payments\nstripe==10.1.0\nopenai>=1.40\nrequests\n", "Stripe/dependency,OpenAI/dependency"},
		{"go.mod", "module example.test/app\n\nrequire (\n\tgithub.com/stripe/stripe-go/v79 v79.1.0\n\tgithub.com/getsentry/sentry-go v0.28.0\n)\n", "Stripe/dependency,Sentry/dependency"},
		{"pom.xml", "<dependency>\n<groupId>com.stripe</groupId><artifactId>stripe-java</artifactId>\n</dependency>", "Stripe/dependency"},
		{"build.gradle", `implementation "io.sentry:sentry:7.0.0"`, "Sentry/dependency"},
		{"composer.json", `{"require": {"sentry/sentry-laravel": "^4.0"}}`, "Sentry/dependency"},
		{"app.py", "import stripe\nfrom mixpanel import Mixpanel\nimport os\n", "Stripe/import,Mixpanel/import"},
		{"main.go", "import (\n\t\"fmt\"\n\t\"github.com/stripe/stripe-go/v79/customer\"\n)\n", "Stripe/import"},
		{"web.ts", "import Stripe from 'stripe'\nimport * as Sentry from \"@sentry/node/tracing\"\n", "Stripe/import,Sentry/import"},
		{"App.java", "import com.twilio.Twilio;\nimport java.util.List;\n", "Twilio/import"},
		{"Pay.php", "use Stripe\\StripeClient;\n", "Stripe/import"},
		{"Mail.cs", "using SendGrid;\n", "SendGrid/import"},
		{"client.rb", "require 'twilio-ruby'\n", ""},
		{"config.yml", "endpoint: https://api-eu.mixpanel.com/track\nwebhook: https://hooks.slack.com/services/T000/B000/XXXX\n", "Mixpanel/host,Slack/host"},
		{"notes.go", "// see https://example.test/stripe.com.html\nvar x = \"notstripe\"\n", ""},
		{"package.json", `{"dependencies": {"logrocket": "^9.0.0", "@fullstory/browser": "^2.0.0", "@microsoft/clarity": "^1.0.0", "smartlook-client": "^10.0.0", "react-facebook-pixel": "^1.0.4", "react-gtm-module": "^2.0.11", "tiktok-pixel": "^2.0.0", "react-linkedin-insight": "^0.1.3", "@heap/react-native-heap": "^0.22.0"}}`,
			"LogRocket/dependency,FullStory/dependency,Microsoft Clarity/dependency,Smartlook/dependency,Meta Pixel and Conversions API/dependency,Google Analytics/dependency,TikTok Pixel and Events API/dependency,LinkedIn Insight Tag/dependency,Heap/dependency"},
		{"index.html", "<script async src=\"https://www.googletagmanager.com/gtag/js?id=G-XXXX\"></script>\n<script src=\"https://snap.licdn.com/li.lms-analytics/insight.min.js\"></script>\n", "Google Analytics/host,LinkedIn Insight Tag/host"},
		{"layout.html", "<script>(function(c,l,a,r,i,t,y){t=l.createElement(r);t.src=\"https://www.clarity.ms/tag/\"+i;})(window,document,\"clarity\",\"script\",\"abc\");</script>", "Microsoft Clarity/host"},
		{"base.html.twig", "r.src=t+h._hjSettings.hjid+j+h._hjSettings.hjsv;})(window,document,'https://static.hotjar.com/c/hotjar-','.js?sv=');", "Hotjar/host"},
		{"App.vue", "window['_fs_host'] = 'fullstory.com';\nn.src = 'https://edge.fullstory.com/s/fs.js';\n", "FullStory/host"},
		{"pixel.erb", "s.src='https://connect.facebook.net/en_US/fbevents.js';\nt.src='https://analytics.tiktok.com/i18n/pixel/events.js';\n", "Meta Pixel and Conversions API/host,TikTok Pixel and Events API/host"},
		{"tag.html", "<script src=\"//cdn.mouseflow.com/projects/abc.js\"></script><script src=\"https://t.contentsquare.net/uxa/abc.js\"></script>", "Mouseflow/host,Contentsquare/host"},
		{"snippet.js", "!function(){window.heap=window.heap||[];var r=document.createElement('script');r.src='https://cdn.us.heap-api.com/config/'+e+'/heap_config.js'}()", "Heap/host"},
		{"snippet.js", "posthog.init('phc_x', { api_host: 'https://eu.i.posthog.com' });\np.src = 'https://us-assets.i.posthog.com/static/array.js';\n", "PostHog/host"},
		{"build.gradle", "implementation 'com.mixpanel.android:mixpanel-android:7.5.0'\nimplementation 'com.amplitude:analytics-android:1.16.0'\nimplementation 'com.microsoft.clarity:clarity:3.0.0'\n", "Mixpanel/dependency,Amplitude/dependency,Microsoft Clarity/dependency"},
		{"Track.kt", "import com.smartlook.android.core.api.Smartlook\nimport com.contentsquare.android.Contentsquare\nimport com.logrocket.core.SDK\nimport androidx.core.app.ActivityCompat\n", "Smartlook/import,Contentsquare/import,LogRocket/import"},
		{"Podfile", "platform :ios, '15.0'\npod 'Mixpanel-swift'\npod 'FBSDKCoreKit'\npod 'Alamofire'\n", "Mixpanel/dependency,Meta Pixel and Conversions API/dependency"},
		{"AppDelegate.swift", "import UIKit\nimport FullStory\nimport AmplitudeSwift\nimport SmartlookAnalytics\n", "FullStory/import,Amplitude/import,Smartlook/import"},
		{"AppDelegate.m", "#import <UIKit/UIKit.h>\n#import <FBSDKCoreKit/FBSDKCoreKit.h>\n@import FullStory;\n", "Meta Pixel and Conversions API/import,FullStory/import"},
		{"about.html", "<p>We do not use heatmaps or session recording.</p>\n", ""},
	}
	for _, c := range cases {
		if got := names(c.path, c.text); got != c.want {
			t.Errorf("%s: got %q, want %q", c.path, got, c.want)
		}
	}
}

func TestCandidate(t *testing.T) {
	for p, want := range map[string]bool{"index.html": true, "views/layout.erb": true, "App.vue": true, "ios/AppDelegate.swift": true, "ios/Podfile": true, "package.json": true, "notes.txt": false, "logo.svg": false} {
		if Candidate(p) != want {
			t.Errorf("%s: want %v", p, want)
		}
	}
}

func TestOncePerRecipientAndEvidence(t *testing.T) {
	got := Find("app.py", []byte("import stripe\nimport stripe.checkout\nURL = 'https://api.stripe.com/v1'\n"))
	if len(got) != 2 || got[0].Location.Line != 1 || got[1].Evidence != FromHost {
		t.Errorf("got %+v", got)
	}
}

func TestCatalogIsConsistent(t *testing.T) {
	seen := map[string]string{}
	for _, r := range Catalog() {
		if r.Name == "" || r.Purpose == "" || len(r.Packages)+len(r.Hosts) == 0 {
			t.Errorf("incomplete recipient %+v", r)
		}
		for _, p := range r.Packages {
			if other, dup := seen[strings.ToLower(p)]; dup {
				t.Errorf("package %q belongs to %s and %s", p, other, r.Name)
			}
			seen[strings.ToLower(p)] = r.Name
		}
	}
}

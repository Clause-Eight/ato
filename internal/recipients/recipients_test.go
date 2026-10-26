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
	}
	for _, c := range cases {
		if got := names(c.path, c.text); got != c.want {
			t.Errorf("%s: got %q, want %q", c.path, got, c.want)
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

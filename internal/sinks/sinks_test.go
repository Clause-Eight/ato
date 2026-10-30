package sinks

import (
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/internal/domain"
)

func scan(path, code string) []domain.Finding {
	s := NewScanner(path)
	var out []domain.Finding
	for i, line := range strings.Split(code, "\n") {
		out = append(out, s.Line(i+1, line)...)
	}
	return out
}

type want struct {
	sink       domain.Sink
	masked     string
	category   domain.DataCategory
	confidence domain.Confidence
}

func TestSinkCalls(t *testing.T) {
	for _, c := range []struct {
		path, code string
		want       []want
	}{
		{"app/handler.go", `log.Printf("signup %s", u.Email)`, []want{{domain.SinkLog, "u.Email", domain.CategoryContact, domain.Medium}}},
		{"app/handler.go", `slog.Info("signup", "phone", phone)`, []want{{domain.SinkLog, `"phone"`, domain.CategoryContact, domain.Medium}}},
		{"app/handler.go", `fmt.Printf("%+v\n", currentUser)`, []want{{domain.SinkLog, "currentUser", "", domain.Low}}},
		{"app/handler.go", `fmt.Fprintf(os.Stderr, "iban %s\n", acct.IBAN)`, []want{{domain.SinkLog, "acct.IBAN", domain.CategoryFinancial, domain.Medium}}},
		{"app/handler.go", `log.Info().Str("user", "x").Str("dob", c.DateOfBirth).Msg("loaded")`, []want{{domain.SinkLog, `"dob"`, domain.CategoryBirth, domain.Medium}}},
		{"app/handler.go", `sentry.CaptureMessage("lookup failed for " + customer.Iban)`, []want{{domain.SinkErrorTracker, "customer.Iban", domain.CategoryFinancial, domain.Medium}}},
		{"app/track.go", `client.Enqueue(analytics.Identify{UserId: id, Traits: analytics.NewTraits().SetEmail(u.Email)})`, []want{{domain.SinkAnalytics, "u.Email", domain.CategoryContact, domain.Medium}}},
		{"app/views.py", `logger.info(f"created {user.first_name} {user.last_name}")`, []want{{domain.SinkLog, "user.first_name", domain.CategoryName, domain.Medium}}},
		{"app/views.py", `logging.debug("payload: %s", request.json)`, []want{{domain.SinkLog, "request.json", "", domain.Medium}}},
		{"app/views.py", `print(patient.diagnosis)`, []want{{domain.SinkLog, "patient.diagnosis", domain.CategorySpecial, domain.Medium}}},
		{"app/views.py", `sentry_sdk.set_user({"email": user.email})`, []want{{domain.SinkErrorTracker, `"email"`, domain.CategoryContact, domain.Medium}}},
		{"app/views.py", "logger.warning(\n    \"retry for %s\",\n    customer.phone_number,\n)", []want{{domain.SinkLog, "customer.phone_number", domain.CategoryContact, domain.Medium}}},
		{"web/app.ts", "console.log(`order for ${customer.email}`)", []want{{domain.SinkLog, "customer.email", domain.CategoryContact, domain.Medium}}},
		{"web/app.ts", `console.error(req.body)`, []want{{domain.SinkLog, "req.body", "", domain.Medium}}},
		{"web/app.js", `this.logger?.debug("user", user)`, []want{{domain.SinkLog, "user", "", domain.Low}}},
		{"web/app.js", `analytics.identify(user.id, { email: user.email, plan: user.plan })`, []want{{domain.SinkAnalytics, "email", domain.CategoryContact, domain.Medium}}},
		{"web/app.js", `mixpanel.people.set({ $phone: profile.phone })`, []want{{domain.SinkAnalytics, "$phone", domain.CategoryContact, domain.Medium}}},
		{"web/app.js", `Sentry.setUser({ id: user.id, ip_address: ip })`, []want{{domain.SinkErrorTracker, "ip_address", domain.CategoryOnline, domain.Low}}},
		{"web/app.js", `posthog.capture("signed_up", { birthday: form.birthday })`, []want{{domain.SinkAnalytics, "birthday", domain.CategoryBirth, domain.Medium}}},
		{"web/app.js", `gtag("event", "purchase", { email: buyer.email })`, []want{{domain.SinkAnalytics, "email", domain.CategoryContact, domain.Medium}}},
		{"src/Svc.java", `log.info("Saved customer {}", customer.getEmail());`, []want{{domain.SinkLog, "customer.getEmail()", domain.CategoryContact, domain.Medium}}},
		{"src/Svc.java", `System.out.println(user);`, []want{{domain.SinkLog, "user", "", domain.Low}}},
		{"src/Svc.java", `FirebaseCrashlytics.getInstance().setCustomKey("phone", user.getPhone());`, []want{{domain.SinkErrorTracker, "user.getPhone()", domain.CategoryContact, domain.Medium}}},
		{"src/Svc.kt", `println("hello ${user.fullName}")`, []want{{domain.SinkLog, "user.fullName", domain.CategoryName, domain.Medium}}},
		{"src/Svc.kt", `logger.debug("address $address")`, nil},
		{"src/Svc.kt", `logger.debug("address ${customer.address}")`, []want{{domain.SinkLog, "customer.address", domain.CategoryAddress, domain.Medium}}},
		{"app/models/user.rb", `Rails.logger.info "signup #{user.email}"`, []want{{domain.SinkLog, "user.email", domain.CategoryContact, domain.Medium}}},
		{"app/models/user.rb", `puts params.inspect`, []want{{domain.SinkLog, "params.inspect", "", domain.Medium}}},
		{"app/models/user.rb", `Sentry.set_user(email: current_user.email)`, []want{{domain.SinkErrorTracker, "email", domain.CategoryContact, domain.Medium}}},
		{"app/models/user.rb", `Analytics.track(user_id: user.id, event: "Signed Up", properties: { phone: user.phone })`, []want{{domain.SinkAnalytics, "phone", domain.CategoryContact, domain.Medium}}},
		{"src/Ctl.php", `$this->logger->info("login", ['email' => $user->email]);`, []want{{domain.SinkLog, "'email'", domain.CategoryContact, domain.Medium}}},
		{"src/Ctl.php", `Log::debug("request", $request->all());`, []want{{domain.SinkLog, "$request->all()", "", domain.Medium}}},
		{"src/Ctl.php", `error_log("signup {$user->phone}");`, []want{{domain.SinkLog, "$user->phone", domain.CategoryContact, domain.Medium}}},
		{"src/Ctl.php", `error_log(print_r($_POST, true));`, []want{{domain.SinkLog, "$_POST", "", domain.Medium}}},
		{"src/Ctl.php", `\Sentry\captureMessage("no account for " . $customer->iban);`, []want{{domain.SinkErrorTracker, "$customer->iban", domain.CategoryFinancial, domain.Medium}}},
	} {
		got := scan(c.path, c.code)
		if len(got) != len(c.want) {
			t.Errorf("%s: %q: got %d findings, want %d: %+v", c.path, c.code, len(got), len(c.want), got)
			continue
		}
		for i, w := range c.want {
			g := got[i]
			if g.Kind != domain.KindSink || g.Detector != Detector || g.Sink != w.sink || g.Masked != w.masked || g.Category != w.category || g.Confidence != w.confidence {
				t.Errorf("%s: %q: got %s %q %s %s, want %s %q %s %s", c.path, c.code, g.Sink, g.Masked, g.Category, g.Confidence, w.sink, w.masked, w.category, w.confidence)
			}
			if g.Label != Labels[w.sink] || len(g.Reasons) != 2 {
				t.Errorf("%s: %q: label or reasons missing: %+v", c.path, c.code, g)
			}
		}
	}
}

func TestQuietCalls(t *testing.T) {
	for _, c := range []struct{ path, code string }{
		{"app/handler.go", `log.Printf("sending email to %d users", len(users))`},
		{"app/handler.go", `log.Printf("listening on %s", server.Addr)`},
		{"app/handler.go", `log.Printf("listening on %s", srv.address)`},
		{"app/handler.go", `log.Println("email sent", emailCount, u.ID)`},
		{"app/handler.go", `logger.Info("verified", "email_verified", u.EmailVerified)`},
		{"app/handler.go", `log.Printf("hash %s", hashEmail(u.Email))`},
		{"app/handler.go", `log.Printf("masked %s", mask(u.Phone))`},
		{"app/handler.go", `// log.Printf("%s", u.Email)`},
		{"app/handler.go", `func (l *Logger) Info(email string) {`},
		{"app/handler.go", `msg := "log.Printf(u.Email)"`},
		{"app/handler.go", `catalog.Info(u.Email)`},
		{"app/handler.go", `x.Printf("%s", u.Email)`},
		{"app/views.py", `logger.info("user %s logged in", user.id)`},
		{"app/views.py", `logger.info("template %s", EMAIL_TEMPLATE)`},
		{"app/views.py", `print(len(user.email))`},
		{"app/views.py", `# print(user.email)`},
		{"app/views.py", `def print(self, email):`},
		{"app/views.py", `logger.info("has email: %s", user.email is not None)`},
		{"web/app.ts", `console.log("Enter your email address")`},
		{"web/app.ts", `console.log(user.email === undefined)`},
		{"web/app.ts", `heap.push(user)`},
		{"web/app.ts", `this.$refs.group(user.email)`},
		{"web/app.ts", `console.log(!!user.email)`},
		{"src/Svc.java", `log.info("Mail server {}", mailConfig.getHost());`},
		{"src/Svc.java", `log.debug("processing {}", order.getId());`},
		{"app/models/user.rb", `puts "Done"`},
		{"app/models/user.rb", `p name`},
		{"src/Ctl.php", `$logger->info("cache age {$cache->age}");`},
		{"src/Ctl.php", `echo $user->email;`},
		{"docs/notes.txt", `log.Printf("%s", u.Email)`},
		{"app/views.py", `logger.info("file %s", file.name)`},
	} {
		if got := scan(c.path, c.code); len(got) != 0 {
			t.Errorf("%s: %q: want no findings, got %+v", c.path, c.code, got)
		}
	}
}

func TestOneFindingPerCategoryPerCall(t *testing.T) {
	got := scan("a.py", `logger.info("%s %s %s", user.email, user.phone, customer.first_name)`)
	if len(got) != 2 {
		t.Fatalf("want contact once and name once, got %+v", got)
	}
}

func TestMultiLineCallsEnd(t *testing.T) {
	got := scan("a.js", "logger.info(\n  'loaded',\n)\nconst email = user.email\nconsole.log(\n  email)\n")
	if len(got) != 1 || got[0].Location.Line != 6 || got[0].Location.Column != 3 {
		t.Fatalf("got %+v", got)
	}
}

func TestWords(t *testing.T) {
	if got := strings.Join(words("currentUserDTO_v2"), " "); got != "current user dto v" {
		t.Fatalf("got %q", got)
	}
	for name, want := range map[string]bool{"user": true, "users": true, "currentUser": true, "customer_data": true, "userId": false, "userService": false, "username": false, "customers": true, "loggedInUser": true} {
		if person(name) != want {
			t.Errorf("person(%q) = %v", name, !want)
		}
	}
}

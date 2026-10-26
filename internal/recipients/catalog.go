package recipients

// Purposes, as a record of processing names the reason data goes to a recipient.
const (
	Payments       = "payments"
	Email          = "email and messaging"
	Analytics      = "product analytics"
	ErrorTracking  = "error tracking and monitoring"
	Support        = "customer support and CRM"
	AI             = "AI model provider"
	Identity       = "sign-in and identity"
	Search         = "search"
	Media          = "media hosting"
	Notifications  = "push notifications"
	Advertising    = "advertising"
	CloudPlatform  = "cloud platform services"
	Collaboration  = "team messaging"
	Infrastructure = "infrastructure provider"
)

// catalog lists services that commonly receive personal data. EEA says whether the provider is established in
// the EEA; a provider outside it may still offer EU hosting, so the data map asks for a check, not a verdict.
// Packages are names as they appear in dependency files and imports; Hosts are API host names.
var catalog = []Recipient{
	{Name: "Stripe", Purpose: Payments, Packages: []string{"stripe", "@stripe/stripe-js", "github.com/stripe/stripe-go", "com.stripe:stripe-java", "com.stripe", "stripe/stripe-php"}, Hosts: []string{"api.stripe.com"}},
	{Name: "Adyen", Purpose: Payments, EEA: true, Packages: []string{"adyen", "@adyen/api-library", "@adyen/adyen-web", "github.com/adyen/adyen-go-api-library", "com.adyen:adyen-java-api-library", "com.adyen", "adyen/php-api-library"}, Hosts: []string{"adyen.com"}},
	{Name: "Mollie", Purpose: Payments, EEA: true, Packages: []string{"mollie-api-python", "@mollie/api-client", "mollie/mollie-api-php", "mollie-api-ruby"}, Hosts: []string{"api.mollie.com"}},
	{Name: "PayPal", Purpose: Payments, Packages: []string{"paypalrestsdk", "@paypal/checkout-server-sdk", "@paypal/paypal-js", "paypal/rest-api-sdk-php", "braintree"}, Hosts: []string{"api.paypal.com", "api-m.paypal.com", "api.braintreegateway.com"}},
	{Name: "Twilio", Purpose: Email, Packages: []string{"twilio", "github.com/twilio/twilio-go", "com.twilio.sdk:twilio", "com.twilio", "twilio/sdk"}, Hosts: []string{"api.twilio.com"}},
	{Name: "SendGrid", Purpose: Email, Packages: []string{"sendgrid", "@sendgrid/mail", "github.com/sendgrid/sendgrid-go", "com.sendgrid:sendgrid-java", "com.sendgrid", "sendgrid/sendgrid"}, Hosts: []string{"api.sendgrid.com"}},
	{Name: "Mailgun", Purpose: Email, Packages: []string{"mailgun", "mailgun.js", "mailgun-js", "github.com/mailgun/mailgun-go", "mailgun/mailgun-php"}, Hosts: []string{"api.mailgun.net", "api.eu.mailgun.net"}},
	{Name: "Mailchimp", Purpose: Email, Packages: []string{"mailchimp3", "mailchimp-marketing", "@mailchimp/mailchimp_marketing", "@mailchimp/mailchimp_transactional", "mailchimp-transactional"}, Hosts: []string{"api.mailchimp.com", "mandrillapp.com"}},
	{Name: "Postmark", Purpose: Email, Packages: []string{"postmark", "postmarker", "wildbit/postmark-php"}, Hosts: []string{"api.postmarkapp.com"}},
	{Name: "Brevo", Purpose: Email, EEA: true, Packages: []string{"sib-api-v3-sdk", "@getbrevo/brevo", "brevo-python", "getbrevo/brevo-php"}, Hosts: []string{"api.brevo.com", "api.sendinblue.com"}},
	{Name: "Amazon SES", Purpose: Email, Packages: []string{"@aws-sdk/client-ses", "@aws-sdk/client-sesv2", "github.com/aws/aws-sdk-go-v2/service/ses", "github.com/aws/aws-sdk-go-v2/service/sesv2"}, Hosts: []string{"email.amazonaws.com"}},
	{Name: "Segment", Purpose: Analytics, Packages: []string{"analytics-python", "segment-analytics-python", "@segment/analytics-node", "@segment/analytics-next", "analytics-node", "github.com/segmentio/analytics-go", "segmentio/analytics-php"}, Hosts: []string{"api.segment.io", "events.eu1.segmentapis.com"}},
	{Name: "Mixpanel", Purpose: Analytics, Packages: []string{"mixpanel", "mixpanel-browser", "github.com/mixpanel/mixpanel-go", "mixpanel/mixpanel-php"}, Hosts: []string{"api.mixpanel.com", "api-eu.mixpanel.com"}},
	{Name: "Amplitude", Purpose: Analytics, Packages: []string{"amplitude-analytics", "@amplitude/analytics-browser", "@amplitude/analytics-node", "github.com/amplitude/analytics-go"}, Hosts: []string{"api2.amplitude.com", "api.eu.amplitude.com"}},
	{Name: "Google Analytics", Purpose: Analytics, Packages: []string{"react-ga4", "react-ga", "@google-analytics/data", "google-analytics-data"}, Hosts: []string{"www.google-analytics.com", "region1.google-analytics.com", "www.googletagmanager.com"}},
	{Name: "PostHog", Purpose: Analytics, Packages: []string{"posthog", "posthog-js", "posthog-node", "github.com/posthog/posthog-go"}, Hosts: []string{"app.posthog.com", "us.i.posthog.com", "eu.i.posthog.com"}},
	{Name: "Hotjar", Purpose: Analytics, EEA: true, Packages: []string{"@hotjar/browser", "react-hotjar"}, Hosts: []string{"static.hotjar.com"}},
	{Name: "Sentry", Purpose: ErrorTracking, Packages: []string{"sentry-sdk", "@sentry/node", "@sentry/browser", "@sentry/react", "@sentry/nextjs", "github.com/getsentry/sentry-go", "io.sentry:sentry", "io.sentry", "sentry/sentry", "sentry/sentry-laravel", "sentry-ruby", "sentry-rails"}, Hosts: []string{"sentry.io", "ingest.sentry.io", "ingest.de.sentry.io"}},
	{Name: "Datadog", Purpose: ErrorTracking, Packages: []string{"ddtrace", "datadog", "dd-trace", "@datadog/browser-rum", "@datadog/browser-logs", "gopkg.in/DataDog/dd-trace-go.v1", "github.com/DataDog/dd-trace-go", "datadog/dd-trace"}, Hosts: []string{"datadoghq.com", "datadoghq.eu"}},
	{Name: "New Relic", Purpose: ErrorTracking, Packages: []string{"newrelic", "github.com/newrelic/go-agent", "com.newrelic.agent.java:newrelic-agent", "com.newrelic"}, Hosts: []string{"newrelic.com", "eu01.nr-data.net"}},
	{Name: "Bugsnag", Purpose: ErrorTracking, Packages: []string{"bugsnag", "@bugsnag/js", "@bugsnag/node", "github.com/bugsnag/bugsnag-go", "bugsnag/bugsnag-laravel"}, Hosts: []string{"notify.bugsnag.com"}},
	{Name: "Rollbar", Purpose: ErrorTracking, Packages: []string{"rollbar", "github.com/rollbar/rollbar-go", "rollbar/rollbar"}, Hosts: []string{"api.rollbar.com"}},
	{Name: "Intercom", Purpose: Support, Packages: []string{"python-intercom", "intercom-client", "@intercom/messenger-js-sdk", "intercom/intercom-php"}, Hosts: []string{"api.intercom.io", "api.eu.intercom.io"}},
	{Name: "Zendesk", Purpose: Support, Packages: []string{"zenpy", "node-zendesk", "zendesk_api", "zendesk/zendesk_api_client_php"}, Hosts: []string{"zendesk.com"}},
	{Name: "HubSpot", Purpose: Support, Packages: []string{"hubspot-api-client", "@hubspot/api-client", "hubspot/api-client"}, Hosts: []string{"api.hubapi.com"}},
	{Name: "Salesforce", Purpose: Support, Packages: []string{"simple-salesforce", "jsforce", "restforce"}, Hosts: []string{"salesforce.com", "force.com"}},
	{Name: "OpenAI", Purpose: AI, Packages: []string{"openai", "@openai/agents", "github.com/openai/openai-go", "github.com/sashabaranov/go-openai", "openai-php/client", "langchain-openai", "@langchain/openai"}, Hosts: []string{"api.openai.com"}},
	{Name: "Anthropic", Purpose: AI, Packages: []string{"anthropic", "@anthropic-ai/sdk", "github.com/anthropics/anthropic-sdk-go", "langchain-anthropic", "@langchain/anthropic"}, Hosts: []string{"api.anthropic.com"}},
	{Name: "Google Gemini", Purpose: AI, Packages: []string{"google-generativeai", "google-genai", "@google/generative-ai", "@google/genai", "google.golang.org/genai"}, Hosts: []string{"generativelanguage.googleapis.com"}},
	{Name: "Mistral AI", Purpose: AI, EEA: true, Packages: []string{"mistralai", "@mistralai/mistralai", "langchain-mistralai"}, Hosts: []string{"api.mistral.ai"}},
	{Name: "Cohere", Purpose: AI, Packages: []string{"cohere", "cohere-ai", "langchain-cohere"}, Hosts: []string{"api.cohere.com", "api.cohere.ai"}},
	{Name: "Firebase", Purpose: CloudPlatform, Packages: []string{"firebase", "firebase-admin", "firebase.google.com/go", "firebase.google.com/go/v4", "kreait/firebase-php"}, Hosts: []string{"firebaseio.com", "fcm.googleapis.com"}},
	{Name: "Auth0", Purpose: Identity, Packages: []string{"auth0-python", "auth0", "@auth0/nextjs-auth0", "@auth0/auth0-react", "@auth0/auth0-spa-js", "github.com/auth0/go-auth0", "auth0/auth0-php"}, Hosts: []string{"auth0.com"}},
	{Name: "Okta", Purpose: Identity, Packages: []string{"okta", "@okta/okta-sdk-nodejs", "@okta/okta-auth-js", "github.com/okta/okta-sdk-golang"}, Hosts: []string{"okta.com"}},
	{Name: "Algolia", Purpose: Search, Packages: []string{"algoliasearch", "algolia/algoliasearch-client-php", "github.com/algolia/algoliasearch-client-go"}, Hosts: []string{"algolia.net", "algolianet.com"}},
	{Name: "Cloudinary", Purpose: Media, Packages: []string{"cloudinary", "cloudinary/cloudinary_php", "github.com/cloudinary/cloudinary-go"}, Hosts: []string{"api.cloudinary.com"}},
	{Name: "OneSignal", Purpose: Notifications, Packages: []string{"onesignal-sdk", "@onesignal/node-onesignal", "react-onesignal", "onesignal/onesignal-php-api"}, Hosts: []string{"onesignal.com"}},
	{Name: "Meta Pixel and Conversions API", Purpose: Advertising, Packages: []string{"facebook_business", "facebook-nodejs-business-sdk", "facebook/php-business-sdk", "react-facebook-pixel"}, Hosts: []string{"graph.facebook.com", "connect.facebook.net"}},
	{Name: "Slack", Purpose: Collaboration, Packages: []string{"slack-sdk", "slack_sdk", "@slack/web-api", "@slack/bolt", "github.com/slack-go/slack"}, Hosts: []string{"hooks.slack.com", "slack.com"}},
}

package sinks

import (
	"path"
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

// language is what the scanner needs to know about a source language: its comments and its sink calls.
type language struct {
	name          string
	slashComments bool
	hashComments  bool
	rules         []rule
}

// rule recognises the start of a sink call. The match ends where the arguments begin: just after the opening
// bracket, or after the space of a call written without brackets. A line must hold one of words, in lower
// case, before the pattern runs.
type rule struct {
	sink  domain.Sink
	words []string
	re    *regexp.Regexp
}

func newRule(sink domain.Sink, words string, re string) rule {
	return rule{sink: sink, words: strings.Split(words, " "), re: regexp.MustCompile(re)}
}

// admits reports whether a lower-cased line holds one of the rule's words.
func (r rule) admits(lower string) bool {
	for _, w := range r.words {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

const (
	trackerWords   = "sentry raven bugsnag rollbar honeybadger airbrake newrelic crashlytics raygun appsignal datadogrum"
	analyticsWords = "analytics mixpanel amplitude posthog fullstory braze appboy klaviyo customerio countly clevertap appsflyer kissmetrics woopra heap fs umami rudder"
	logWords       = "log console zap sugar pino winston bunyan timber"
)

// chain is a receiver path such as this.logger., $this->logger->, Rails.logger. or Sentry::.
const chain = `(?:[\w$@\\]+(?:\(\))?\s*(?:\.|\?\.|->|::|\\)\s*)*`

// Log calls: a logger-like receiver and a level or print method.
const logReceiver = `(?:[$@]?_*(?i:log|logger|logging|slog|zap|zlog|zerolog|logrus|sugar|log4j|winston|pino|bunyan|console|timber)|[$@]?\w*(?:Logger|Log|_logger|_log))`

const logMethod = `(?i:debug|info|warn|warning|error|err|fatal|critical|crit|exception|trace|notice|alert|emergency|emerg|log|print|printf|println|verbose|silly|panic|dir|table|severe|fine|finer|finest|d|i|w|e|v|wtf|` +
	`debugf|infof|warnf|errorf|fatalf|panicf|tracef|debugw|infow|warnw|errorw|fatalw|panicw|debugln|infoln|warnln|errorln|fatalln|panicln|` +
	`debugcontext|infocontext|warncontext|errorcontext|logattrs|with|withfield|withfields)`

const trackerName = `(?i:sentry|sentry_sdk|raven|bugsnag|rollbar|honeybadger|airbrake|newrelic|crashlytics|raygun|appsignal|datadogrum)`

const trackerMethod = `(?i:capture_?exception|capture_?message|capture_?event|set_?user|set_?context|set_?extras?|set_?tags?|add_?breadcrumb|leave_?breadcrumb|` +
	`notify|notify_?exception|notify_?error|notice_?error|error|critical|warning|warn|info|log|report|record_?exception|set_?custom_?key|set_?user_?id|` +
	`add_?custom_?attributes?|set_?custom_?data|set_?custom_?attribute|context|set_?context|add_?metadata|add_?action|set_?global_?context_?property)`

// analyticsName allows a prefix (myAnalytics) only for names too long to be part of an unrelated word.
const analyticsName = `(?:\w*(?i:analytics|mixpanel|amplitude|posthog|fullstory|braze|appboy|klaviyo|customerio|countly|clevertap|appsflyer|kissmetrics|woopra)|(?i:heap|fs|umami|rudder)\b)`

const analyticsMethod = `(?i:track|identify|page|screen|group|alias|capture|people\.set(?:_once)?|set_?user_?properties|set_?user_?id|log_?event|add_?user_?properties|set_?user_?vars|record|track_?event|record_?event|set_?profile|profile_?set|on_?user_?login|enqueue|register)`

// common rules apply in every language. Error trackers and analytics come first, so a call that matches both
// a tracker and a log rule is reported as the more specific sink.
var common = []rule{
	newRule(domain.SinkErrorTracker, trackerWords, `(?:^|[^\w$@.>:])[$@]?`+chain+`[\w$]*`+trackerName+`\w*(?:\(\))?\s*`+`(?:(?:\.|->|::|\\)\s*[\w$]+(?:\(\))?\s*)*?(?:\.|->|::|\\)\s*`+trackerMethod+`\s*\(`),
	newRule(domain.SinkErrorTracker, "scope hub", `(?:^|[^\w$@.>:])(?i:scope|hub)\s*(?:\.|->)\s*(?i:set_?user|set_?extras?|set_?context|set_?tags?)\s*\(`),
	newRule(domain.SinkAnalytics, analyticsWords, `(?:^|[^\w$@.>:])[$@]?`+chain+analyticsName+`(?:\(\))?\s*(?:\.|->|::)\s*`+analyticsMethod+`\s*\(`),
	newRule(domain.SinkAnalytics, "gtag plausible intercom datalayer _paq _kmq", `(?:^|[^\w$@.>:])(?:gtag|plausible|Intercom|dataLayer\.push|_paq\.push|_kmq\.push)\s*\(`),
	newRule(domain.SinkLog, logWords, `(?:^|[^\w$@.>:])`+chain+logReceiver+`\s*(?:\.|\?\.|->|::)\s*`+logMethod+`\s*\(`),
}

var languages = map[string]language{}

func init() {
	add := func(name string, slash, hash bool, exts []string, extra ...rule) {
		l := language{name: name, slashComments: slash, hashComments: hash, rules: append(append([]rule{}, common...), extra...)}
		for _, e := range exts {
			languages[e] = l
		}
	}
	log := func(words, re string) rule { return newRule(domain.SinkLog, words, re) }
	add("go", true, false, []string{".go"},
		log("fmt.print", `(?:^|[^\w.])fmt\.Print(?:f|ln)?\s*\(`),
		log("fmt.fprint", `(?:^|[^\w.])fmt\.Fprint(?:f|ln)?\s*\(\s*os\.Std(?:out|err)\s*,`),
		newRule(domain.SinkAnalytics, "analytics.", `(?:^|[^\w.])analytics\.(?:Track|Identify|Page|Group|Alias|Screen)\s*\{`),
	)
	add("python", false, true, []string{".py", ".pyw"},
		log("print", `(?:^|[^\w.])p?print\s*\(`),
		log("sys.std", `(?:^|[^\w.])sys\.std(?:out|err)\.write\s*\(`),
	)
	add("js", true, false, []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts"},
		log("process.std", `(?:^|[^\w.])process\.std(?:out|err)\.write\s*\(`),
	)
	add("java", true, false, []string{".java"},
		log("system.", `(?:^|[^\w.])System\.(?:out|err)\.print(?:ln|f)?\s*\(`),
	)
	add("kotlin", true, false, []string{".kt", ".kts"},
		log("system.", `(?:^|[^\w.])System\.(?:out|err)\.print(?:ln|f)?\s*\(`),
		log("print", `(?:^|[^\w.])print(?:ln)?\s*\(`),
		log(logWords, `(?:^|[^\w$@.>:])`+chain+logReceiver+`\s*(?:\.|\?\.)\s*`+logMethod+`\s*\{`),
	)
	add("ruby", false, true, []string{".rb", ".rake"},
		log("puts print p pp warn", `^\s*(?:puts|print|p|pp|warn)(?:\s*\(|\s+)`),
		log(logWords, `^\s*`+chain+logReceiver+`\s*\.\s*`+logMethod+`\s+`),
	)
	add("php", true, true, []string{".php"},
		log("error_log var_dump print_r var_export syslog dump dd logger", `(?:^|[^\w$>:])(?:error_log|var_dump|print_r|var_export|syslog|dump|dd|logger)\s*\(`),
	)
}

// languageOf picks the language from a file's extension. ok is false for files that are not source code.
func languageOf(p string) (language, bool) {
	l, ok := languages[strings.ToLower(path.Ext(p))]
	return l, ok
}

// definition reports whether a match is the definition of a function or method of that name, not a call.
var definition = regexp.MustCompile(`(?:\b(?:def|function|func|fun|void|public|private|protected|static)\s+|\)\s*)$`)

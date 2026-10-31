package lifecycle

import (
	"path"
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Engines of the stores that expire data on their own once the code tells them to. Evidence with an engine can
// also give a Terraform store of that kind a retention limit.
const (
	DynamoDB = "dynamodb"
	Redis    = "redis"
	Search   = "search"
)

// keyStart is the start of a literal Redis key: a quote, optionally after a Python string prefix, and the key's
// first word, which names what the key holds ("session:" + id names session).
const keyStart = "[fbr]?[\"'`]([A-Za-z_][\\w-]*)"

// redisRules recognise Redis commands that give a key a time to live, in redis-py, ioredis, node-redis,
// go-redis, Jedis, Spring Data Redis, Predis and phpredis. Only keys written as literals name a store.
var redisRules = []rule{
	{kind: Retention, how: "Redis key expiry", re: regexp.MustCompile(`(?:\.|->|::)\s*(?i:setex|psetex|set_?ex|getex|expire|pexpire|expireat|pexpireat)\s*\(\s*(?:ctx\s*,\s*|context\.\w+\(\)\s*,\s*)?` + keyStart), group: 1},
	{kind: Retention, how: "Redis SET with an expiry", re: regexp.MustCompile(`(?:\.|->)\s*(?i:set)\s*\(\s*` + keyStart), group: 1,
		onlyIf: regexp.MustCompile(`["'](?i:ex|px|exat|pxat)["']\s*(?:,|=>)|\b(?:ex|px|exat|pxat)\s*=\s*[^=\sN]|\{\s*(?:EX|PX|EXAT|PXAT)\s*:|SetParams|Duration\.of|TimeUnit\.`)},
	{kind: Retention, how: "Redis SET with an expiry", re: regexp.MustCompile(`\.Set(?:NX|XX)?\(\s*\w+\s*,\s*["` + "`" + `]([A-Za-z_][\w-]*)`), group: 1,
		onlyIf: regexp.MustCompile(`,\s*(?:\d+\s*\*\s*)?time\.(?:Second|Minute|Hour)|,\s*\w*(?i:ttl|expir|lifetime)\w*\s*\)`)},
}

// redisWords must appear in a file before the Redis rules run, so a set or expire method of another library
// does not count.
var redisWords = []string{"redis", "rdb", "jedis", "valkey", "predis"}

var (
	ttlCall    = regexp.MustCompile(`update_time_to_live\s*\(|UpdateTimeToLive(?:Command|Request|Input)?\b|updateTimeToLive\s*\(|timeToLiveAttribute\s*:`)
	tableName  = regexp.MustCompile("(?i)table_?name[\"']?\\s*[:=(]\\s*(?:aws\\.String\\(\\s*)?[\"'`]([\\w.-]+)[\"'`]")
	ttlOff     = regexp.MustCompile(`(?i)["']?enabled["']?\s*[:=(]\s*(?:aws\.Bool\(\s*)?false`)
	policyName = regexp.MustCompile(`(?:_ilm/policy|_plugins/_ism/policies|_opendistro/_ism/policies)/([\w-]+)|(?i:put_?lifecycle|put_?policy)\s*\(\s*\{?\s*(?:name|policy_?id|id)["']?\s*[:=]\s*["']([\w-]+)["']`)
	indexPat   = regexp.MustCompile(`["']?index_patterns["']?\s*[:=]\s*\[\s*["']([\w-]+)`)
	deletes    = regexp.MustCompile(`["']?\bdelete["']?\s*[:=]\s*\{`)
	policyKeys = regexp.MustCompile(`["']?\b(?:phases|states)["']?\s*[:=]`)
)

// window is how far after a call its arguments are read, in bytes, so arguments on the next lines count.
const window = 600

// findExpiry returns the store-level expiry in a file: DynamoDB time to live switched on through the SDK or
// the CDK, Redis keys written with a time to live, and Elasticsearch or OpenSearch index policies with a
// delete phase or action. JSON files are read only for index policies.
func findExpiry(p, text string) []Evidence {
	lower := strings.ToLower(text)
	var out []Evidence
	if strings.ToLower(path.Ext(p)) == ".json" {
		if strings.Contains(lower, `"policy"`) && policyKeys.MatchString(text) && deletes.MatchString(text) {
			target := ""
			if m := indexPat.FindStringSubmatch(text); m != nil {
				target = m[1]
			} else {
				target = strings.TrimSuffix(path.Base(p), path.Ext(p))
			}
			at := deletes.FindStringIndex(text)[0]
			out = append(out, Evidence{Kind: Retention, Target: policyTarget(target), How: "index policy with a delete phase", Engine: Search, Location: locate(p, text, at)})
		}
		return out
	}
	if strings.Contains(lower, "timetolive") || strings.Contains(lower, "time_to_live") {
		for _, m := range ttlCall.FindAllStringIndex(text, -1) {
			args := text[m[0]:min(len(text), m[1]+window)]
			if next := ttlCall.FindStringIndex(args[m[1]-m[0]:]); next != nil {
				args = args[:m[1]-m[0]+next[0]]
			}
			start := m[0]
			if strings.HasPrefix(text[m[0]:], "timeToLiveAttribute") {
				// a CDK table names itself before its time to live attribute: read back to the table's start.
				start = max(0, m[0]-window)
				if i := strings.LastIndex(text[start:m[0]], "Table("); i >= 0 {
					start += i
				}
				args = text[start:m[1]]
			}
			t := tableName.FindStringSubmatchIndex(args)
			if t == nil || ttlOff.MatchString(args) {
				continue
			}
			out = append(out, Evidence{Kind: Retention, Target: args[t[2]:t[3]], How: "DynamoDB time to live", Engine: DynamoDB, Location: locate(p, text, start+t[2])})
		}
	}
	if policyKeys.MatchString(text) {
		for _, m := range policyName.FindAllStringSubmatchIndex(text, -1) {
			g := 2
			if m[g] < 0 {
				g = 4
			}
			body := text[m[1]:min(len(text), m[1]+4*window)]
			if !deletes.MatchString(body) || !policyKeys.MatchString(body) {
				continue
			}
			target := text[m[g]:m[g+1]]
			if ip := indexPat.FindStringSubmatch(body); ip != nil {
				target = ip[1]
			}
			out = append(out, Evidence{Kind: Retention, Target: policyTarget(target), How: "index policy with a delete phase", Engine: Search, Location: locate(p, text, m[g])})
		}
	}
	redis := false
	for _, w := range redisWords {
		redis = redis || strings.Contains(lower, w)
	}
	if !redis {
		return out
	}
	for i, l := range strings.Split(text, "\n") {
		for _, r := range redisRules {
			if r.onlyIf != nil && !r.onlyIf.MatchString(l) {
				continue
			}
			if m := r.re.FindStringSubmatchIndex(l); m != nil {
				out = append(out, Evidence{Kind: Retention, Target: l[m[2]:m[3]], How: r.how, Engine: Redis, Location: domain.Location{Path: p, Line: i + 1, Column: m[2] + 1}})
				break
			}
		}
	}
	return out
}

// policyTarget turns a policy name or index pattern into the name of what it keeps: "audit-logs-policy" and
// "audit-logs-*" both give audit-logs.
func policyTarget(name string) string {
	name = strings.TrimRight(name, "-_*")
	for _, suffix := range []string{"policy", "ilm", "ism", "retention", "lifecycle"} {
		for _, sep := range []string{"-", "_"} {
			name = strings.TrimSuffix(name, sep+suffix)
		}
	}
	return name
}

// locate turns a byte offset into a location.
func locate(p, text string, at int) domain.Location {
	line := strings.Count(text[:at], "\n") + 1
	col := at - strings.LastIndex(text[:at], "\n")
	return domain.Location{Path: p, Line: line, Column: col}
}

// FindExpiry returns only the store-level expiry in a file, the evidence a Terraform store can use. It is
// cheaper than Find, for scans that read stores but do not assess the lifecycle of tables.
func FindExpiry(p string, data []byte) []Evidence {
	if !Candidate(p) {
		return nil
	}
	return dedupe(findExpiry(p, strings.ReplaceAll(string(data), "\r\n", "\n")))
}

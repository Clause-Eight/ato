package infra

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

// A module call can create stores that pii-eu cannot see: registry and git modules are not downloaded, and a
// local module outside the scanned files is not read. Such a call is listed as a store of type "module" with
// every setting unknown, when its source or its arguments say it holds data. A local module whose folder was
// scanned is not listed: its own resources are read as stores of that folder.

// settingWords sort the arguments of a module call by the setting they look like. An argument that matches
// none is not a store setting.
var settingWords = []struct {
	setting string
	words   []string
}{
	{"encryption", []string{"encrypt", "kms", "sse", "cmk"}},
	{"backup", []string{"backup", "versioning", "point_in_time", "snapshot", "pitr"}},
	{"retention", []string{"retention", "lifecycle", "ttl", "expir"}},
	{"public", []string{"public", "acl"}},
}

// storeArgs are arguments that only data stores take, beyond the settings above.
var storeArgs = map[string]bool{
	"engine": true, "engine_version": true, "allocated_storage": true, "storage_type": true, "hash_key": true,
	"range_key": true, "billing_mode": true, "bucket": true, "bucket_name": true, "table_name": true, "db_name": true,
	"database_name": true, "queue_name": true, "log_group_name": true, "multi_az": true, "deletion_protection": true,
}

// moduleNames are the arguments that usually carry the store's name in the cloud.
var moduleNames = []string{"name", "identifier", "bucket", "bucket_name", "table_name", "db_name", "database_name", "cluster_identifier", "queue_name", "topic_name", "stream_name", "log_group_name", "replication_group_id", "cluster_id", "domain_name"}

// serviceWords map words of a module's source or label to the kind of store it creates, first match wins.
var serviceWords = []struct {
	service string
	words   []string
}{
	{"key-value", []string{"dynamodb", "firestore", "bigtable", "datastore"}},
	{"warehouse", []string{"redshift", "bigquery", "synapse", "snowflake"}},
	{"cache", []string{"elasticache", "redis", "memcached", "memorystore", "valkey"}},
	{"search", []string{"opensearch", "elasticsearch"}},
	{"queue", []string{"sqs", "queue", "servicebus"}},
	{"topic", []string{"sns", "pubsub", "topic"}},
	{"stream", []string{"kinesis", "msk", "kafka", "eventhub", "firehose"}},
	{"file-storage", []string{"efs", "fsx", "filestore"}},
	{"block-storage", []string{"ebs", "disk"}},
	{"object-storage", []string{"s3", "bucket", "gcs", "blob", "storage"}},
	{"log", []string{"cloudwatch", "logs", "logging", "loggroup"}},
	{"database", []string{"rds", "aurora", "postgres", "postgresql", "mysql", "mariadb", "mssql", "sql", "cloudsql", "database", "db", "docdb", "documentdb", "neptune", "cosmosdb", "spanner", "mongodb", "dynamo"}},
}

var providerWords = map[string]string{"aws": "aws", "google": "google", "gcp": "google", "azurerm": "azurerm", "azure": "azurerm"}

func isLocalSource(src string) bool {
	return strings.HasPrefix(src, "./") || strings.HasPrefix(src, "../")
}

// moduleStore turns a module call into a store, or reports false when the call is a local module that was
// read, or nothing about it points to a data store.
func moduleStore(m *module, call *resource, scanned map[string]bool) (domain.Store, bool) {
	src := ""
	if a := call.body.attrs["source"]; a != nil {
		if v := m.eval(a.val); v.known && v.obj == nil && v.list == nil {
			src = v.s
		}
	}
	if src == "" {
		return domain.Store{}, false
	}
	if isLocalSource(src) && scanned[path.Clean(path.Join(path.Dir(call.file.Path), src))] {
		return domain.Store{}, false
	}
	service := serviceOf(sourceWords(src))
	if service == "" {
		service = serviceOf(splitWords(call.name))
	}
	settings := moduleSettings(call)
	if service == "" && len(settings) == 0 {
		return domain.Store{}, false
	}
	if service == "" {
		service = "unknown"
	}
	s := domain.Store{
		Type: domain.ModuleType, Name: call.name, Provider: providerOf(m, src), Service: service, Module: src,
		Location: domain.Location{Path: call.file.Path, Line: call.body.line, Column: 1},
	}
	for _, x := range settings {
		s.ModuleSettings = append(s.ModuleSettings, x.name)
	}
	c := &check{m: m, r: call, s: &s}
	s.Names = c.names(moduleNames)
	s.Tags = c.tags()
	s.PersonalData, s.PersonalWhy = personalTags(s.Tags)
	inside := fmt.Sprintf("decided inside module %q (%s), which pii-eu does not read", call.name, src)
	unknown := func(setting, label string) domain.Setting {
		set := domain.Setting{State: domain.Unknown, Why: label + " is " + inside}
		var passed []string
		for _, x := range settings {
			if x.setting == setting {
				passed = append(passed, x.text)
				if set.Line == 0 {
					set.Line = x.line
				}
			}
		}
		if len(passed) > 0 {
			set.Why += "; the call passes " + strings.Join(passed, ", ")
		}
		return set
	}
	s.Encryption = unknown("encryption", "encryption at rest")
	s.Retention = unknown("retention", "retention")
	s.Backup = unknown("backup", "backup")
	s.Public = unknown("public", "public access")
	s.InEEA = setting(domain.Unknown, "the region is "+inside)
	return s, true
}

type moduleArg struct {
	name, setting, text string
	line                int
}

// moduleSettings lists the arguments of a call that look like store settings, sorted by name, with their
// value when it can be read.
func moduleSettings(call *resource) []moduleArg {
	var out []moduleArg
	for name, a := range call.body.attrs {
		key := strings.ToLower(name)
		set := settingOf(key)
		if set == "" && !storeArgs[key] {
			continue
		}
		text := name
		if a.val != nil && a.val.kind == eLiteral && !a.val.str {
			text += " = " + a.val.lit
		}
		out = append(out, moduleArg{name: name, setting: set, text: text, line: a.line})
	}
	for _, b := range call.body.blocks {
		if set := settingOf(strings.ToLower(b.typ)); set != "" {
			out = append(out, moduleArg{name: b.typ, setting: set, text: b.typ + " block", line: b.line})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// settingOf names the setting an argument looks like: a word of the argument starts with one of the setting's
// words, or the argument contains a word written with underscores.
func settingOf(key string) string {
	words := strings.Split(key, "_")
	for _, sw := range settingWords {
		for _, w := range sw.words {
			if strings.Contains(w, "_") {
				if strings.Contains(key, w) {
					return sw.setting
				}
				continue
			}
			for _, x := range words {
				if strings.HasPrefix(x, w) {
					return sw.setting
				}
			}
		}
	}
	return ""
}

// sourceWords are the words of the part of a module source that names it: the module name of a registry
// address (namespace/name/provider), or the last folder of a path or URL, without ref and subfolder.
func sourceWords(src string) []string {
	s := src
	if i := strings.Index(s, "?"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, ".git")
	parts := strings.Split(strings.Trim(strings.ReplaceAll(s, "//", "/"), "/"), "/")
	if len(parts) >= 3 && providerWords[parts[len(parts)-1]] != "" && !strings.Contains(src, ":") {
		return append(splitWords(parts[len(parts)-2]), parts[len(parts)-1])
	}
	return splitWords(parts[len(parts)-1])
}

func splitWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
}

func serviceOf(words []string) string {
	for _, sw := range serviceWords {
		for _, w := range sw.words {
			for _, x := range words {
				if x == w {
					return sw.service
				}
			}
		}
	}
	return ""
}

// providerOf reads the provider from the module's source, or else from the folder when it uses only one
// provider; "unknown" otherwise.
func providerOf(m *module, src string) string {
	for _, w := range splitWords(src) {
		if p := providerWords[w]; p != "" {
			return p
		}
	}
	seen := map[string]bool{}
	for key := range m.providers {
		if p := providerWords[strings.SplitN(key, ".", 2)[0]]; p != "" {
			seen[p] = true
		}
	}
	for _, r := range m.order {
		if p := providerWords[strings.SplitN(r.typ, "_", 2)[0]]; p != "" {
			seen[p] = true
		}
	}
	if len(seen) == 1 {
		for p := range seen {
			return p
		}
	}
	return "unknown"
}

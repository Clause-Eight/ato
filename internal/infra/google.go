package infra

import (
	"fmt"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Google Cloud encrypts all data at rest; the settings that vary are location, expiry and public grants.

const googleEncrypted = "Google Cloud encrypts all data at rest"

func google(typ, service string, names []string, region func(c *check) ([]string, domain.Setting), assess func(c *check)) {
	register(typ, spec{provider: "google", service: service, names: names, region: region, assess: func(c *check) {
		c.s.Encryption = setting(domain.Yes, googleEncrypted)
		if c.r.body.attrs["encryption_key_name"] != nil || len(blocksNamed(c.r.body, "encryption")) > 0 || len(blocksNamed(c.r.body, "default_encryption_configuration")) > 0 {
			c.s.Encryption.Why += ", here with a customer-managed key"
		}
		assess(c)
	}})
}

func locationAttr(c *check) ([]string, domain.Setting) { return c.regionAttr("location", nil) }

func init() {
	google("google_sql_database_instance", "database", []string{"name"}, func(c *check) ([]string, domain.Setting) {
		return c.regionAttr("region", c.providerRegion)
	}, func(c *check) {
		c.s.Backup = c.flag(c.r, "settings.backup_configuration.enabled", domain.Yes, domain.No, setting(domain.No, "settings.backup_configuration is not enabled"))
		c.s.Public = c.sqlPublic()
	})
	google("google_storage_bucket", "object-storage", []string{"name"}, locationAttr, func(c *check) {
		c.s.Retention = c.gcsRetention()
		c.s.Backup = c.flag(c.r, "versioning.enabled", domain.Yes, domain.No, setting(domain.No, "versioning is not enabled"))
		c.s.Public = c.publicGrant([]string{"google_storage_bucket_iam_member", "google_storage_bucket_iam_binding", "google_storage_bucket_access_control"}, "bucket")
		if c.s.Public.State != domain.Yes {
			if s, a, ok := c.text(c.r, "public_access_prevention"); a != nil && ok && s == "enforced" {
				c.s.Public = domain.Setting{State: domain.No, Why: `public_access_prevention = "enforced"`, Line: a.line}
			}
		}
	})
	google("google_bigquery_dataset", "warehouse", []string{"dataset_id"}, func(c *check) ([]string, domain.Setting) {
		return c.regionAttr("location", func() ([]string, domain.Setting) {
			return []string{"US"}, setting(domain.NotApplicable, "location is not set and BigQuery defaults to US")
		})
	}, func(c *check) {
		c.s.Retention = c.firstPositive([]string{"default_table_expiration_ms", "default_partition_expiration_ms"}, setting(domain.No, "no default table or partition expiration, so tables are kept forever"))
		c.s.Public = c.datasetPublic()
	})
	google("google_bigquery_table", "warehouse", []string{"table_id"}, func(c *check) ([]string, domain.Setting) {
		ds := c.dataset()
		if ds == nil {
			return nil, setting(domain.Unknown, "the dataset is not declared in this folder")
		}
		if a := ds.body.attrs["location"]; a != nil {
			regions, found := c.regionFrom(ds, a, "location")
			if found.State != domain.Unknown {
				found.Why = "from " + ds.address()
			}
			return regions, found
		}
		return []string{"US"}, setting(domain.NotApplicable, ds.address()+" sets no location and BigQuery defaults to US")
	}, func(c *check) {
		inherited := setting(domain.No, "no expiration_time or partition expiration, so the table is kept forever")
		if ds := c.dataset(); ds != nil {
			if a := ds.body.attrs["default_table_expiration_ms"]; a != nil {
				inherited = c.number(ds, "default_table_expiration_ms", "", inherited)
			}
		}
		c.s.Retention = c.firstPositive([]string{"expiration_time", "time_partitioning.expiration_ms"}, inherited)
	})
	google("google_firestore_database", "database", []string{"name"}, func(c *check) ([]string, domain.Setting) {
		return c.regionAttr("location_id", nil)
	}, func(c *check) {
		c.s.Retention = c.firestoreTTL()
		c.s.Backup = c.enumSetting("point_in_time_recovery_enablement", map[string]domain.Tri{"POINT_IN_TIME_RECOVERY_ENABLED": domain.Yes, "POINT_IN_TIME_RECOVERY_DISABLED": domain.No}, setting(domain.No, "point_in_time_recovery_enablement is not set; it is off by default"))
		if c.s.Backup.State != domain.Yes {
			if bs := c.related("google_firestore_backup_schedule", "database"); len(bs) > 0 {
				c.s.Backup = domain.Setting{State: domain.Yes, Why: c.where(bs[0], "a backup schedule backs up the database"), Line: c.on(bs[0], bs[0].body.line)}
			}
		}
	})
	google("google_redis_instance", "cache", []string{"name"}, func(c *check) ([]string, domain.Setting) {
		return c.regionAttr("region", c.providerRegion)
	}, func(c *check) {
		c.s.Public = setting(domain.No, "Memorystore instances have only private addresses in their VPC network")
		mode, a, ok := c.text(c.r, "persistence_config.persistence_mode")
		switch {
		case a == nil:
			c.s.Backup = setting(domain.No, "persistence_config is not set, so there are no snapshots")
		case !ok:
			c.s.Backup = domain.Setting{State: domain.Unknown, Why: unresolved("persistence_config.persistence_mode", a), Line: a.line}
		case strings.EqualFold(mode, "RDB"):
			c.s.Backup = domain.Setting{State: domain.Yes, Why: `persistence_config.persistence_mode = "RDB"`, Line: a.line}
		default:
			c.s.Backup = domain.Setting{State: domain.No, Why: fmt.Sprintf("persistence_config.persistence_mode = %q", mode), Line: a.line}
		}
	})
	google("google_spanner_instance", "database", []string{"name"}, func(c *check) ([]string, domain.Setting) {
		regions, found := c.regionAttr("config", nil)
		for i, r := range regions {
			regions[i] = strings.TrimPrefix(r[strings.LastIndex(r, "/")+1:], "regional-")
		}
		return regions, found
	}, func(c *check) {})
	google("google_pubsub_topic", "topic", []string{"name"}, func(c *check) ([]string, domain.Setting) {
		a, dynamic := lookup(c.r.body, "message_storage_policy.allowed_persistence_regions")
		if a == nil {
			why := "no message_storage_policy, so Pub/Sub may store messages in any region"
			if dynamic {
				why = "message_storage_policy comes from a dynamic block"
			}
			return nil, setting(domain.Unknown, why)
		}
		return c.regionFrom(c.r, a, "message_storage_policy.allowed_persistence_regions")
	}, func(c *check) {
		c.s.Retention = setting(domain.Yes, "Pub/Sub deletes messages after the retention duration (31 days at most)")
	})
	google("google_logging_project_bucket_config", "log", []string{"bucket_id"}, func(c *check) ([]string, domain.Setting) {
		regions, found := c.regionAttr("location", nil)
		if len(regions) == 1 && strings.EqualFold(regions[0], "global") {
			return nil, domain.Setting{State: domain.Unknown, Why: `location = "global" does not fix a region`, Line: found.Line}
		}
		return regions, found
	}, func(c *check) {
		c.s.Retention = c.number(c.r, "retention_days", "days", setting(domain.Yes, "retention_days is not set; the default is 30 days"))
	})
}

// firestoreTTL looks for a field with a TTL policy in this database: Firestore deletes a document some time after
// the timestamp in that field has passed. A field without a database argument belongs to "(default)".
func (c *check) firestoreTTL() domain.Setting {
	fields := c.related("google_firestore_field", "database")
	if contains(c.s.Names, "(default)") {
		for _, r := range c.m.order {
			if r.typ == "google_firestore_field" && r.body.attrs["database"] == nil {
				fields = append(fields, r)
			}
		}
	}
	for _, f := range fields {
		if ttl := blocksNamed(f.body, "ttl_config"); len(ttl) > 0 {
			coll, _, _ := c.text(f, "collection")
			field, _, _ := c.text(f, "field")
			return domain.Setting{State: domain.Yes, Why: c.where(f, fmt.Sprintf("a TTL policy on %s.%s deletes expired documents", coll, field)), Line: c.on(f, ttl[0].line)}
		}
		if dynamicNamed(f.body, "ttl_config") {
			return setting(domain.Unknown, c.where(f, "ttl_config comes from a dynamic block"))
		}
	}
	return setting(domain.No, "no google_firestore_field sets a TTL policy, so documents are kept forever")
}

// firstPositive is Yes for the first numeric attribute above zero. Otherwise an unreadable attribute makes it
// unknown, a zero makes it No, and with none of them set it is absent.
func (c *check) firstPositive(paths []string, absent domain.Setting) domain.Setting {
	var unknown, zero *domain.Setting
	for _, p := range paths {
		s := c.number(c.r, p, "", absent)
		switch {
		case s == absent:
		case s.State == domain.Yes:
			return s
		case s.State == domain.Unknown && unknown == nil:
			unknown = &s
		case s.State == domain.No && zero == nil:
			zero = &s
		}
	}
	switch {
	case absent.State == domain.Yes:
		return absent
	case unknown != nil:
		return *unknown
	case zero != nil:
		return *zero
	}
	return absent
}

func (c *check) dataset() *resource {
	a := c.r.body.attrs["dataset_id"]
	if a == nil {
		return nil
	}
	if r := c.m.target(a.val); r != nil && r.typ == "google_bigquery_dataset" {
		return r
	}
	v := c.m.eval(a.val)
	if !v.known {
		return nil
	}
	for _, r := range c.m.order {
		if r.typ != "google_bigquery_dataset" {
			continue
		}
		if id, _, ok := c.text(r, "dataset_id"); ok && id == v.s {
			return r
		}
	}
	return nil
}

var publicMembers = []string{"allUsers", "allAuthenticatedUsers"}

// publicGrant looks for IAM resources that grant a role on this store to everyone.
func (c *check) publicGrant(types []string, attr string) domain.Setting {
	for _, typ := range types {
		for _, r := range c.related(typ, attr) {
			for _, name := range []string{"member", "members", "entity"} {
				a := r.body.attrs[name]
				if a == nil {
					continue
				}
				members, ok := c.m.eval(a.val).strings()
				if !ok {
					return domain.Setting{State: domain.Unknown, Why: c.where(r, unresolved(name, a)), Line: c.on(r, a.line)}
				}
				for _, m := range members {
					if contains(publicMembers, m) {
						return domain.Setting{State: domain.Yes, Why: c.where(r, fmt.Sprintf("a role is granted to %s", m)), Line: c.on(r, a.line)}
					}
				}
			}
		}
	}
	return setting(domain.No, "no role is granted to allUsers or allAuthenticatedUsers")
}

func (c *check) datasetPublic() domain.Setting {
	for _, acc := range blocksNamed(c.r.body, "access") {
		for _, name := range []string{"special_group", "iam_member"} {
			if a := acc.attrs[name]; a != nil {
				if v := c.m.eval(a.val); v.known && contains(publicMembers, v.s) {
					return domain.Setting{State: domain.Yes, Why: fmt.Sprintf("access is granted to %s", v.s), Line: a.line}
				}
			}
		}
	}
	return c.publicGrant([]string{"google_bigquery_dataset_iam_member", "google_bigquery_dataset_iam_binding"}, "dataset_id")
}

// sqlPublic is Yes when an authorised network is open to the whole internet.
func (c *check) sqlPublic() domain.Setting {
	settings := blocksNamed(c.r.body, "settings")
	if len(settings) == 0 {
		return setting(domain.No, "no authorised network is open to the internet")
	}
	for _, ipc := range blocksNamed(settings[0], "ip_configuration") {
		if dynamicNamed(ipc, "authorized_networks") {
			return setting(domain.Unknown, "authorized_networks comes from a dynamic block")
		}
		for _, n := range blocksNamed(ipc, "authorized_networks") {
			a := n.attrs["value"]
			if a == nil {
				continue
			}
			v := c.m.eval(a.val)
			if !v.known {
				return domain.Setting{State: domain.Unknown, Why: unresolved("authorized_networks.value", a), Line: a.line}
			}
			if v.s == "0.0.0.0/0" || v.s == "::/0" {
				return domain.Setting{State: domain.Yes, Why: fmt.Sprintf("an authorised network is %s", v.s), Line: a.line}
			}
		}
	}
	return setting(domain.No, "no authorised network is open to the internet")
}

// gcsRetention looks for a lifecycle rule that deletes objects.
func (c *check) gcsRetention() domain.Setting {
	if dynamicNamed(c.r.body, "lifecycle_rule") {
		return setting(domain.Unknown, "lifecycle_rule comes from a dynamic block")
	}
	rules := blocksNamed(c.r.body, "lifecycle_rule")
	for _, rule := range rules {
		a, _ := lookup(rule, "action.type")
		if a == nil {
			continue
		}
		v := c.m.eval(a.val)
		if !v.known {
			return domain.Setting{State: domain.Unknown, Why: unresolved("lifecycle_rule.action.type", a), Line: a.line}
		}
		if v.s == "Delete" {
			return domain.Setting{State: domain.Yes, Why: "a lifecycle rule deletes objects", Line: a.line}
		}
	}
	if len(rules) > 0 {
		return setting(domain.No, "lifecycle rules exist but none deletes objects")
	}
	return setting(domain.No, "no lifecycle rule deletes objects, so they are kept forever")
}

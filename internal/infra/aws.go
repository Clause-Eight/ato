package infra

import (
	"fmt"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

// Defaults below follow the AWS provider and service documentation. Where a default changed between versions
// or depends on account settings, an unset attribute is unknown.

func aws(typ, service string, names []string, assess func(c *check)) {
	register(typ, spec{provider: "aws", service: service, names: names, assess: assess, region: func(c *check) ([]string, domain.Setting) {
		return c.regionAttr("region", c.providerRegion)
	}})
}

func init() {
	aws("aws_db_instance", "database", []string{"identifier", "db_name"}, func(c *check) {
		c.s.Encryption = c.flag(c.r, "storage_encrypted", domain.Yes, domain.No, c.inherited("storage_encrypted is not set; RDS leaves storage unencrypted by default", "replicate_source_db", "snapshot_identifier"))
		c.s.Public = c.flag(c.r, "publicly_accessible", domain.Yes, domain.No, setting(domain.No, "publicly_accessible is not set; the default is private"))
		c.s.Backup = c.number(c.r, "backup_retention_period", "days", setting(domain.Unknown, "backup_retention_period is not set"))
	})
	aws("aws_rds_cluster", "database", []string{"cluster_identifier", "database_name"}, func(c *check) {
		absent := c.inherited("storage_encrypted is not set; Aurora leaves storage unencrypted by default", "replication_source_identifier", "snapshot_identifier")
		if mode, _, _ := c.text(c.r, "engine_mode"); mode == "serverless" {
			absent = setting(domain.Yes, "Aurora Serverless v1 clusters are always encrypted")
		}
		c.s.Encryption = c.flag(c.r, "storage_encrypted", domain.Yes, domain.No, absent)
		c.s.Backup = c.number(c.r, "backup_retention_period", "days", setting(domain.Yes, "backup_retention_period is not set; the default is 1 day"))
	})
	aws("aws_docdb_cluster", "database", []string{"cluster_identifier"}, func(c *check) {
		c.s.Encryption = c.flag(c.r, "storage_encrypted", domain.Yes, domain.No, c.inherited("storage_encrypted is not set; DocumentDB leaves storage unencrypted by default", "snapshot_identifier"))
		c.s.Backup = c.number(c.r, "backup_retention_period", "days", setting(domain.Yes, "backup_retention_period is not set; the default is 1 day"))
	})
	aws("aws_redshift_cluster", "warehouse", []string{"cluster_identifier", "database_name"}, func(c *check) {
		c.s.Encryption = c.flag(c.r, "encrypted", domain.Yes, domain.No, setting(domain.Unknown, "encrypted is not set; the default depends on the provider version"))
		c.s.Public = c.flag(c.r, "publicly_accessible", domain.Yes, domain.No, setting(domain.Unknown, "publicly_accessible is not set; the default depends on the provider version"))
		c.s.Backup = c.number(c.r, "automated_snapshot_retention_period", "days", setting(domain.Yes, "automated_snapshot_retention_period is not set; the default is 1 day"))
	})
	aws("aws_dynamodb_table", "key-value", []string{"name"}, func(c *check) {
		c.s.Encryption = setting(domain.Yes, "DynamoDB always encrypts tables at rest")
		c.s.Retention = c.flag(c.r, "ttl.enabled", domain.Yes, domain.No, setting(domain.No, "no ttl is enabled, so items never expire"))
		if c.s.Retention.State == domain.No && len(blocksNamed(c.r.body, "ttl")) == 0 && !dynamicNamed(c.r.body, "ttl") {
			c.s.Retention.Why = "no ttl block, so items never expire"
		}
		c.s.Backup = c.flag(c.r, "point_in_time_recovery.enabled", domain.Yes, domain.No, setting(domain.No, "point_in_time_recovery is not enabled"))
		c.addReplicaRegions()
	})
	aws("aws_s3_bucket", "object-storage", []string{"bucket", "bucket_prefix"}, func(c *check) {
		c.s.Encryption = setting(domain.Yes, "S3 encrypts every new object at rest (SSE-S3 by default since January 2023)")
		c.s.Retention = c.s3Retention()
		c.s.Backup = c.s3Versioning()
		c.s.Public = c.s3Public()
	})
	aws("aws_cloudwatch_log_group", "log", []string{"name", "name_prefix"}, func(c *check) {
		c.s.Encryption = setting(domain.Yes, "CloudWatch Logs always encrypts log data at rest")
		c.s.Retention = c.number(c.r, "retention_in_days", "days", setting(domain.No, "retention_in_days is not set, so logs are kept forever"))
		if c.s.Retention.State == domain.No && c.s.Retention.Line > 0 {
			c.s.Retention.Why += " (0 keeps logs forever)"
		}
	})
	aws("aws_sqs_queue", "queue", []string{"name", "name_prefix"}, func(c *check) {
		if a := c.r.body.attrs["kms_master_key_id"]; a != nil {
			c.s.Encryption = domain.Setting{State: domain.Yes, Why: "kms_master_key_id is set", Line: a.line}
		} else {
			c.s.Encryption = c.flag(c.r, "sqs_managed_sse_enabled", domain.Yes, domain.No, setting(domain.Yes, "SQS encrypts new queues with SSE-SQS by default"))
		}
		c.s.Retention = setting(domain.Yes, "SQS deletes messages after message_retention_seconds (4 days by default, 14 at most)")
	})
	aws("aws_sns_topic", "topic", []string{"name", "name_prefix"}, func(c *check) {
		if a := c.r.body.attrs["kms_master_key_id"]; a != nil {
			c.s.Encryption = domain.Setting{State: domain.Yes, Why: "kms_master_key_id is set", Line: a.line}
		} else {
			c.s.Encryption = setting(domain.No, "kms_master_key_id is not set, so messages are not encrypted at rest")
		}
	})
	aws("aws_kinesis_stream", "stream", []string{"name"}, func(c *check) {
		c.s.Encryption = c.enumSetting("encryption_type", map[string]domain.Tri{"KMS": domain.Yes, "NONE": domain.No}, setting(domain.No, "encryption_type is not set; the default is NONE"))
		c.s.Retention = setting(domain.Yes, "Kinesis deletes records after retention_period (24 hours by default)")
	})
	aws("aws_elasticache_replication_group", "cache", []string{"replication_group_id"}, func(c *check) {
		c.s.Encryption = c.flag(c.r, "at_rest_encryption_enabled", domain.Yes, domain.No, setting(domain.Unknown, "at_rest_encryption_enabled is not set; the default depends on the engine version"))
		c.s.Backup = c.number(c.r, "snapshot_retention_limit", "days", setting(domain.No, "snapshot_retention_limit is not set, so there are no automatic backups"))
	})
	for _, typ := range []string{"aws_opensearch_domain", "aws_elasticsearch_domain"} {
		aws(typ, "search", []string{"domain_name"}, func(c *check) {
			c.s.Encryption = c.flag(c.r, "encrypt_at_rest.enabled", domain.Yes, domain.No, setting(domain.Unknown, "encrypt_at_rest is not set; the default depends on the engine version"))
			if len(blocksNamed(c.r.body, "vpc_options")) > 0 {
				c.s.Public = setting(domain.No, "the domain runs inside a VPC")
			} else {
				c.s.Public = setting(domain.Unknown, "no vpc_options, so the endpoint is public and access depends on the access policy")
			}
		})
	}
	aws("aws_ebs_volume", "block-storage", nil, func(c *check) {
		absent := setting(domain.Unknown, "encrypted is not set; it depends on the account's EBS encryption by default")
		for _, r := range c.m.order {
			if r.typ == "aws_ebs_encryption_by_default" {
				absent = c.flag(r, "enabled", domain.Yes, domain.No, setting(domain.Yes, "EBS encryption by default is on"))
			}
		}
		c.s.Encryption = c.flag(c.r, "encrypted", domain.Yes, domain.No, absent)
	})
	aws("aws_efs_file_system", "file-storage", []string{"creation_token"}, func(c *check) {
		c.s.Encryption = c.flag(c.r, "encrypted", domain.Yes, domain.No, setting(domain.No, "encrypted is not set; EFS leaves file systems unencrypted by default"))
	})
}

// inherited is the setting for an unset encryption attribute: unknown when the store is created from a
// snapshot or a source, whose encryption it takes over, and No otherwise.
func (c *check) inherited(why string, sources ...string) domain.Setting {
	for _, s := range sources {
		if c.r.body.attrs[s] != nil {
			return setting(domain.Unknown, fmt.Sprintf("storage_encrypted is not set and the store is created from %s, whose encryption it keeps", s))
		}
	}
	return setting(domain.No, why)
}

// enumSetting reads a string attribute whose values map to states.
func (c *check) enumSetting(path string, states map[string]domain.Tri, absent domain.Setting) domain.Setting {
	s, a, ok := c.text(c.r, path)
	switch {
	case a == nil:
		return absent
	case !ok:
		return domain.Setting{State: domain.Unknown, Why: unresolved(path, a), Line: a.line}
	}
	state, known := states[strings.ToUpper(s)]
	if !known {
		return domain.Setting{State: domain.Unknown, Why: fmt.Sprintf("%s has a value pii-eu does not know", path), Line: a.line}
	}
	return domain.Setting{State: state, Why: fmt.Sprintf("%s = %q", path, s), Line: a.line}
}

// addReplicaRegions adds the regions of DynamoDB global table replicas.
func (c *check) addReplicaRegions() {
	replicas := blocksNamed(c.r.body, "replica")
	if len(replicas) == 0 && !dynamicNamed(c.r.body, "replica") {
		return
	}
	regions := append([]string{}, c.s.Regions...)
	known := len(regions) > 0 && !dynamicNamed(c.r.body, "replica")
	for _, rep := range replicas {
		a := rep.attrs["region_name"]
		if a == nil {
			known = false
			continue
		}
		v := c.m.eval(a.val)
		if !v.known || v.obj != nil {
			known = false
			continue
		}
		if !contains(regions, v.s) {
			regions = append(regions, v.s)
		}
	}
	c.s.Regions = regions
	found := setting(domain.NotApplicable, "with its replicas")
	if !known {
		found = setting(domain.Unknown, "the region of the table or of a replica cannot be read")
	}
	c.s.InEEA, c.s.Adequacy = residencyOf("aws", regions, found)
}

// s3Retention looks for a lifecycle rule that expires current objects, inline or in a lifecycle configuration.
func (c *check) s3Retention() domain.Setting {
	sources := []struct {
		r     *resource
		rules []*block
		dyn   bool
		on    string
	}{{c.r, blocksNamed(c.r.body, "lifecycle_rule"), dynamicNamed(c.r.body, "lifecycle_rule"), "enabled"}}
	for _, lc := range c.related("aws_s3_bucket_lifecycle_configuration", "bucket") {
		sources = append(sources, struct {
			r     *resource
			rules []*block
			dyn   bool
			on    string
		}{lc, blocksNamed(lc.body, "rule"), dynamicNamed(lc.body, "rule"), "status"})
	}
	rules, unknown := 0, false
	for _, src := range sources {
		unknown = unknown || src.dyn
		for _, rule := range src.rules {
			rules++
			if a := rule.attrs[src.on]; a != nil {
				v := c.m.eval(a.val)
				if !v.known {
					unknown = true
					continue
				}
				if v.s == "false" || strings.EqualFold(v.s, "Disabled") {
					continue
				}
			}
			for _, exp := range blocksNamed(rule, "expiration") {
				if exp.attrs["days"] != nil || exp.attrs["date"] != nil {
					return domain.Setting{State: domain.Yes, Why: c.where(src.r, "a lifecycle rule expires objects"), Line: c.on(src.r, exp.line)}
				}
			}
			unknown = unknown || dynamicNamed(rule, "expiration")
		}
	}
	switch {
	case unknown:
		return setting(domain.Unknown, "a lifecycle rule is generated or switched by an expression")
	case rules > 0:
		return setting(domain.No, "lifecycle rules exist but none expires current objects")
	}
	return setting(domain.No, "no lifecycle rule expires objects, so they are kept forever")
}

func (c *check) s3Versioning() domain.Setting {
	if vs := c.related("aws_s3_bucket_versioning", "bucket"); len(vs) > 0 {
		return c.enumOf(vs[len(vs)-1], "versioning_configuration.status", map[string]domain.Tri{"ENABLED": domain.Yes, "SUSPENDED": domain.No, "DISABLED": domain.No})
	}
	return c.flag(c.r, "versioning.enabled", domain.Yes, domain.No, setting(domain.No, "versioning is not enabled"))
}

func (c *check) enumOf(r *resource, path string, states map[string]domain.Tri) domain.Setting {
	s, a, ok := c.text(r, path)
	switch {
	case a == nil:
		return setting(domain.Unknown, c.where(r, path+" is not set"))
	case !ok:
		return domain.Setting{State: domain.Unknown, Why: c.where(r, unresolved(path, a)), Line: c.on(r, a.line)}
	}
	state, known := states[strings.ToUpper(s)]
	if !known {
		state = domain.Unknown
	}
	return domain.Setting{State: state, Why: c.where(r, fmt.Sprintf("%s = %q", path, s)), Line: c.on(r, a.line)}
}

var publicACLs = map[string]bool{"public-read": true, "public-read-write": true, "authenticated-read": true}

// s3Public reports a bucket as public when an ACL grants public access and no public access block ignores it.
// Without a public grant a bucket is private: new buckets block public access by default since April 2023.
func (c *check) s3Public() domain.Setting {
	ignoresACLs := false
	for _, pab := range c.related("aws_s3_bucket_public_access_block", "bucket") {
		if a, _ := lookup(pab.body, "ignore_public_acls"); a != nil {
			if v := c.m.eval(a.val); v.known && v.s == "true" {
				ignoresACLs = true
			}
		}
	}
	type source struct {
		r    *resource
		path string
	}
	sources := []source{{c.r, "acl"}}
	for _, acl := range c.related("aws_s3_bucket_acl", "bucket") {
		sources = append(sources, source{acl, "acl"})
	}
	for _, src := range sources {
		s, a, ok := c.text(src.r, src.path)
		if a == nil {
			continue
		}
		if !ok {
			return domain.Setting{State: domain.Unknown, Why: c.where(src.r, unresolved("acl", a)), Line: c.on(src.r, a.line)}
		}
		if publicACLs[s] {
			if ignoresACLs {
				return setting(domain.No, fmt.Sprintf("acl = %q, but the public access block ignores public ACLs", s))
			}
			return domain.Setting{State: domain.Yes, Why: c.where(src.r, fmt.Sprintf("acl = %q", s)), Line: c.on(src.r, a.line)}
		}
	}
	return setting(domain.No, "no public ACL; new buckets block public access by default")
}

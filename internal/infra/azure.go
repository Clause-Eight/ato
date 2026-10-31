package infra

import (
	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Azure encrypts storage, databases and logs at rest unless transparent data encryption is switched off. Every
// resource names its location; it is often azurerm_resource_group.x.location, which is followed.

func azure(typ, service string, region func(c *check) ([]string, domain.Setting), assess func(c *check)) {
	if region == nil {
		region = locationAttr
	}
	register(typ, spec{provider: "azurerm", service: service, names: []string{"name"}, region: region, assess: assess})
}

func init() {
	azure("azurerm_storage_account", "object-storage", nil, func(c *check) {
		c.s.Encryption = setting(domain.Yes, "Azure Storage always encrypts data at rest")
		c.s.Retention = c.azureLifecycle()
		c.s.Backup = c.flag(c.r, "blob_properties.versioning_enabled", domain.Yes, domain.No, setting(domain.No, "blob versioning is not enabled"))
		public := c.flag(c.r, "allow_nested_items_to_be_public", domain.Yes, domain.No, setting(domain.Unknown, "allow_nested_items_to_be_public is not set; the default depends on the provider version"))
		if c.r.body.attrs["allow_nested_items_to_be_public"] == nil && c.r.body.attrs["allow_blob_public_access"] != nil {
			public = c.flag(c.r, "allow_blob_public_access", domain.Yes, domain.No, public)
		}
		c.s.Public = public
	})
	azure("azurerm_mssql_database", "database", func(c *check) ([]string, domain.Setting) {
		a := c.r.body.attrs["server_id"]
		server := c.m.target(a.valOrNil())
		if server == nil || server.body.attrs["location"] == nil {
			return nil, setting(domain.Unknown, "the server and its location are not declared in this folder")
		}
		regions, found := c.regionFrom(server, server.body.attrs["location"], "location")
		if found.State != domain.Unknown {
			found.Why = "from " + server.address()
		}
		return regions, found
	}, func(c *check) {
		c.s.Encryption = c.flag(c.r, "transparent_data_encryption_enabled", domain.Yes, domain.No, setting(domain.Yes, "transparent data encryption is on by default"))
		c.s.Backup = c.number(c.r, "short_term_retention_policy.retention_days", "days", setting(domain.Yes, "Azure SQL keeps automated backups for 7 days by default"))
	})
	azure("azurerm_postgresql_flexible_server", "database", nil, func(c *check) {
		c.s.Encryption = setting(domain.Yes, "Azure Database for PostgreSQL always encrypts data at rest")
		c.s.Backup = c.number(c.r, "backup_retention_days", "days", setting(domain.Yes, "backup_retention_days is not set; the default is 7 days"))
		absent := setting(domain.Unknown, "public_network_access_enabled is not set and the server is not in a delegated subnet")
		if c.r.body.attrs["delegated_subnet_id"] != nil {
			absent = setting(domain.No, "the server runs in a delegated subnet")
		}
		c.s.Public = c.flag(c.r, "public_network_access_enabled", domain.Yes, domain.No, absent)
	})
	azure("azurerm_cosmosdb_account", "database", func(c *check) ([]string, domain.Setting) {
		geo := blocksNamed(c.r.body, "geo_location")
		if len(geo) == 0 || dynamicNamed(c.r.body, "geo_location") {
			return nil, setting(domain.Unknown, "the geo_location blocks cannot be read")
		}
		var regions []string
		found := domain.Setting{}
		for _, g := range geo {
			a := g.attrs["location"]
			if a == nil {
				return nil, setting(domain.Unknown, "a geo_location has no location")
			}
			rs, f := c.regionFrom(c.r, a, "geo_location.location")
			if f.State == domain.Unknown {
				return nil, f
			}
			if found.Line == 0 {
				found.Line = f.Line
			}
			regions = append(regions, rs...)
		}
		return regions, found
	}, func(c *check) {
		c.s.Encryption = setting(domain.Yes, "Cosmos DB always encrypts data at rest")
		c.s.Public = c.flag(c.r, "public_network_access_enabled", domain.Yes, domain.No, setting(domain.Unknown, "public_network_access_enabled is not set; it is on by default and access depends on the firewall"))
	})
	azure("azurerm_log_analytics_workspace", "log", nil, func(c *check) {
		c.s.Encryption = setting(domain.Yes, "Azure Monitor always encrypts log data at rest")
		c.s.Retention = c.number(c.r, "retention_in_days", "days", setting(domain.Yes, "retention_in_days is not set; the default is 30 days"))
	})
}

func (a *attr) valOrNil() *expr {
	if a == nil {
		return nil
	}
	return a.val
}

// azureLifecycle looks for a storage management policy that deletes blobs of the account.
func (c *check) azureLifecycle() domain.Setting {
	policies := c.related("azurerm_storage_management_policy", "storage_account_id")
	for _, p := range policies {
		for _, rule := range blocksNamed(p.body, "rule") {
			for _, actions := range blocksNamed(rule, "actions") {
				for _, base := range blocksNamed(actions, "base_blob") {
					for _, name := range []string{"delete_after_days_since_modification_greater_than", "delete_after_days_since_creation_greater_than", "delete_after_days_since_last_access_time_greater_than"} {
						if a := base.attrs[name]; a != nil {
							return domain.Setting{State: domain.Yes, Why: c.where(p, "a management policy deletes blobs"), Line: c.on(p, a.line)}
						}
					}
				}
			}
			if dynamicNamed(rule, "actions") {
				return setting(domain.Unknown, c.where(p, "the policy actions come from a dynamic block"))
			}
		}
	}
	if len(policies) > 0 {
		return setting(domain.No, "the management policy does not delete blobs")
	}
	return setting(domain.No, "no management policy deletes blobs, so they are kept forever")
}

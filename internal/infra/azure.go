package infra

import (
	"fmt"

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
		return c.parentRegion("the server", "server_id")
	}, func(c *check) {
		c.s.Encryption = c.flag(c.r, "transparent_data_encryption_enabled", domain.Yes, domain.No, setting(domain.Yes, "transparent data encryption is on by default"))
		c.s.Backup = c.number(c.r, "short_term_retention_policy.retention_days", "days", setting(domain.Yes, "Azure SQL keeps automated backups for 7 days by default"))
		c.s.Public = c.mssqlPublic()
	})
	for _, db := range []struct{ typ, engine string }{{"azurerm_postgresql_flexible_server", "PostgreSQL"}, {"azurerm_mysql_flexible_server", "MySQL"}} {
		azure(db.typ, "database", nil, func(c *check) {
			c.s.Encryption = setting(domain.Yes, "Azure Database for "+db.engine+" always encrypts data at rest")
			c.s.Backup = c.number(c.r, "backup_retention_days", "days", setting(domain.Yes, "backup_retention_days is not set; the default is 7 days"))
			absent := setting(domain.Unknown, "public_network_access_enabled is not set and the server is not in a delegated subnet")
			if c.r.body.attrs["delegated_subnet_id"] != nil {
				absent = setting(domain.No, "the server runs in a delegated subnet")
			}
			c.s.Public = c.flag(c.r, "public_network_access_enabled", domain.Yes, domain.No, absent)
			if c.s.Public.State != domain.No {
				if open := c.openFirewall(c.r, db.typ+"_firewall_rule", "server_id"); open.State == domain.Yes {
					c.s.Public = open
				}
			}
		})
	}
	azure("azurerm_storage_container", "object-storage", func(c *check) ([]string, domain.Setting) {
		return c.parentRegion("the storage account", "storage_account_id", "storage_account_name")
	}, func(c *check) {
		c.s.Encryption = setting(domain.Yes, "Azure Storage always encrypts data at rest")
		account := c.storageAccount()
		if account == nil {
			c.s.Retention = setting(domain.Unknown, "the storage account is not declared in this folder, so its management policy cannot be read")
		} else {
			sub := &check{m: c.m, r: account, s: &domain.Store{Provider: c.s.Provider}}
			sub.s.Names = sub.names([]string{"name"})
			c.s.Retention = sub.azureLifecycle()
			c.s.Retention.Why = c.where(account, c.s.Retention.Why)
			c.s.Retention.Line = c.on(account, c.s.Retention.Line)
		}
		c.s.Public = c.enumSetting("container_access_type", map[string]domain.Tri{"PRIVATE": domain.No, "BLOB": domain.Yes, "CONTAINER": domain.Yes}, setting(domain.No, "container_access_type is not set; the default is private"))
		if c.s.Public.State == domain.Yes && account != nil {
			if a := account.body.attrs["allow_nested_items_to_be_public"]; a != nil {
				if v := c.m.eval(a.val); v.known && v.s == "false" {
					c.s.Public = setting(domain.No, c.where(account, "container_access_type allows anonymous reads, but allow_nested_items_to_be_public = false"))
				}
			}
		}
	})
	azure("azurerm_redis_cache", "cache", nil, func(c *check) {
		c.s.Encryption = setting(domain.Yes, "Azure Cache for Redis encrypts its disks and persistence files at rest")
		absent := setting(domain.Unknown, "public_network_access_enabled is not set; it is on by default and access needs an access key")
		if c.r.body.attrs["subnet_id"] != nil {
			absent = setting(domain.No, "the cache runs in a virtual network subnet")
		}
		c.s.Public = c.flag(c.r, "public_network_access_enabled", domain.Yes, domain.No, absent)
		if c.s.Public.State == domain.Yes {
			c.s.Public.State = domain.Unknown
			c.s.Public.Why += "; access still needs an access key and depends on firewall rules"
		}
		c.s.Backup = c.flag(c.r, "redis_configuration.rdb_backup_enabled", domain.Yes, domain.No, setting(domain.No, "RDB persistence is not enabled, so there are no backups"))
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
		c.s.Backup = setting(domain.Yes, "Cosmos DB always takes periodic or continuous backups")
		c.s.Public = c.flag(c.r, "public_network_access_enabled", domain.Yes, domain.No, setting(domain.Unknown, "public_network_access_enabled is not set; it is on by default and access depends on the firewall"))
	})
	azure("azurerm_log_analytics_workspace", "log", nil, func(c *check) {
		c.s.Encryption = setting(domain.Yes, "Azure Monitor always encrypts log data at rest")
		c.s.Retention = c.number(c.r, "retention_in_days", "days", setting(domain.Yes, "retention_in_days is not set; the default is 30 days"))
	})
}

// parentRegion reads the location of the resource that one of attrs refers to, such as a database's server.
func (c *check) parentRegion(what string, attrs ...string) ([]string, domain.Setting) {
	var parent *resource
	for _, name := range attrs {
		if parent = c.m.target(c.r.body.attrs[name].valOrNil()); parent != nil {
			break
		}
	}
	if parent == nil || parent.body.attrs["location"] == nil {
		return nil, setting(domain.Unknown, what+" and its location are not declared in this folder")
	}
	regions, found := c.regionFrom(parent, parent.body.attrs["location"], "location")
	if found.State != domain.Unknown {
		found.Why = "from " + parent.address()
	}
	return regions, found
}

// storageAccount is the account a container belongs to, by reference or by literal name.
func (c *check) storageAccount() *resource {
	for _, name := range []string{"storage_account_id", "storage_account_name"} {
		a := c.r.body.attrs[name]
		if a == nil {
			continue
		}
		if r := c.m.target(a.val); r != nil && r.typ == "azurerm_storage_account" {
			return r
		}
		v := c.m.eval(a.val)
		if !v.known {
			continue
		}
		for _, r := range c.m.order {
			if n, _, ok := c.text(r, "name"); r.typ == "azurerm_storage_account" && ok && n == v.s {
				return r
			}
		}
	}
	return nil
}

// mssqlPublic reads public access from the database's server and the server's firewall rules.
func (c *check) mssqlPublic() domain.Setting {
	server := c.m.target(c.r.body.attrs["server_id"].valOrNil())
	if server == nil {
		return setting(domain.Unknown, "the server is not declared in this folder")
	}
	if open := c.openFirewall(server, "azurerm_mssql_firewall_rule", "server_id"); open.State == domain.Yes {
		return open
	}
	public := c.flag(server, "public_network_access_enabled", domain.Yes, domain.No, setting(domain.Unknown, c.where(server, "public_network_access_enabled is not set; it is on by default and access depends on firewall rules")))
	if public.State == domain.Yes {
		public = domain.Setting{State: domain.Unknown, Why: public.Why + "; access depends on firewall rules", Line: public.Line}
	}
	return public
}

// openFirewall is Yes when a firewall rule of the server opens it to every address. A rule from 0.0.0.0 to
// 0.0.0.0 only lets Azure services in, so it does not count.
func (c *check) openFirewall(server *resource, typ, attr string) domain.Setting {
	for _, r := range c.m.order {
		if r.typ != typ || c.m.target(r.body.attrs[attr].valOrNil()) != server {
			continue
		}
		start, _, ok1 := c.text(r, "start_ip_address")
		end, a, ok2 := c.text(r, "end_ip_address")
		if ok1 && ok2 && a != nil && start == "0.0.0.0" && end == "255.255.255.255" {
			return domain.Setting{State: domain.Yes, Why: c.where(r, fmt.Sprintf("a firewall rule allows %s to %s", start, end)), Line: c.on(r, a.line)}
		}
	}
	return setting(domain.No, "no firewall rule opens the server to every address")
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

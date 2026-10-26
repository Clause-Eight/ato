package domain

// KindInfra is a finding about a data store declared in infrastructure code, such as a Terraform resource.
const KindInfra Kind = "infra"

// Tri is what the code says about one setting of a store. The zero value means the setting does not apply to
// that kind of store. Unknown means the value comes from something that cannot be read without running the
// tool, such as a variable without a default; it is never read as No.
type Tri string

const (
	NotApplicable Tri = ""
	Yes           Tri = "yes"
	No            Tri = "no"
	Unknown       Tri = "unknown"
)

// String is the state as written in reports.
func (t Tri) String() string {
	if t == NotApplicable {
		return "not-applicable"
	}
	return string(t)
}

// Setting is one assessed setting of a store: its state, why, and the line that decided it (0 for the line of
// the resource itself).
type Setting struct {
	State Tri
	Why   string
	Line  int
}

// Store is a place where a deployment keeps data: a database, a bucket, a queue, a log group. Values are what
// the code declares; nothing is read from a cloud account.
type Store struct {
	// Type is the resource type, such as aws_s3_bucket; Name is its label in the code.
	Type string
	Name string
	// Provider is aws, google or azurerm. Service is the kind of store, such as object-storage or database.
	Provider string
	Service  string
	// Names are the names the store has in the cloud (bucket, table, database), as far as they are written
	// out. Parts that come from variables are left out, so a name may be partial.
	Names []string
	// Regions where the store keeps its data; empty when they cannot be read.
	Regions []string
	// InEEA says whether every region is in the European Economic Area. Adequacy is the country of a region
	// outside the EEA that has an EU adequacy decision (GDPR Art. 45), such as CH or GB.
	InEEA    Setting
	Adequacy string

	Encryption Setting
	Retention  Setting
	Backup     Setting
	Public     Setting

	// Tags are the literal tags or labels; values set from expressions are left out.
	Tags map[string]string
	// PersonalData says the store is linked to personal data, by a classification tag or because its name
	// matches a table or model in the inventory; PersonalWhy says how.
	PersonalData bool
	PersonalWhy  string

	Location Location
}

// Address is how Terraform names the resource: type.name.
func (s Store) Address() string { return s.Type + "." + s.Name }

// InfraCheck is one kind of finding about stores, with the detector name used in settings and reports.
type InfraCheck struct {
	Detector string
	Label    string
	// When says what makes a finding; Rule is the GDPR provision the check supports, and Help what to do.
	When string
	Rule string
	Help string
}

// The checks on stores, in the order they are documented.
var InfraChecks = []InfraCheck{
	{Detector: "tf-unencrypted", Label: "Data store without encryption at rest", Rule: "GDPR Art. 32(1)(a)",
		When: "encryption at rest is switched off, or not set where the service leaves it off",
		Help: "Turn on encryption at rest for the store, with a customer-managed key where the data is sensitive."},
	{Detector: "tf-no-retention", Label: "Data store without a retention limit", Rule: "GDPR Art. 5(1)(e)",
		When: "nothing expires the data: no lifecycle rule, retention period or TTL",
		Help: "Set an expiry (lifecycle rule, retention period or TTL) that matches how long the data is needed."},
	{Detector: "tf-non-eea-region", Label: "Data store outside the EEA", Rule: "GDPR Chapter V",
		When: "a region is outside the EEA; one step lower when the country has an EU adequacy decision",
		Help: "Move the store to a region in the EEA, or document the transfer mechanism (adequacy decision, SCCs) for this region."},
	{Detector: "tf-public", Label: "Data store open to the public", Rule: "GDPR Art. 32(1)(b)",
		When: "a public ACL, a grant to everyone or an internet-facing endpoint opens the store",
		Help: "Remove the public grant or public endpoint, and give access to named principals or private networks only."},
}

// InfraCheckFor returns the check with the detector name.
func InfraCheckFor(detector string) (InfraCheck, bool) {
	for _, c := range InfraChecks {
		if c.Detector == detector {
			return c, true
		}
	}
	return InfraCheck{}, false
}

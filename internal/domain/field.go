package domain

// Kind says what a finding is: a personal-data value, or a schema field that holds a special category of data.
type Kind string

const (
	// KindValue is a value found in text, confirmed by a detector.
	KindValue Kind = "value"
	// KindField is a column, property or message field whose name says it holds special-category data.
	KindField Kind = "field"
	// KindSink is a call that writes personal data to a log, an error tracker or analytics.
	KindSink Kind = "sink"
	// KindLifecycle is a table, model or collection holding personal data that the code never erases or expires.
	KindLifecycle Kind = "lifecycle"
)

// Field is a named place in a schema or model that holds data: a table column, a model attribute, an API or
// JSON Schema property, a protobuf message field or a dbt column.
type Field struct {
	Name      string
	Container string
	Format    string
	Location  Location
}

// DataCategory is the kind of personal data a field holds, as a record of processing (GDPR Art. 30) lists it.
type DataCategory string

const (
	CategoryName        DataCategory = "name"
	CategoryContact     DataCategory = "contact"
	CategoryAddress     DataCategory = "address"
	CategoryIdentifier  DataCategory = "government-identifier"
	CategoryFinancial   DataCategory = "financial"
	CategoryBirth       DataCategory = "birth-date"
	CategoryLocation    DataCategory = "location"
	CategoryOnline      DataCategory = "online-identifier"
	CategoryDemographic DataCategory = "demographic"
	CategorySpecial     DataCategory = "special-category"
)

// SpecialCategory is one of the categories of GDPR Art. 9(1).
type SpecialCategory string

const (
	SpecialHealth       SpecialCategory = "health"
	SpecialGenetic      SpecialCategory = "genetic"
	SpecialBiometric    SpecialCategory = "biometric"
	SpecialRacialEthnic SpecialCategory = "racial-or-ethnic-origin"
	SpecialPolitical    SpecialCategory = "political-opinions"
	SpecialReligious    SpecialCategory = "religious-or-philosophical-beliefs"
	SpecialTradeUnion   SpecialCategory = "trade-union-membership"
	SpecialSexLife      SpecialCategory = "sex-life-or-sexual-orientation"
)

// SpecialCategories lists the Art. 9 categories in the order of the article.
var SpecialCategories = []SpecialCategory{SpecialRacialEthnic, SpecialPolitical, SpecialReligious, SpecialTradeUnion, SpecialGenetic, SpecialBiometric, SpecialHealth, SpecialSexLife}

// Classification is what a field name says about its data. Special is set only for Art. 9 data, and then
// Category is CategorySpecial. Matched is the dictionary word that decided it.
type Classification struct {
	Category DataCategory
	Special  SpecialCategory
	Matched  string
}

// InventoryEntry is one row of a data inventory: a field that holds personal data.
type InventoryEntry struct {
	Field          Field
	Classification Classification
}

// RecipientUse is a place where code names a third-party service that may receive personal data. EEA says
// whether the provider is established in the EEA. Evidence is "dependency", "import" or "host".
type RecipientUse struct {
	Name     string
	Purpose  string
	EEA      bool
	Evidence string
	Location Location
}

// Removal is a place in the code that removes data from a store: How names the mechanism ("SQL DELETE",
// "TTL index", "ON DELETE CASCADE").
type Removal struct {
	How      string
	Location Location
}

// StoreLifecycle says how the code removes data from one store: erasure of a person's data (GDPR Art. 17) and
// removal after a retention period (Art. 5(1)(e)). Empty slices mean nothing was found, not that nothing exists.
type StoreLifecycle struct {
	Store     string
	Erasure   []Removal
	Retention []Removal
}

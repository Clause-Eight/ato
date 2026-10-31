package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// InventorySchemaVersion is the version of the JSON inventory, versioned like the findings report.
const InventorySchemaVersion = "2.1"

// Inventory is a draft record of the personal data a code base declares: field names and where they are, and the
// data stores its Terraform deploys. Never values.
type Inventory struct {
	Tool    string
	Version string
	Entries []domain.InventoryEntry
	Stores  []domain.Store
	Files   int
}

// InventoryFormats are the formats `ato inventory` writes.
var InventoryFormats = []string{"json", "csv", "html"}

type jsonInventory struct {
	SchemaVersion string           `json:"schema_version"`
	Tool          jsonTool         `json:"tool"`
	FilesScanned  int              `json:"files_scanned"`
	Entries       []jsonInventoryE `json:"entries"`
	Stores        []jsonStoreE     `json:"stores"`
}

// jsonStoreE is a data store in the inventory. Tag values are left out: they are free text and may name people.
type jsonStoreE struct {
	Type         string      `json:"type"`
	Name         string      `json:"name"`
	Provider     string      `json:"provider"`
	Service      string      `json:"service"`
	Names        []string    `json:"names"`
	Regions      []string    `json:"regions"`
	InEEA        string      `json:"in_eea"`
	Adequacy     string      `json:"adequacy,omitempty"`
	Encrypted    string      `json:"encrypted"`
	Retention    string      `json:"retention"`
	Backup       string      `json:"backup"`
	Public       string      `json:"public"`
	PersonalData bool        `json:"personal_data"`
	PersonalWhy  string      `json:"personal_data_reason,omitempty"`
	Module       *jsonModule `json:"module,omitempty"`
	Path         string      `json:"path"`
	Line         int         `json:"line"`
}

// jsonModule is a module call that ATO does not read inside: its source and the arguments that look like
// store settings, by name.
type jsonModule struct {
	Source   string   `json:"source"`
	Settings []string `json:"settings"`
}

func moduleOf(s domain.Store) *jsonModule {
	if s.Module == "" {
		return nil
	}
	return &jsonModule{Source: s.Module, Settings: nonNilStrings(s.ModuleSettings)}
}

func storeEntry(s domain.Store) jsonStoreE {
	return jsonStoreE{
		Type: s.Type, Name: s.Name, Provider: s.Provider, Service: s.Service, Names: nonNilStrings(s.Names), Regions: nonNilStrings(s.Regions),
		InEEA: s.InEEA.State.String(), Adequacy: s.Adequacy, Encrypted: s.Encryption.State.String(), Retention: s.Retention.State.String(),
		Backup: s.Backup.State.String(), Public: s.Public.State.String(), PersonalData: s.PersonalData, PersonalWhy: s.PersonalWhy,
		Module: moduleOf(s), Path: s.Location.Path, Line: s.Location.Line,
	}
}

type jsonInventoryE struct {
	Field           string `json:"field"`
	Container       string `json:"container"`
	DataCategory    string `json:"data_category"`
	SpecialCategory string `json:"special_category,omitempty"`
	Matched         string `json:"matched"`
	SourceFormat    string `json:"source_format"`
	Path            string `json:"path"`
	Line            int    `json:"line"`
	Column          int    `json:"column"`
}

// WriteInventory writes the inventory as JSON, as CSV laid out like an Art. 30 record, or as an HTML page.
// Stores are written in JSON only, since the CSV columns are fixed.
func WriteInventory(format string, w io.Writer, inv Inventory) error {
	switch format {
	case "json":
		out := jsonInventory{SchemaVersion: InventorySchemaVersion, Tool: jsonTool{Name: inv.Tool, Version: inv.Version}, FilesScanned: inv.Files, Entries: make([]jsonInventoryE, 0, len(inv.Entries)), Stores: make([]jsonStoreE, 0, len(inv.Stores))}
		for _, s := range inv.Stores {
			out.Stores = append(out.Stores, storeEntry(s))
		}
		for _, e := range inv.Entries {
			out.Entries = append(out.Entries, jsonInventoryE{
				Field: e.Field.Name, Container: e.Field.Container, DataCategory: string(e.Classification.Category),
				SpecialCategory: string(e.Classification.Special), Matched: e.Classification.Matched, SourceFormat: e.Field.Format,
				Path: e.Field.Location.Path, Line: e.Field.Location.Line, Column: e.Field.Location.Column,
			})
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(out)
	case "csv":
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"Field", "Table or model", "Category of personal data", "Special category (Art. 9)", "Source format", "Source file", "Line"})
		for _, e := range inv.Entries {
			_ = cw.Write([]string{e.Field.Name, e.Field.Container, string(e.Classification.Category), string(e.Classification.Special), e.Field.Format, e.Field.Location.Path, fmt.Sprint(e.Field.Location.Line)})
		}
		cw.Flush()
		return cw.Error()
	case "html":
		return writeInventoryHTML(w, inv)
	}
	return fmt.Errorf("unknown inventory format %q: use json, csv or html", format)
}

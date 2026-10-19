package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// InventorySchemaVersion is the version of the JSON inventory, versioned like the findings report.
const InventorySchemaVersion = "1.0"

// Inventory is a draft record of the personal data a code base declares: field names and where they are, never values.
type Inventory struct {
	Tool    string
	Version string
	Entries []domain.InventoryEntry
	Files   int
}

// InventoryFormats are the formats `pii-eu inventory` writes.
var InventoryFormats = []string{"json", "csv"}

type jsonInventory struct {
	SchemaVersion string           `json:"schema_version"`
	Tool          jsonTool         `json:"tool"`
	FilesScanned  int              `json:"files_scanned"`
	Entries       []jsonInventoryE `json:"entries"`
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

// WriteInventory writes the inventory as JSON or as CSV laid out like an Art. 30 record.
func WriteInventory(format string, w io.Writer, inv Inventory) error {
	switch format {
	case "json":
		out := jsonInventory{SchemaVersion: InventorySchemaVersion, Tool: jsonTool{Name: inv.Tool, Version: inv.Version}, FilesScanned: inv.Files, Entries: make([]jsonInventoryE, 0, len(inv.Entries))}
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
	}
	return fmt.Errorf("unknown inventory format %q: use json or csv", format)
}

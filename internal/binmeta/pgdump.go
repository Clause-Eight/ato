package binmeta

import (
	"bytes"
	"regexp"
	"strings"
)

// maxDumpTables caps the tables read from one pg_dump archive.
const maxDumpTables = 10_000

// DumpTable is a table whose rows a database dump holds, with its column names as the dump lists them.
type DumpTable struct {
	Name    string
	Columns []string
}

// IsPGDump reports whether data starts like a pg_dump custom-format archive (pg_dump -Fc).
func IsPGDump(data []byte) bool { return bytes.HasPrefix(data, []byte("PGDMP")) }

var pgCopy = regexp.MustCompile(`COPY ([^\s(]{1,200}) \(([^)]{1,1000})\) FROM stdin;`)

// PGDumpTables lists the tables a pg_dump custom-format archive holds data for. The archive's table of contents
// stores, for each table's data, the COPY statement that restores it as plain text, while the rows themselves
// are compressed; only the statements are read. A table listed here may still have no rows.
func PGDumpTables(data []byte) []DumpTable {
	if !IsPGDump(data) {
		return nil
	}
	var out []DumpTable
	seen := map[string]bool{}
	for _, m := range pgCopy.FindAllSubmatch(data, maxDumpTables) {
		name := string(m[1])
		if seen[name] {
			continue
		}
		seen[name] = true
		var cols []string
		for _, c := range strings.Split(string(m[2]), ",") {
			cols = append(cols, strings.Trim(strings.TrimSpace(c), `"`))
		}
		out = append(out, DumpTable{Name: name, Columns: cols})
	}
	return out
}

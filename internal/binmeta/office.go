package binmeta

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// Limits on office documents. A document is a zip file; only its metadata part is decompressed, and never more
// than maxMetaBytes of it, whatever the zip headers claim.
const (
	maxZipEntries = 10_000
	maxMetaBytes  = 1 << 20
	maxAuthorLen  = 200
	maxAuthors    = 16
)

// ErrNotOffice is returned for data that is not a zip file with office metadata.
var ErrNotOffice = errors.New("not an office document")

// Author is a name in a document's metadata and the property that holds it, such as dc:creator.
type Author struct {
	Name     string
	Property string
}

// Office Open XML (ECMA-376 part 2, core properties) keeps the author in docProps/core.xml; OpenDocument
// (ODF 1.2 part 1, 4.3) in meta.xml.
var metaParts = []string{"docProps/core.xml", "meta.xml"}

// authorProps are the metadata properties that hold a person: the creator, who saved it last, and the
// OpenDocument initial creator. Keys are the XML namespace and local name.
var authorProps = map[xml.Name]string{
	{Space: "http://purl.org/dc/elements/1.1/", Local: "creator"}:                                               "dc:creator",
	{Space: "http://schemas.openxmlformats.org/package/2006/metadata/core-properties", Local: "lastModifiedBy"}: "cp:lastModifiedBy",
	{Space: "urn:oasis:names:tc:opendocument:xmlns:meta:1.0", Local: "initial-creator"}:                         "meta:initial-creator",
}

// OfficeAuthors returns the author names in an office document's metadata, each name once, in document order.
// The body of the document is not read.
func OfficeAuthors(data []byte) ([]Author, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, ErrNotOffice
	}
	if len(zr.File) > maxZipEntries {
		return nil, errors.New("too many entries in the document")
	}
	for _, part := range metaParts {
		for _, f := range zr.File {
			if f.Name != part {
				continue
			}
			if f.Flags&0x1 != 0 {
				return nil, errors.New("encrypted document")
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			meta, err := io.ReadAll(io.LimitReader(rc, maxMetaBytes))
			rc.Close()
			if err != nil {
				return nil, err
			}
			return metaAuthors(meta), nil
		}
	}
	return nil, nil
}

// metaAuthors reads the author properties from a metadata part. The decoder does not fetch external entities
// or DTDs; a malformed part yields what was read before the error.
func metaAuthors(meta []byte) []Author {
	dec := xml.NewDecoder(bytes.NewReader(meta))
	dec.Strict = false
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	var out []Author
	seen := map[string]int{}
	prop := ""
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		switch t := tok.(type) {
		case xml.StartElement:
			prop = authorProps[t.Name]
			text.Reset()
		case xml.CharData:
			if prop != "" && text.Len() < maxAuthorLen {
				text.Write(t)
			}
		case xml.EndElement:
			if prop == "" {
				continue
			}
			name := strings.Join(strings.Fields(text.String()), " ")
			if name != "" && len(name) <= maxAuthorLen {
				if i, ok := seen[name]; ok {
					if !strings.Contains(out[i].Property, prop) {
						out[i].Property += ", " + prop
					}
				} else if len(out) < maxAuthors {
					seen[name] = len(out)
					out = append(out, Author{Name: name, Property: prop})
				}
			}
			prop = ""
		}
	}
}

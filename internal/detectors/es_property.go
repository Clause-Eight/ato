package detectors

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Spanish supply point code (código unificado de punto de suministro, CUPS): ES, the distributor's four digits,
// twelve digits for the supply point and two check letters, sometimes followed by a border point digit and letter.
// The sixteen digits mod 529, written as two base-23 digits, are read in TRWAGMYFPDXBNJZSQVHLCKE. One code is one
// household's electricity or gas meter, so it identifies the people living there.
// Source: Ministerio de Industria, Orden ITC/3860/2007, and https://es.wikipedia.org/wiki/Código_Unificado_de_Punto_de_Suministro
//
// Spanish cadastral reference (referencia catastral): twenty characters, fourteen for the parcel, four for the
// property within it and two check letters. Urban parcels are seven digits and the map sheet (two letters, four
// digits, a letter); rural parcels are the province and municipality (five digits), a sector letter, the polygon
// (three digits) and the parcel (five digits). Each check letter weighs a seven-character half of the parcel and
// the four property digits by 13, 15, 12, 5, 4, 17, 9, 21, 3, 7, 1 (letters counted from A = 1, Ñ after N), mod 23,
// read in MQWERTYUIOPASDFGHJKLBZX. It identifies one property and, through the land registry, its owner.
// Source: Dirección General del Catastro, https://www.catastro.hacienda.gob.es/documentos/05042010_P.pdf
func init() {
	Register(Detector{
		Name:        "es-cups",
		Label:       "Spanish energy supply point code (CUPS)",
		Country:     "ES",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`ES ?\d{4} ?\d{4} ?\d{4} ?\d{4} ?[A-Z]{2}(?: ?\d[FPRCXYZ])?`),
		Normalize:   func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:    validCUPS,
		Needs:       Needs{AnyOf: []string{"ES"}},
		Keywords:    []string{"cups", "código cups", "codigo cups", "punto de suministro", "código de suministro"},
		Concepts:    []Concept{Address},
		Base:        domain.Medium,
		Validation:  "distributor digits and both check letters are valid",
	})
	Register(Detector{
		Name:        "es-cadastral-reference",
		Label:       "Spanish cadastral reference (referencia catastral)",
		Country:     "ES",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{7} ?[A-Z]{2}\d{4}[A-Z] ?\d{4} ?[A-Z]{2}|\d{5}[A-Z]\d{8} ?\d{4} ?[A-Z]{2}`),
		Normalize:   func(s string) string { return strings.ReplaceAll(s, " ", "") },
		Validate:    validCadastralReference,
		Keywords:    []string{"referencia catastral", "ref. catastral", "ref catastral", "refcatastral", "catastro"},
		Concepts:    []Concept{Address},
		Base:        domain.Low,
		Validation:  "both check letters are valid",
	})
}

const cupsLetters = "TRWAGMYFPDXBNJZSQVHLCKE"

func validCUPS(s string) bool {
	if len(s) != 20 && len(s) != 22 || !strings.HasPrefix(s, "ES") {
		return false
	}
	n, err := strconv.ParseUint(s[2:18], 10, 64)
	if err != nil {
		return false
	}
	r := n % 529
	return s[18] == cupsLetters[r/23] && s[19] == cupsLetters[r%23]
}

const cadastralLetters = "MQWERTYUIOPASDFGHJKLBZX"

// cadastralAlphabet holds Ñ, so values are rune positions, not byte offsets.
var cadastralAlphabet = []rune("ABCDEFGHIJKLMNÑOPQRSTUVWXYZ")

var cadastralWeights = []int{13, 15, 12, 5, 4, 17, 9, 21, 3, 7, 1}

func validCadastralReference(s string) bool {
	if len(s) != 20 {
		return false
	}
	return s[18] == cadastralLetter(s[0:7]+s[14:18]) && s[19] == cadastralLetter(s[7:14]+s[14:18])
}

func cadastralLetter(part string) byte {
	sum := 0
	for i, c := range []rune(part) {
		v := 0
		switch {
		case c >= '0' && c <= '9':
			v = int(c - '0')
		default:
			v = slices.Index(cadastralAlphabet, c) + 1
		}
		sum += cadastralWeights[i] * v
	}
	return cadastralLetters[sum%23]
}

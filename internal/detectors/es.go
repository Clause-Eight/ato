package detectors

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Spanish national ID (DNI, also the NIF of Spanish citizens): 8 digits and a check letter. The letter is the
// number mod 23 looked up in TRWAGMYFPDXBNJZSQVHLCKE.
// Spanish foreigner ID (NIE): X, Y or Z, 7 digits and a check letter. X, Y and Z stand for 0, 1 and 2, and the
// letter is then computed as for the DNI. The two are separate detectors because they are separate documents
// with their own names (dni, nie), so a nearby word picks the right one.
// Source: Ministerio del Interior, "Cálculo del dígito de control del NIF/NIE"
// (https://www.interior.gob.es/opencms/es/servicios-al-ciudadano/tramites-y-gestiones/dni/calculo-del-digito-de-control-del-nif-nie/).
func init() {
	Register(Detector{
		Name:        "es-dni",
		Label:       "Spanish national ID (DNI)",
		Country:     "ES",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{8}[- ]?[A-Z]`),
		Normalize:   withoutDashesOrSpaces,
		Validate:    validDNI,
		Keywords:    []string{"dni", "d.n.i.", "nif", "documento nacional de identidad"},
		Concepts:    []Concept{NationalID, IDCard, TaxID},
		Base:        domain.Low,
		Validation:  "mod-23 check letter is valid",
	})
	Register(Detector{
		Name:        "es-nie",
		Label:       "Spanish foreigner ID (NIE)",
		Country:     "ES",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[XYZ][- ]?\d{7}[- ]?[A-Z]`),
		Normalize:   withoutDashesOrSpaces,
		Validate:    validNIE,
		Needs:       Needs{AnyOf: []string{"X", "Y", "Z"}},
		Keywords:    []string{"nie", "n.i.e.", "número de identidad de extranjero", "número de identificación de extranjero"},
		Concepts:    []Concept{NationalID},
		Base:        domain.Low,
		Validation:  "mod-23 check letter is valid",
	})
}

const dniLetters = "TRWAGMYFPDXBNJZSQVHLCKE"

func withoutDashesOrSpaces(s string) string {
	return strings.NewReplacer("-", "", " ", "").Replace(s)
}

func validDNI(s string) bool {
	if len(s) != 9 || DigitsOnly(s[:8]) != s[:8] {
		return false
	}
	n, err := strconv.Atoi(s[:8])
	return err == nil && dniLetters[n%23] == s[8]
}

func validNIE(s string) bool {
	if len(s) != 9 {
		return false
	}
	i := strings.IndexByte("XYZ", s[0])
	return i >= 0 && validDNI(strconv.Itoa(i)+s[1:])
}

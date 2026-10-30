package detectors

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

// Spanish national ID (DNI, also the NIF of Spanish citizens): 8 digits and a check letter. The letter is the
// number mod 23 looked up in TRWAGMYFPDXBNJZSQVHLCKE.
// Spanish foreigner ID (NIE): X, Y or Z, 7 digits and a check letter. X, Y and Z stand for 0, 1 and 2, and the
// letter is then computed as for the DNI. The two are separate detectors because they are separate documents
// with their own names (dni, nie), so a nearby word picks the right one.
// Source: Ministerio del Interior, "Cálculo del dígito de control del NIF/NIE"
// (https://www.interior.gob.es/opencms/es/servicios-al-ciudadano/tramites-y-gestiones/dni/calculo-del-digito-de-control-del-nif-nie/).
//
// Spanish social security number (número de la Seguridad Social, NAF/NUSS): 12 digits, written PP NNNNNNNN CC or
// PP/NNNNNNNN/CC. PP is the province where it was assigned (01-53, 66 abroad), NNNNNNNN a sequence and CC the
// remainder mod 97 of PP and NNNNNNNN read as one number; when the sequence is below 10,000,000 (older numbers)
// the number is PP * 10,000,000 + sequence instead.
// Source: Grupo Alquerque, "Dígitos de control del número de afiliación a la Seguridad Social"
// (http://www.grupoalquerque.es/ferias/2012/archivos/digitos/codigo_seguridad_social.pdf), and Intervia,
// "Validar números de la Seguridad Social" (https://intervia.com/doc/validar-numeros-de-la-seguridad-social/)
// for the rule below 10,000,000.
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
	Register(Detector{
		Name:        "es-nss",
		Label:       "Spanish social security number (NSS)",
		Country:     "ES",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{2}[ /-]?\d{8}[ /-]?\d{2}`),
		Normalize:   DigitsOnly,
		Validate:    validNSS,
		Formatted:   regexp.MustCompile(`^\d{2}[ /-]\d{8}[ /-]\d{2}$`),
		Keywords:    []string{"nss", "naf", "nuss", "número de la seguridad social", "número de afiliación", "seguridad social"},
		Concepts:    []Concept{SocialSecurity},
		Base:        domain.Low,
		Validation:  "province code and mod-97 check digits are valid",
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

func validNSS(s string) bool {
	if len(s) != 12 || DigitsOnly(s) != s {
		return false
	}
	province, _ := strconv.Atoi(s[:2])
	if !(province >= 1 && province <= 53 || province == 66) {
		return false
	}
	seq, _ := strconv.ParseInt(s[2:10], 10, 64)
	check, _ := strconv.Atoi(s[10:])
	n := int64(province)*100_000_000 + seq
	if seq < 10_000_000 {
		n = int64(province)*10_000_000 + seq
	}
	return n%97 == int64(check)
}

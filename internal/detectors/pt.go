package detectors

import (
	"regexp"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Portuguese tax number (NIF, número de identificação fiscal): nine digits, the last a mod-11 check digit.
// Digits 1-8 are weighted 9 down to 2; a remainder of 0 or 1 gives 0, otherwise the check is 11 minus the
// remainder. The first digits give the kind of holder (Decreto-Lei n.º 14/2013, artigo 4.º): 1, 2 and 3 are
// people, 45 is a non-resident person; 5, 6, 7, 8 and 9 are companies, public bodies, estates and funds. Only
// the personal ranges are accepted: company numbers are not personal data, and eu-vat finds them as PT VAT
// numbers. Sources: https://diariodarepublica.pt/dr/detalhe/decreto-lei/14-2013-257001 and
// https://info.portaldasfinancas.gov.pt/pt/destaques/Paginas/Atribuicao_Nova_Gama_de_NIF_a_Pessoas_Singulares.aspx
//
// Portuguese citizen card number (número de documento do Cartão de Cidadão): DDDDDDDD C AA T, the
// eight-digit civil ID number, its check digit, a two-character version (A-Z, 0-9) and a check digit over the
// whole number. The check is Luhn-like: from the right, every second element is doubled (letters count
// A=10 … Z=35, and a doubled value of 10 or more loses 9), and the sum must be divisible by 10. Source: AMA,
// "Validação de Número de Documento do Cartão de Cidadão", v1.0, 26 January 2009,
// https://www.autenticacao.gov.pt/documents/10179/11463/Valida%C3%A7%C3%A3o+de+N%C3%BAmero+de+Documento+do+Cart%C3%A3o+de+Cidad%C3%A3o/0dbc446b-3718-41e5-b982-551a72f8b8a8
// The pattern asks for at least one letter in the version, which every issued card has, so twelve plain
// digits are not taken for a card number.
func init() {
	Register(Detector{
		Name:        "pt-nif",
		Label:       "Portuguese tax number (NIF)",
		Country:     "PT",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`(?:[123]\d\d|45\d) ?\d{3} ?\d{3}`),
		Normalize:   DigitsOnly,
		Validate:    validPTNIF,
		Keywords:    []string{"número de identificação fiscal", "numero de identificacao fiscal", "contribuinte", "número de contribuinte", "numero de contribuinte", "nif_pt"},
		Concepts:    []Concept{TaxID},
		Base:        domain.Low,
		Validation:  "personal prefix and check digit are valid",
	})
	Register(Detector{
		Name:        "pt-citizen-card",
		Label:       "Portuguese citizen card number (Cartão de Cidadão)",
		Country:     "PT",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{8} ?\d ?(?:[A-Z][A-Z0-9]|\d[A-Z]) ?\d`),
		Normalize:   ptRemoveSpaces,
		Validate:    validPTCitizenCard,
		Formatted:   regexp.MustCompile(`^\d{8} \d [A-Z0-9]{2}\d$`),
		Keywords:    []string{"cartão de cidadão", "cartao de cidadao", "cartao cidadao", "citizen card", "citizen_card"},
		Concepts:    []Concept{IDCard, NationalID},
		Base:        domain.Medium,
		Samples:     map[string]bool{"000000000ZZ4": true},
		Validation:  "document check digit is valid",
	})
}

func validPTNIF(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 9 {
		return false
	}
	if d[0] < 1 || d[0] > 4 || d[0] == 4 && d[1] != 5 {
		return false
	}
	sum := 0
	for i := 0; i < 8; i++ {
		sum += d[i] * (9 - i)
	}
	check := 11 - sum%11
	if check >= 10 {
		check = 0
	}
	return check == d[8]
}

func validPTCitizenCard(s string) bool {
	if len(s) != 12 {
		return false
	}
	sum := 0
	for i := 0; i < 12; i++ {
		c := s[i]
		var v int
		switch {
		case c >= '0' && c <= '9':
			v = int(c - '0')
		case c >= 'A' && c <= 'Z':
			if i != 9 && i != 10 {
				return false
			}
			v = int(c-'A') + 10
		default:
			return false
		}
		if (11-i)%2 == 1 {
			v *= 2
			if v > 9 {
				v -= 9
			}
		}
		sum += v
	}
	return sum%10 == 0
}

func ptRemoveSpaces(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			b = append(b, s[i])
		}
	}
	return string(b)
}

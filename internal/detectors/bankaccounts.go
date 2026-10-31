package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Domestic bank account numbers, written without the IBAN country prefix, as forms and older systems still store
// them.
//
// Czech account number (číslo účtu): an optional prefix of up to six digits and a dash, a root of two to ten digits,
// a slash and the four-digit bank code. Prefix and root, padded to ten digits, each weighted 6,3,7,9,10,5,8,4,2,1
// sum to a multiple of 11; the bank code is one the Czech National Bank has assigned.
// Source: ČNB decree 169/2011 (https://www.cnb.cz/cs/platebni-styk/ucty-kody-bank/) and the bank code list,
// https://www.cnb.cz/cs/platebni-styk/.galleries/ucty_kody_bank/download/kody_bank_CR.csv
//
// Spanish account code (código cuenta cliente, CCC): bank (4), branch (4), two check digits and the account (10).
// The first check digit covers 00 + bank + branch, the second the account; each is the digits weighted 1, 2, 4, 8,
// 5, 10, 9, 7, 3, 6 mod 11, subtracted from 11, with 11 written 0 and 10 written 1.
// Source: Asociación Española de Banca, Cuaderno 34; https://es.wikipedia.org/wiki/Código_cuenta_cliente
//
// Norwegian account number (kontonummer): eleven digits printed 4.2.5; the last is 11 minus the first ten weighted
// 5,4,3,2,7,6,5,4,3,2 mod 11, written 0 for 11, and 10 is never issued.
// Source: https://www.ecbs.org/iban/norway-bank-account-number.html (MOD11 as in the Norwegian banks' standard).
//
// Polish account number (numer rachunku bankowego, NRB): 26 digits, the IBAN without PL, printed as two digits and
// six groups of four. The first two digits are the IBAN check digits, so PL in front passes mod 97.
// Source: NBP regulation on the NRB and IBAN (Dz.U. 2020 poz. 2238), https://www.nbp.pl/
func init() {
	Register(Detector{
		Name:        "cz-bank-account",
		Label:       "Czech bank account number (číslo účtu)",
		Country:     "CZ",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`(?:\d{1,6}-)?\d{2,10}/\d{4}`),
		Validate:    validCZBankAccount,
		Keywords:    []string{"číslo účtu", "cislo uctu", "č. ú.", "č.ú.", "bankovní účet", "bankovni ucet"},
		Concepts:    []Concept{BankAccount},
		Base:        domain.Low,
		Validation:  "mod-11 checks of prefix and root and an assigned bank code are valid",
	})
	Register(Detector{
		Name:        "es-ccc",
		Label:       "Spanish bank account code (código cuenta cliente, CCC)",
		Country:     "ES",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{4}[ -]?\d{4}[ -]?\d{2}[ -]?(?:\d{10}|\d{5}[ -]\d{5})`),
		Normalize:   DigitsOnly,
		Validate:    validESCCC,
		Formatted:   regexp.MustCompile(`^\d{4}[ -]\d{4}[ -]\d{2}[ -]\d{5}[ -]?\d{5}$`),
		Keywords:    []string{"ccc", "código cuenta cliente", "codigo cuenta cliente", "cuenta corriente", "número de cuenta", "numero de cuenta", "nº de cuenta"},
		Concepts:    []Concept{BankAccount},
		Base:        domain.Low,
		Validation:  "both mod-11 check digits are valid",
	})
	Register(Detector{
		Name:        "no-kontonr",
		Label:       "Norwegian bank account number (kontonummer)",
		Country:     "NO",
		OnByDefault: false,
		Pattern:     regexp.MustCompile(`\d{4}[. ]?\d{2}[. ]?\d{5}`),
		Normalize:   DigitsOnly,
		Validate:    validNOKontonr,
		Formatted:   regexp.MustCompile(`^\d{4}\.\d{2}\.\d{5}$`),
		Keywords:    []string{"kontonummer", "kontonr", "konto nr", "bankkonto"},
		Concepts:    []Concept{BankAccount},
		Base:        domain.Low,
		Validation:  "mod-11 check digit is valid",
	})
	Register(Detector{
		Name:        "pl-nrb",
		Label:       "Polish bank account number (NRB)",
		Country:     "PL",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{2}(?: ?\d{4}){6}`),
		Normalize:   DigitsOnly,
		Validate:    validPLNRB,
		Formatted:   regexp.MustCompile(`^\d{2}(?: \d{4}){6}$`),
		Keywords:    []string{"nrb", "numer rachunku", "nr rachunku", "rachunek bankowy", "numer konta", "nr konta"},
		Concepts:    []Concept{BankAccount},
		Base:        domain.Low,
		Validation:  "IBAN mod-97 check digits are valid",
	})
}

// czBankCodes are the bank codes in the Czech National Bank's list.
var czBankCodes = map[string]bool{}

func init() {
	for _, c := range strings.Fields(`0100 0300 0600 0710 0800 2010 2020 2060 2070 2100 2200 2220 2250 2260 2275 2600
		2700 3030 3050 3060 3500 4000 4300 5500 5800 6000 6100 6200 6210 6300 6700 6800 7910 7950 7960 7970 7990 8030
		8040 8060 8090 8150 8190 8198 8199 8200 8220 8230 8240 8250 8255 8265 8270 8280 8293 8299 8500`) {
		czBankCodes[c] = true
	}
}

var czAccountWeights = []int{6, 3, 7, 9, 10, 5, 8, 4, 2, 1}

func validCZBankAccount(s string) bool {
	number, bank, ok := strings.Cut(s, "/")
	if !ok || !czBankCodes[bank] {
		return false
	}
	prefix, root, hasPrefix := strings.Cut(number, "-")
	if !hasPrefix {
		prefix, root = "", number
	}
	return strings.Trim(root, "0") != "" && czAccountPart(prefix) && czAccountPart(root)
}

func czAccountPart(s string) bool {
	d, ok := Digits(strings.Repeat("0", 10-len(s)) + s)
	if !ok {
		return false
	}
	sum := 0
	for i, w := range czAccountWeights {
		sum += w * d[i]
	}
	return sum%11 == 0
}

func validESCCC(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 20 {
		return false
	}
	return d[8] == cccDigit(append([]int{0, 0}, d[:8]...)) && d[9] == cccDigit(d[10:])
}

func cccDigit(d []int) int {
	sum, w := 0, 1
	for _, x := range d {
		sum += x * w
		w = w * 2 % 11
	}
	switch c := 11 - sum%11; c {
	case 11:
		return 0
	case 10:
		return 1
	default:
		return c
	}
}

var noAccountWeights = []int{5, 4, 3, 2, 7, 6, 5, 4, 3, 2}

func validNOKontonr(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 11 {
		return false
	}
	sum := 0
	for i, w := range noAccountWeights {
		sum += w * d[i]
	}
	c := (11 - sum%11) % 11
	return c != 10 && c == d[10]
}

func validPLNRB(s string) bool {
	return len(s) == 26 && validIBAN("PL"+s[:2]+s[2:])
}

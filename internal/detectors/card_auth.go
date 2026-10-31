package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Card security codes and expiry dates next to a card number. PCI DSS v4.0 requirement 3.3.1 forbids storing
// the card verification code (CVV2, CVC2, CID, CAV2) after authorisation, even encrypted; the expiry date is
// account data to protect under requirement 3.5 (https://www.pcisecuritystandards.org/document_library/).
// A security code is three digits (four on American Express, printed as the CID), so a number alone says
// nothing: it is reported only under its own label (cvv, cvc, cid, security code and their names in other
// languages) with a valid card number on its line, in its row or in the lines just above. An expiry date is
// MM/YY, MM/YYYY or MMYY under an expiry label (exp, expiry, valid thru), with the same card number nearby.
// A code next to a published test card, or one written by ato fix, is reported as sample data.
func init() {
	Register(Detector{
		Name:        "card-security-code",
		Label:       "Card security code (CVV/CVC)",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`\d{3,4}`),
		Validate:    func(s string) bool { return len(s) == 3 || len(s) == 4 },
		Keywords:    []string{"cvv", "cvv2", "cvc", "cvc2", "cvn", "cid", "csc", "ccv", "cvd", "security_code", "card_verification", "card_code", "cardcode", "securitycode"},
		Base:        domain.High,
		Validation:  "three or four digits under a card security code label, next to a valid card number",
		Needs:       Needs{Check: maybeSecurityCode},
		Context:     securityCodeContext,
	})
	Register(Detector{
		Name:        "card-expiry",
		Label:       "Card expiry date",
		Country:     "EU",
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`(?:0[1-9]|1[0-2])(?:/(?:20\d{2}|\d{2})|\d{2})`),
		Validate:    func(s string) bool { return len(s) >= 4 },
		Keywords:    []string{"exp", "expiry", "expires", "expiration", "exp_date", "expiry_date", "expiration_date", "expdate", "expirydate", "expirationdate", "valid_thru", "valid_until", "card_expiry", "cc_exp", "ccexp", "ablaufdatum", "gültig bis", "scadenza", "vencimiento", "caducidad", "lejárat", "ważności", "platnost do", "vervaldatum", "geldig tot"},
		Base:        domain.Low,
		Validation:  "a month and year under a card expiry label, next to a valid card number",
		Needs:       Needs{Check: maybeExpiry},
		Context:     expiryContext,
	})
}

// securityCodeLabels are card security code labels, folded and written together.
var securityCodeLabels = words("cvv", "cvv2", "cvc", "cvc2", "cvn", "cvn2", "cid", "csc", "ccv", "cvd", "cav2",
	"securitycode", "cardsecuritycode", "cardverificationvalue", "cardverificationcode", "cardverificationnumber",
	"cardcode", "cardcvv", "cardcvc", "cccvv", "cccvc", "kartenprufnummer", "prufnummer", "cryptogramme",
	"cryptogrammevisuel", "codicedisicurezza", "codicesicurezza", "codigodeseguridad", "codigoseguridad",
	"codigodeseguranca", "beveiligingscode", "kodbezpieczenstwa", "biztonsagikod", "codsecuritate", "coddesecuritate")

// expiryLabels are card expiry labels, folded and written together.
var expiryLabels = words("exp", "expiry", "expires", "expiration", "expdate", "expirydate", "expirationdate",
	"ccexp", "ccexpiry", "cardexp", "cardexpiry", "cardexpiration", "validthru", "validthrough", "validuntil",
	"validto", "goodthru", "ablaufdatum", "gultigbis", "scadenza", "datascadenza", "vencimiento", "fechavencimiento",
	"caducidad", "fechacaducidad", "dateexpiration", "lejarat", "waznosc", "datawaznosci", "platnostdo",
	"dataexpirarii", "vervaldatum", "geldigtot")

// securityCodeStems are what a line must hold, ignoring case, before the security code pattern runs on it; a
// short text is a cell of a file with columns, whose label is its column name.
var securityCodeStems = []string{"cvv", "cvc", "cid", "csc", "cvn", "ccv", "cvd", "cav2", "secur", "verif", "cardcode", "card_code", "pruf", "prüf", "crypto", "sicurezza", "segur", "beveilig", "bezpiecz", "biztons"}

var expiryStems = []string{"exp", "valid", "thru", "ablauf", "gultig", "gültig", "scaden", "vencim", "caduc", "lejar", "lejár", "wazn", "ważn", "platnost", "verval", "geldig"}

func maybeSecurityCode(line string) bool {
	return len(line) <= 12 || containsFold(line, securityCodeStems)
}

func maybeExpiry(line string) bool {
	return len(line) <= 12 || containsFold(line, expiryStems)
}

func securityCodeContext(a Around) Fit {
	if !keyOrColumn(a, securityCodeLabels) {
		return Unfit
	}
	return cardInRecord(a.Record)
}

func expiryContext(a Around) Fit {
	// 12/27 at the start of 12/27/2026 or 2026/12/27 is a date, not an expiry.
	if strings.HasSuffix(a.Before, "/") || len(a.After) > 0 && strings.ContainsRune("/.-", rune(a.After[0])) && len(a.After) > 1 && isASCIIDigit(a.After[1]) {
		return Unfit
	}
	if !keyOrColumn(a, expiryLabels) {
		return Unfit
	}
	return cardInRecord(a.Record)
}

// recordCards finds card numbers the way the card detector does, longest first.
var recordCards = func() *regexp.Regexp {
	re := regexp.MustCompile(cardPattern.String())
	re.Longest()
	return re
}()

// cardInRecord says whether text holds a valid card number: Fits for a real one, FitsSample when every one is
// a published test number or carries the synthetic mark of ato fix.
func cardInRecord(text string) Fit {
	fit := Unfit
	for _, loc := range recordCards.FindAllStringIndex(text, -1) {
		start, end := loc[0], loc[1]
		if start > 0 && isASCIIAlnum(text[start-1]) {
			continue
		}
		for end > start {
			if end == len(text) || !isASCIIAlnum(text[end]) {
				if v := DigitsOnly(text[start:end]); CardBrand(v) != "" {
					if !sampleCards[v] && !Synthetic(text[start:end]) {
						return Fits
					}
					fit = FitsSample
					break
				}
			}
			cut := strings.LastIndexAny(text[start:end], " -")
			if cut <= 0 {
				break
			}
			end = start + cut
		}
	}
	return fit
}

// words makes a set of folded labels.
func words(ws ...string) map[string]bool {
	out := make(map[string]bool, len(ws))
	for _, w := range ws {
		out[w] = true
	}
	return out
}

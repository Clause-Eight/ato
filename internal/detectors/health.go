package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Diagnosis and medicine codes are not identifiers: "E11" or "A01AA01" alone names a disease or a substance, not
// a person, and codes of that shape fill seat maps, part numbers and hex dumps. They are health data (GDPR
// Art. 9) only in a record about someone, so each of these detectors needs both a medical word next to the code
// (its column name, in a file with columns) and a person in the same record (NeedsPerson). See ADR 13.
//
// ICD-10 (WHO, International Statistical Classification of Diseases and Related Health Problems, 10th revision,
// 2019 edition, https://icd.who.int/browse10/2019/en): a category is a chapter letter and two digits, optionally
// followed by a point and a subcategory digit; national modifications such as ICD-10-GM add a fifth character.
// The chapters cover A00-B99, C00-D48, D50-D89, E00-E90, F00-F99, G00-G99, H00-H95, I00-I99, J00-J99, K00-K93,
// L00-L99, M00-M99, N00-N99, O00-O99, P00-P96, Q00-Q99, R00-R99, S00-T98, V01-Y98, Z00-Z99 and the codes for
// special purposes U00-U85, so the letter and the number together must fall inside one of them.
//
// ICD-11 (WHO, ICD-11 Reference Guide, section 2.9 "Code structure", https://icd.who.int/icd11refguide/en): a stem
// code is four characters, a chapter character (1-9 for chapters 1 to 9, then A-H, J-N and P-S for chapters 10 to
// 26), a letter so it cannot be read as an ICD-10 code, a digit and a letter or digit, optionally followed by a
// point and one or two more characters. The letters I and O are never used. Extension codes (chapter X) and
// functioning (V) are not diagnoses and are left out.
//
// ATC (WHO Collaborating Centre for Drug Statistics Methodology, Guidelines for ATC classification and DDD
// assignment, https://atcddd.fhi.no/atc_ddd_index_and_guidelines/guidelines/): the fifth level, a chemical
// substance, is seven characters: an anatomical main group letter (A, B, C, D, G, H, J, L, M, N, P, R, S or V),
// two digits for the therapeutic subgroup, two letters for the pharmacological and chemical subgroups and two
// digits for the substance.
func init() {
	diagnosisWords := []string{"icd", "diagnosis", "diagnoses", "diagnose", "diagnosi", "diagnostico", "diagnoza",
		"diagnózis", "diagnoos", "diagnozė", "diagnoosi", "diagnóza", "диагноза", "διάγνωση", "mkb", "мкб"}
	Register(Detector{
		Name:         "icd10",
		Label:        "ICD-10 diagnosis code in a record about a person (health data, GDPR Art. 9)",
		Country:      domain.CrossBorder,
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`[A-Z]\d{2}(?:\.\d{1,2})?`),
		Validate:     validICD10,
		Keywords:     diagnosisWords,
		Base:         domain.Medium,
		Validation:   "a chapter letter and two digits inside an ICD-10 chapter's range",
		Needs:        Needs{Check: letterThenDigit},
		NeedsKeyword: true,
		NeedsPerson:  true,
		Special:      domain.SpecialHealth,
	})
	Register(Detector{
		Name:         "icd11",
		Label:        "ICD-11 diagnosis code in a record about a person (health data, GDPR Art. 9)",
		Country:      domain.CrossBorder,
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`[1-9A-HJ-NP-S][A-HJ-NP-Z]\d[0-9A-HJ-NP-Z](?:\.[0-9A-HJ-NP-Z]{1,2})?`),
		Validate:     validICD11,
		Keywords:     diagnosisWords,
		Base:         domain.Medium,
		Validation:   "a chapter, a letter, a digit and a fourth character in the ICD-11 stem code layout",
		Needs:        Needs{Check: letterThenDigit},
		NeedsKeyword: true,
		NeedsPerson:  true,
		Special:      domain.SpecialHealth,
	})
	Register(Detector{
		Name:        "atc-code",
		Label:       "ATC medicine code in a record about a person (health data, GDPR Art. 9)",
		Country:     domain.CrossBorder,
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`[ABCDGHJLMNPRSV]\d{2}[A-Z]{2}\d{2}`),
		Validate:    validATC,
		Keywords: []string{"atc", "medication", "medications", "medicine", "medicines", "drug", "drugs", "prescription", "rx",
			"farmaco", "medicinale", "medicamento", "médicament", "medicament", "arzneimittel", "wirkstoff", "geneesmiddel", "läkemedel",
			"lægemiddel", "lek", "leki", "lék", "liek", "gyógyszer", "lääke", "ravim", "лекарство", "φάρμακο"},
		Base:         domain.Medium,
		Validation:   "an ATC anatomical group, two digits, two letters and two digits",
		Needs:        Needs{Check: letterThenDigit},
		NeedsKeyword: true,
		NeedsPerson:  true,
		Special:      domain.SpecialHealth,
	})
}

// letterThenDigit is the prefilter of the code detectors: an upper-case letter next to a digit.
func letterThenDigit(line string) bool {
	for i := 1; i < len(line); i++ {
		a, b := line[i-1], line[i]
		if a >= 'A' && a <= 'Z' && isASCIIDigit(b) || isASCIIDigit(a) && b >= 'A' && b <= 'Z' {
			return true
		}
	}
	return false
}

// icd10Last is the highest category number of each ICD-10 letter; icd10First is the lowest where it is not 00.
var (
	icd10Last  = map[byte]int{'C': 97, 'E': 90, 'H': 95, 'K': 93, 'P': 96, 'T': 98, 'U': 85, 'Y': 98}
	icd10First = map[byte]int{'V': 1}
)

func validICD10(s string) bool {
	if len(s) < 3 || s[0] < 'A' || s[0] > 'Z' || !isASCIIDigit(s[1]) || !isASCIIDigit(s[2]) {
		return false
	}
	n := int(s[1]-'0')*10 + int(s[2]-'0')
	if s[0] == 'D' {
		return n <= 48 || n >= 50 && n <= 89
	}
	last, ok := icd10Last[s[0]]
	if !ok {
		last = 99
	}
	return n >= icd10First[s[0]] && n <= last
}

func validICD11(s string) bool {
	if len(s) < 4 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] == 'I' || s[i] == 'O' {
			return false
		}
	}
	return s[0] != 'X' && s[0] != 'V' && s[1] >= 'A' && s[1] <= 'Z' && isASCIIDigit(s[2])
}

// atcGroups are the anatomical main groups of the ATC classification.
const atcGroups = "ABCDGHJLMNPRSV"

func validATC(s string) bool {
	if len(s) != 7 || !strings.ContainsRune(atcGroups, rune(s[0])) {
		return false
	}
	for i, c := range []byte(s[1:]) {
		letter := i == 2 || i == 3
		if letter && (c < 'A' || c > 'Z') || !letter && !isASCIIDigit(c) {
			return false
		}
	}
	return s[1:3] != "00"
}

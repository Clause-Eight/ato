// Package art9 finds records and files whose structure says they hold special categories of personal data
// (GDPR Art. 9) about someone: HL7 v2 patient segments and FHIR resources about a patient (health), VCF files
// and raw genotype data (genetic), and biometric templates such as face embeddings. Unlike a detector, which
// confirms one value, each check reads the shape of a whole record: a patient segment with a name and an
// identifier, a resource whose subject is named, a file of genotype calls. It works line by line, keeps state
// across lines and reads no file whole. See ADR 13.
package art9

import (
	"strings"
	"unicode"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Check is one kind of record, switched on and off in settings like a detector. When says what is reported.
type Check struct {
	Detector string
	Label    string
	Special  domain.SpecialCategory
	When     string
}

// The checks' detector names.
const (
	HL7       = "hl7-patient"
	FHIR      = "fhir-patient"
	VCF       = "vcf-genotypes"
	Genotypes = "rsid-genotypes"
	Biometric = "biometric-template"
)

// Checks lists the record checks.
var Checks = []Check{
	{HL7, "HL7 v2 patient segment (health data, GDPR Art. 9)", domain.SpecialHealth,
		"a PID segment names the patient (a family name) and identifies them by an identifier (PID-2, PID-3, PID-19) or a date of birth (PID-7)"},
	{FHIR, "FHIR resource about an identified patient (health data, GDPR Art. 9)", domain.SpecialHealth,
		"a Patient resource has a name and a birth date or an identifier, or a clinical resource (Observation, Condition, Encounter and others) names or identifies its subject"},
	{VCF, "VCF file with genotypes of samples (genetic data, GDPR Art. 9)", domain.SpecialGenetic,
		"a file starting with ##fileformat=VCFv4 has sample columns and at least one called genotype"},
	{Genotypes, "Genotype calls by rsID (genetic data, GDPR Art. 9)", domain.SpecialGenetic,
		"five or more lines of a genotyping export (rsID, chromosome, position, genotype), or three or more rsID-genotype pairs, or one with the word genotype nearby"},
	{Biometric, "Biometric template (biometric data, GDPR Art. 9)", domain.SpecialBiometric,
		"a field such as face_embedding, fingerprint_template, minutiae, iris_code or voiceprint is given 64 numbers or more, or an encoded string of 100 characters or more"},
}

func checkFor(name string) Check {
	for _, c := range Checks {
		if c.Detector == name {
			return c
		}
	}
	return Check{}
}

// Hit is one record found. Line and Previous are the text of its line and the line before, for ignore comments;
// they hold the record's values and must not reach a report. The finding has no fingerprint, and its confidence
// does not yet account for test paths: the engine adds both.
type Hit struct {
	Finding        domain.Finding
	Line, Previous string
}

// Scanner reads one file line by line.
type Scanner struct {
	path     string
	previous string
	hl7      bool
	fhir     *fhirScanner
	vcf      *vcfScanner
	geno     *genotypeScanner
	bio      *biometricScanner
}

// NewScanner returns a scanner for the file at path that runs the named checks.
func NewScanner(path string, enabled []string) *Scanner {
	s := &Scanner{path: path}
	for _, name := range enabled {
		switch name {
		case HL7:
			s.hl7 = true
		case FHIR:
			s.fhir = &fhirScanner{}
		case VCF:
			s.vcf = &vcfScanner{}
		case Genotypes:
			s.geno = &genotypeScanner{}
		case Biometric:
			s.bio = &biometricScanner{}
		}
	}
	return s
}

// Line returns the records found by the time line number has been read; a record that spans lines is returned
// on its last line, with the location of its first.
func (s *Scanner) Line(number int, line string) []Hit {
	var out []Hit
	at := place{line: number, text: line, previous: s.previous}
	if s.hl7 {
		out = append(out, s.pid(at)...)
	}
	if s.fhir != nil {
		out = append(out, s.fhir.line(s, at)...)
	}
	if s.vcf != nil {
		out = append(out, s.vcf.line(s, at)...)
	}
	if s.geno != nil {
		s.geno.line(at)
	}
	if s.bio != nil {
		out = append(out, s.bio.line(s, at)...)
	}
	s.previous = line
	return out
}

// End returns the records only known once the whole file is read.
func (s *Scanner) End() []Hit {
	var out []Hit
	if s.fhir != nil {
		out = append(out, s.fhir.end(s)...)
	}
	if s.geno != nil {
		out = append(out, s.geno.end(s)...)
	}
	if s.bio != nil {
		out = append(out, s.bio.end(s)...)
	}
	return out
}

// place is where a record starts: a line number, its text and the line before.
type place struct {
	line     int
	column   int
	text     string
	previous string
}

// hit builds a record finding. A sample record, such as one with the name of a specification's example
// patient, is reported at low confidence with the reason.
func (s *Scanner) hit(name string, at place, masked string, reasons []string, sample string) Hit {
	c := checkFor(name)
	f := domain.Finding{
		Kind:       domain.KindRecord,
		Category:   domain.CategorySpecial,
		Special:    c.Special,
		Detector:   c.Detector,
		Label:      c.Label,
		Country:    domain.CrossBorder,
		Location:   domain.Location{Path: s.path, Line: at.line, Column: max(at.column, 1)},
		Masked:     masked,
		Confidence: domain.High,
		Reasons:    reasons,
	}
	if sample != "" {
		f.Sample = true
		f.Confidence = domain.Low
		f.Reasons = append(f.Reasons, sample)
	}
	return Hit{Finding: f, Line: at.text, Previous: at.previous}
}

// placeholders are names used in examples rather than for people: the HL7 and FHIR specifications' example
// patients, the usual stand-ins for an unknown person in several languages, and words that name the field.
var placeholders = map[string]bool{
	"doe": true, "everyman": true, "everywoman": true, "chalmers": true, "duck": true, "mustermann": true, "musterfrau": true,
	"test": true, "tester": true, "testpatient": true, "patient": true, "sample": true, "example": true, "demo": true,
	"dummy": true, "fake": true, "anonymous": true, "unknown": true, "xxx": true, "family": true, "given": true,
	"name": true, "surname": true, "lastname": true, "firstname": true, "last": true, "first": true,
}

// placeholderName reports whether a name is made up: a placeholder, or a name with digits in it, as generated
// test data such as Synthea's writes them.
func placeholderName(name string) bool {
	for _, w := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if placeholders[w] {
			return true
		}
	}
	return strings.IndexFunc(name, unicode.IsDigit) >= 0
}

// hasLetters reports whether s holds at least n letters.
func hasLetters(s string, n int) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			if n--; n <= 0 {
				return true
			}
		}
	}
	return false
}

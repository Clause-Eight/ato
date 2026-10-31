package art9

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Every name, identifier and date below is made up.

func scan(path, text string) []domain.Finding {
	s := NewScanner(path, []string{HL7, FHIR, VCF, Genotypes, Biometric})
	var out []domain.Finding
	for i, line := range strings.Split(text, "\n") {
		for _, h := range s.Line(i+1, line) {
			out = append(out, h.Finding)
		}
	}
	for _, h := range s.End() {
		out = append(out, h.Finding)
	}
	return out
}

func expect(t *testing.T, name string, fs []domain.Finding, detector string, line int, sample bool) {
	t.Helper()
	if len(fs) != 1 {
		t.Fatalf("%s: got %d findings: %+v", name, len(fs), fs)
	}
	f := fs[0]
	if f.Detector != detector || f.Location.Line != line || f.Sample != sample || f.Kind != domain.KindRecord || f.Category != domain.CategorySpecial || f.Special == "" {
		t.Fatalf("%s: got %+v", name, f)
	}
	if want := domain.High; !sample && f.Confidence != want || sample && f.Confidence != domain.Low {
		t.Fatalf("%s: confidence %s", name, f.Confidence)
	}
}

func none(t *testing.T, name string, fs []domain.Finding) {
	t.Helper()
	if len(fs) != 0 {
		t.Fatalf("%s: got %+v", name, fs)
	}
}

const pidLine = "PID|1||48213^^^HOSP^MR||Varnell^Odile||19750412|F"

func TestHL7PatientSegments(t *testing.T) {
	msg := "MSH|^~\\&|APP|FAC|||20240101||ADT^A01|1|P|2.5\r" + pidLine + "\rPV1|1|I\r"
	fs := scan("data/admit.hl7", msg)
	expect(t, "message with carriage returns", fs, HL7, 1, false)
	if fs[0].Location.Column != strings.Index(msg, "PID")+1 || fs[0].Masked != "PID segment" {
		t.Fatalf("location or mask: %+v", fs[0])
	}
	if !strings.Contains(strings.Join(fs[0].Reasons, " "), "identifier") || !strings.Contains(strings.Join(fs[0].Reasons, " "), "date of birth") {
		t.Fatalf("reasons %v", fs[0].Reasons)
	}

	src := `msg := "MSH|^~\\&|APP|FAC|||20240101||ADT^A01|1|P|2.5\r` + pidLine + `\r"`
	expect(t, "message in a string literal", scan("app/adt.go", src), HL7, 1, false)
	expect(t, "segment on its own line", scan("data/feed.txt", "MSH|^~\\&|A\n"+pidLine+"\n"), HL7, 2, false)
	expect(t, "name and birth date only", scan("data/feed.txt", "PID|1||||Varnell^Odile||19750412"), HL7, 1, false)
	expect(t, "placeholder name", scan("data/feed.txt", "PID|1||123||DOE^JOHN||19800101"), HL7, 1, true)

	none(t, "name only", scan("data/feed.txt", "PID|1||||Varnell^Odile"))
	none(t, "identifier and birth date without a name", scan("data/feed.txt", "PID|1||48213||^||19750412"))
	none(t, "PID inside a longer word", scan("app/x.txt", "XPID|1||48213||Varnell^Odile||19750412"))
	none(t, "prose about PID segments", scan("docs/hl7.md", "The PID segment carries PID-5, the patient name."))
}

const patientJSON = `{
  "resourceType": "Patient",
  "id": "p1",
  "identifier": [{"system": "urn:example:mrn", "value": "48213"}],
  "name": [{"family": "Varnell", "given": ["Odile"]}],
  "birthDate": "1975-04-12"
}`

func TestFHIRPatients(t *testing.T) {
	fs := scan("fhir/patient.json", patientJSON)
	expect(t, "pretty patient", fs, FHIR, 2, false)
	if fs[0].Masked != "Patient resource" || fs[0].Location.Column != 3 {
		t.Fatalf("mask or column: %+v", fs[0])
	}
	ndjson := `{"resourceType":"Patient","name":[{"text":"Odile Varnell"}],"identifier":[{"value":"48213"}]}` + "\n" +
		`{"resourceType":"Patient","name":[{"family":"Brannock"}],"birthDate":"1981"}`
	if fs := scan("fhir/patients.ndjson", ndjson); len(fs) != 2 || fs[0].Location.Line != 1 || fs[1].Location.Line != 2 {
		t.Fatalf("ndjson: %+v", fs)
	}
	escaped := `body := "{\"resourceType\": \"Patient\", \"name\": [{\"family\": \"Varnell\"}], \"birthDate\": \"1975-04-12\"}"`
	expect(t, "json in a string literal", scan("app/client_test.go", escaped), FHIR, 1, false)

	chalmers := strings.Replace(patientJSON, "Varnell", "Chalmers", 1)
	chalmers = strings.Replace(chalmers, "Odile", "Peter", 1)
	expect(t, "the specification's example patient", scan("fhir/patient.json", chalmers), FHIR, 2, true)
	generated := strings.Replace(strings.Replace(patientJSON, "Varnell", "Varnell482", 1), "Odile", "Odile17", 1)
	expect(t, "a generated name", scan("fhir/patient.json", generated), FHIR, 2, true)

	none(t, "name only", scan("fhir/p.json", `{"resourceType": "Patient", "name": [{"family": "Varnell"}]}`))
	none(t, "birth date and identifier without a name", scan("fhir/p.json", `{"resourceType": "Patient", "identifier": [{"value": "1"}], "birthDate": "1975-04-12"}`))
	none(t, "a practitioner", scan("fhir/p.json", `{"resourceType": "Practitioner", "name": [{"family": "Varnell"}], "birthDate": "1975-04-12"}`))
	none(t, "a json schema mentioning resourceType", scan("schema.json", `{"properties": {"resourceType": {"type": "string"}, "name": {"type": "array"}}}`))
}

func TestFHIRClinicalResources(t *testing.T) {
	bundle := `{
  "resourceType": "Bundle",
  "entry": [
    {"resource": {"resourceType": "Observation", "code": {"text": "glucose"}, "subject": {"display": "Odile Varnell"}}},
    {"resource": {"resourceType": "Condition", "subject": {"identifier": {"value": "48213"}}}},
    {"resource": {"resourceType": "Observation", "subject": {"reference": "Patient/p1"}}}
  ]
}`
	fs := scan("fhir/bundle.json", bundle)
	if len(fs) != 2 || fs[0].Masked != "Observation resource" || fs[0].Location.Line != 4 || fs[1].Masked != "Condition resource" || fs[1].Location.Line != 5 {
		t.Fatalf("bundle: %+v", fs)
	}
}

func TestVCFGenotypes(t *testing.T) {
	head := "##fileformat=VCFv4.2\n##source=test\n#CHROM\tPOS\tID\tREF\tALT\tQUAL\tFILTER\tINFO\tFORMAT\t"
	vcf := head + "S001\tS002\n1\t1000\trs1000001\tA\tG\t50\tPASS\t.\tGT:DP\t0/1:20\t./.:0\n"
	fs := scan("data/calls.vcf", vcf)
	expect(t, "samples with calls", fs, VCF, 3, false)
	if fs[0].Masked != "VCF header (2 samples)" {
		t.Fatalf("mask %q", fs[0].Masked)
	}
	expect(t, "public reference samples", scan("data/calls.vcf", head+"NA12878\n1\t1000\t.\tA\tG\t50\tPASS\t.\tGT\t1|1\n"), VCF, 3, true)

	none(t, "sites only", scan("data/sites.vcf", "##fileformat=VCFv4.2\n#CHROM\tPOS\tID\tREF\tALT\tQUAL\tFILTER\tINFO\n1\t1000\trs1000001\tA\tG\t50\tPASS\t.\n"))
	none(t, "no calls made", scan("data/calls.vcf", head+"S001\n1\t1000\t.\tA\tG\t50\tPASS\t.\tGT\t./.\n"))
	none(t, "no file format line", scan("data/calls.txt", strings.TrimPrefix(vcf, "##fileformat=VCFv4.2\n")))
}

func TestGenotypeCalls(t *testing.T) {
	var export strings.Builder
	export.WriteString("# rsid\tchromosome\tposition\tgenotype\n")
	for i := 0; i < 6; i++ {
		fmt.Fprintf(&export, "rs%d\t%d\t%d\t%s\n", 1000001+i, 1+i, 50000+i*731, []string{"AA", "AG", "CT", "GG", "--", "TT"}[i])
	}
	fs := scan("data/genome.txt", export.String())
	expect(t, "genotyping export", fs, Genotypes, 2, false)
	if !strings.Contains(fs[0].Masked, "5 calls") {
		t.Fatalf("mask %q", fs[0].Masked)
	}
	csv := `"rs1000001","1","50000","A","G"` + "\n" + strings.Repeat(`"rs1000002","2","50731","C","T"`+"\n", 5)
	expect(t, "export with two allele columns", scan("data/genome.csv", csv), Genotypes, 1, false)

	expect(t, "calls as keys", scan("app/profile.json", `{"rs1000001": "AG", "rs1000002": "GG", "rs1000003": "CT"}`), Genotypes, 1, false)
	expect(t, "one call with the word genotype", scan("app/user.yml", "genotype:\n  rs1000001: AG\n"), Genotypes, 2, false)

	none(t, "short export", scan("data/genome.txt", "rs1000001\t1\t50000\tAA\nrs1000002\t1\t50100\tAG\n"))
	none(t, "one call without the word", scan("app/user.yml", "rs1000001: AG\n"))
	none(t, "variant catalogue with alleles", scan("docs/variants.md", "rs1000001 A/T is common.\nrs1000002 C/G is rare.\nrs1000003 A/G\n"))
	none(t, "rsIDs alone", scan("docs/snps.md", "See rs1000001, rs1000002 and rs1000003.\n"))
}

func TestBiometricTemplates(t *testing.T) {
	floats := func(n int, vary bool) string {
		xs := make([]string, n)
		for i := range xs {
			v := 0.25
			if vary {
				v = float64(i%17)/40 - 0.2
			}
			xs[i] = fmt.Sprintf("%.4f", v)
		}
		return strings.Join(xs, ", ")
	}
	expect(t, "face embedding on one line", scan("app/faces.json", `{"user": 1, "face_embedding": [`+floats(128, true)+`]}`), Biometric, 1, false)
	multi := "face_encoding = np.array([\n" + strings.ReplaceAll(floats(128, true), ", ", ",\n") + "\n])\nother = [1, 2]\n"
	fs := scan("app/enroll.py", multi)
	expect(t, "encoding over many lines", fs, Biometric, 1, false)
	if fs[0].Masked != "face_encoding" {
		t.Fatalf("mask %q", fs[0].Masked)
	}
	expect(t, "constant vector", scan("app/faces.json", `"faceEmbedding": [`+floats(128, false)+`]`), Biometric, 1, true)
	expect(t, "encoded fingerprint template", scan("app/finger.json", `"fingerprint_template": "`+strings.Repeat("QUJDRA", 20)+`"`), Biometric, 1, false)

	none(t, "dimension", scan("app/config.yml", "face_embedding_dim: 128\nface_embedding: 512\n"))
	none(t, "short list", scan("app/faces.json", `"face_embedding": [`+floats(10, true)+`]`))
	none(t, "column type", scan("app/models.py", "face_embedding = Column(ARRAY(Float))\n"))
	none(t, "certificate fingerprint", scan("app/tls.json", `"fingerprint": "`+strings.Repeat("ab", 64)+`"`))
}

func TestOnlyEnabledChecksRun(t *testing.T) {
	s := NewScanner("data/feed.txt", []string{VCF})
	if hs := s.Line(1, pidLine); len(hs) != 0 {
		t.Fatalf("hl7 ran while switched off: %+v", hs)
	}
}

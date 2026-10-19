package detectors

import (
	"strings"
	"testing"
)

func TestPersonNameInANameField(t *testing.T) {
	d, _ := Get("person-name")
	if d.OnByDefault || !d.NeedsKeyword {
		t.Fatal("the name detector must be off by default and keyword-gated")
	}
	if len(personNames) < 5000 {
		t.Fatalf("the name list has only %d names", len(personNames))
	}
	found := map[string]string{
		`"first_name": "Maria"`:         "Maria",
		`lastName: Popescu`:             "Popescu",
		`NACHNAME='Müller'`:             "Müller",
		`<prenume>Andrei</prenume>`:     "Andrei",
		`user.nazwisko = "Kowalski"`:    "Kowalski",
		`full_name: "Ana Popescu"`:      "Ana Popescu",
		`"vorname" => "Klaus"`:          "Klaus",
		`"фамилия": "Петров"`:           "Петров",
		`first_name: "Maria Zyxwvut"`:   "Maria",
		`customer_surname: "Jensen"`:    "Jensen",
		`"apellido":"García"`:           "García",
		`{"imię": "Piotr", "x": 1}`:     "Piotr",
		`row["cognome"] = "Rossi"`:      "Rossi",
		`"επώνυμο": "Παπαδόπουλος"`:     "Παπαδόπουλος",
		`vezetéknév: Nagy`:              "Nagy",
		`efternamn: "Johansson-Borg"`:   "Johansson-Borg",
		`given_names: "Jan Erik"`:       "Jan Erik",
		`"FirstName":"Sean","age":40`:   "Sean",
		`last_name := "Murphy"`:         "Murphy",
		`"nombre": "Javier", "edad": 3`: "Javier",
	}
	for line, want := range found {
		loc := d.Pattern.FindStringIndex(line)
		if loc == nil {
			t.Errorf("no candidate in a name field holding %d characters", len(want))
			continue
		}
		end, ok := validEndForTest(d, line, loc[0], loc[1])
		if !ok {
			t.Errorf("a name field holding %d characters is rejected", len(want))
			continue
		}
		if got := d.Normalize(line[loc[0]:end]); got != want {
			t.Errorf("value is %d characters, want %d", len(got), len(want))
		}
	}
	for i, line := range []string{
		`first_name: "Button"`,       // not a name
		`first_name: "maria"`,        // not capitalised
		`first_name: "MARIA"`,        // all capitals
		`first_name: "Test"`,         // placeholder
		`username: "Maria"`,          // not a name field
		`runtime: "Max"`,             // a key that only ends like one
		`Maria Popescu`,              // free text
		`// the first name is Maria`, // prose, no assignment
		`first_name: {{ .Name }}`,
	} {
		loc := d.Pattern.FindStringIndex(line)
		if loc == nil {
			continue
		}
		if _, ok := validEndForTest(d, line, loc[0], loc[1]); ok && boundedForTest(line, loc[0]) {
			t.Errorf("case %d accepted", i)
		}
	}
	if d.Normalize("Maria") != "" || validPersonName("") {
		t.Error("a bare word is not a name field")
	}
	if m := d.Mask("Maria"); m != strings.Repeat("•", 5) {
		t.Errorf("mask %q", m)
	}
	if !mayHaveNameKey(`"nazwisko": "Nowak"`) || mayHaveNameKey(`timeout 30s`) || mayHaveNameKey(`port: 8080`) {
		t.Error("prefilter wrong")
	}
}

// validEndForTest mirrors the engine: try the candidate, then without its last space- or dash-separated group.
func validEndForTest(d Detector, line string, start, end int) (int, bool) {
	for {
		if d.Validate(d.Normalize(line[start:end])) {
			return end, true
		}
		cut := strings.LastIndexAny(line[start:end], " -")
		if cut <= 0 {
			return 0, false
		}
		end = start + cut
	}
}

func boundedForTest(line string, start int) bool {
	if start == 0 {
		return true
	}
	c := line[start-1]
	return !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9')
}

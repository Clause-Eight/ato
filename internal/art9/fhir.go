package art9

import (
	"regexp"
	"strings"
)

// FHIR (HL7 FHIR R4, https://hl7.org/fhir/R4/patient.html and https://hl7.org/fhir/R4/observation.html) writes
// each resource as a JSON object with a "resourceType" key. A Patient is reported when it has a name (family,
// given or text under "name") and a birth date or an identifier value. A clinical resource, such as an
// Observation or a Condition, is reported when its subject or patient is named (a display) or identified (an
// identifier value); a bare reference such as Patient/123 is left alone, since it names nobody by itself.
//
// The JSON is read with a small tokenizer that follows objects, arrays, keys and string values across lines, so
// pretty-printed, minified and NDJSON resources are all read, as are bundles and contained resources. It starts
// at the first line with "resourceType" and stops when the object it is in closes. JSON written into a string
// literal, with escaped quotes, is read too.

// clinical are the resource types whose subject is a patient and whose content is health data.
var clinical = map[string]bool{
	"Observation": true, "Condition": true, "Encounter": true, "Procedure": true, "DiagnosticReport": true,
	"MedicationRequest": true, "MedicationStatement": true, "MedicationAdministration": true, "MedicationDispense": true,
	"AllergyIntolerance": true, "Immunization": true, "CarePlan": true, "ClinicalImpression": true,
	"FamilyMemberHistory": true, "ImagingStudy": true, "Specimen": true, "QuestionnaireResponse": true,
}

// maxFHIRDepth stops a run of unbalanced brackets from growing the stack without limit.
const maxFHIRDepth = 256

var fhirDate = regexp.MustCompile(`^\d{4}(-\d{2}(-\d{2})?)?$`)

type fhirScanner struct {
	active  bool
	escaped bool
	stack   []*frame
	open    []*resource
	// The string being read, and a finished string waiting for the next character to say if it is a key.
	inString, escape bool
	buf              strings.Builder
	pending          *token
	start            place
}

type token struct {
	text string
	at   place
}

// frame is an open object or array. An object's key is its last key, an array's the key it is the value of;
// path is the keys from the resource the frame belongs to down to the frame.
type frame struct {
	array bool
	key   string
	path  []string
	keyAt place
	res   *resource
	owns  bool
}

type resource struct {
	typ                         string
	at                          place
	names, placeholders         int
	birth, identifier           bool
	subjectName, subjectID      bool
	subjectPlaceholder, emitted bool
}

func (f *fhirScanner) line(s *Scanner, at place) []Hit {
	text, from := at.text, 0
	starting := !f.active
	if starting {
		switch {
		case strings.Contains(text, `"resourceType"`):
			f.escaped = false
		case strings.Contains(text, `\"resourceType\"`):
			f.escaped = true
		default:
			return nil
		}
		f.active = true
		f.stack = []*frame{{}}
		f.inString, f.escape, f.pending = false, false, nil
	}
	if f.escaped {
		text = strings.ReplaceAll(text, `\"`, `"`)
	}
	if starting {
		// Start at the object that holds the key, or at the key when the object opened on an earlier line, so
		// that code before it on the line is not read as JSON.
		k := strings.Index(text, `"resourceType"`)
		if from = strings.LastIndexByte(text[:k], '{'); from < 0 {
			from = k
		}
	}
	var out []Hit
	for i := from; i < len(text) && f.active; i++ {
		c := text[i]
		if f.inString {
			switch {
			case f.escape:
				f.escape = false
				f.buf.WriteByte(c)
			case c == '\\':
				f.escape = true
			case c == '"':
				f.inString = false
				f.pending = &token{text: f.buf.String(), at: f.start}
			default:
				f.buf.WriteByte(c)
			}
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' {
			continue
		}
		if f.pending != nil {
			p := f.pending
			f.pending = nil
			if c == ':' {
				f.top().key = p.text
				f.top().keyAt = p.at
				continue
			}
			f.value(p)
		}
		switch c {
		case '"':
			f.inString = true
			f.buf.Reset()
			f.start = at
			f.start.column = len([]rune(text[:i])) + 1
		case '{', '[':
			f.push(c == '[')
		case '}', ']':
			out = append(out, f.pop(s)...)
		}
	}
	if f.active && len(f.stack) == 1 && f.stack[0].res == nil && !f.inString && f.pending == nil {
		f.active = false
	}
	return out
}

func (f *fhirScanner) top() *frame { return f.stack[len(f.stack)-1] }

func (f *fhirScanner) push(array bool) {
	if len(f.stack) >= maxFHIRDepth {
		f.active = false
		return
	}
	parent := f.top()
	child := &frame{array: array, res: parent.res, path: parent.path}
	if array {
		child.key = parent.key
	} else if parent.key != "" {
		child.path = append(append([]string{}, parent.path...), parent.key)
	}
	f.stack = append(f.stack, child)
}

func (f *fhirScanner) pop(s *Scanner) []Hit {
	top := f.top()
	f.stack = f.stack[:len(f.stack)-1]
	var out []Hit
	if top.owns {
		out = f.finish(s, top.res)
	}
	if len(f.stack) == 0 {
		f.active = false
	}
	return out
}

// value records what a string value says about the resource it is in.
func (f *fhirScanner) value(t *token) {
	fr := f.top()
	key, path, r := fr.key, fr.path, fr.res
	if !fr.array && key == "resourceType" {
		if fr.owns {
			return
		}
		nr := &resource{typ: t.text, at: fr.keyAt}
		fr.res, fr.owns, fr.path = nr, true, nil
		f.open = append(f.open, nr)
		return
	}
	if r == nil || t.text == "" {
		return
	}
	last := ""
	if len(path) > 0 {
		last = path[len(path)-1]
	}
	switch {
	case last == "name" && (key == "family" || key == "given" || key == "text") && hasLetters(t.text, 2):
		r.names++
		if placeholderName(t.text) {
			r.placeholders++
		}
	case len(path) == 0 && key == "birthDate" && fhirDate.MatchString(t.text):
		r.birth = true
	case len(path) == 1 && last == "identifier" && key == "value":
		r.identifier = true
	case len(path) >= 1 && (path[0] == "subject" || path[0] == "patient"):
		if len(path) == 1 && key == "display" && hasLetters(t.text, 2) {
			r.subjectName = true
			r.subjectPlaceholder = r.subjectPlaceholder || placeholderName(t.text)
		}
		if last == "identifier" && key == "value" {
			r.subjectID = true
		}
	}
}

// finish reports a resource once it is closed, if it is about an identified patient.
func (f *fhirScanner) finish(s *Scanner, r *resource) []Hit {
	if r == nil || r.emitted {
		return nil
	}
	r.emitted = true
	var reasons []string
	sample := ""
	switch {
	case r.typ == "Patient" && r.names > 0 && (r.birth || r.identifier):
		reasons = []string{"a FHIR Patient resource with a name"}
		if r.birth {
			reasons = append(reasons, "with a birth date")
		}
		if r.identifier {
			reasons = append(reasons, "with an identifier")
		}
		if r.placeholders > 0 {
			sample = "the name is a placeholder used in examples"
		}
	case clinical[r.typ] && (r.subjectName || r.subjectID):
		reasons = []string{"a FHIR " + r.typ + " resource"}
		if r.subjectName {
			reasons = append(reasons, "its subject is named")
		}
		if r.subjectID {
			reasons = append(reasons, "its subject is identified by an identifier")
		}
		if r.subjectName && r.subjectPlaceholder && !r.subjectID {
			sample = "the name is a placeholder used in examples"
		}
	default:
		return nil
	}
	return []Hit{s.hit(FHIR, r.at, r.typ+" resource", reasons, sample)}
}

// end reports the resources of a file that ends before they close, such as a cut sample.
func (f *fhirScanner) end(s *Scanner) []Hit {
	var out []Hit
	for _, r := range f.open {
		out = append(out, f.finish(s, r)...)
	}
	return out
}

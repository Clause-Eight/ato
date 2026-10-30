package sinks

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Clause-Eight/pii-eu/internal/classify"
	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// expr is an expression such as user.email, $user->email, params[:email] or user.getEmail() in a call's
// arguments, as tokens toks[first:next].
type expr struct {
	segs        []string
	first, next int
	call        bool
	key         bool
	compared    bool
}

func readPath(toks []token, i int) expr {
	p := expr{first: i, segs: []string{toks[i].text}}
	j := i + 1
	for j < len(toks) {
		t := toks[j]
		switch {
		case t.kind == 'p' && (t.text == "." || t.text == "?.") && j+3 < len(toks) && toks[j+1].kind == 'i' &&
			transforms[strings.ToLower(toks[j+1].text)] && toks[j+2].text == "(" && toks[j+3].text == ")":
			// user.email.strip() is still the email: a string method that changes case or spaces is skipped.
			j += 4
			continue
		case t.kind == 'p' && (t.text == "." || t.text == "?." || t.text == "->" || t.text == "::") && j+1 < len(toks) && toks[j+1].kind == 'i':
			p.segs = append(p.segs, toks[j+1].text)
			j += 2
			continue
		case t.text == "[" && j+2 < len(toks) && toks[j+1].kind == 's' && toks[j+2].text == "]":
			p.segs = append(p.segs, strings.Trim(toks[j+1].text, "\"'`"))
			j += 3
			continue
		case t.text == "[" && j+3 < len(toks) && toks[j+1].text == ":" && toks[j+2].kind == 'i' && toks[j+3].text == "]":
			p.segs = append(p.segs, toks[j+2].text)
			j += 4
			continue
		case t.text == "(" && j+1 < len(toks) && toks[j+1].text == ")" && accessor(p.segs[len(p.segs)-1]):
			j += 2
			continue
		}
		break
	}
	p.next = j
	if j < len(toks) {
		switch toks[j].text {
		case "(":
			p.call = true
		case ":", "=":
			p.key = true
		case "==", "!=", "<", ">", "<=", ">=", "is", "instanceof":
			p.compared = true
		}
	}
	return p
}

// transforms are string methods without arguments that change only case or surrounding spaces, so the result
// still holds the value.
var transforms = map[string]bool{"strip": true, "lstrip": true, "rstrip": true, "trim": true, "trimstart": true, "trimend": true,
	"lower": true, "upper": true, "casefold": true, "title": true, "capitalize": true, "tolowercase": true, "touppercase": true,
	"tolocalelowercase": true, "tolocaleuppercase": true, "downcase": true, "upcase": true, "squish": true, "chomp": true,
	"lowercase": true, "uppercase": true, "trimspace": true}

// serializers turn an object into text without changing what it holds.
var serializers = map[string]bool{"to_json": true, "tojson": true, "to_h": true, "to_dict": true, "dict": true, "model_dump": true, "as_json": true, "inspect": true, "tostring": true, "to_s": true, "string": true, "__dict__": true, "read": true, "decode": true, "values": true}

// accessor reports whether a method called without arguments returns data rather than doing something:
// a getter (getEmail, get_email) or a serializer.
func accessor(name string) bool {
	l := strings.ToLower(name)
	return strings.HasPrefix(name, "get") && len(name) > 3 && (name[3] == '_' || unicode.IsUpper(rune(name[3]))) ||
		serializers[l] || l == "get_json" || l == "json" || l == "all" || l == "input" || l == "get_data" || l == "raw_post"
}

// verdict is what an expression passed to a sink says about the data. kind separates fields, request
// bodies and whole objects, so a call is reported once for each.
type verdict struct {
	category   domain.DataCategory
	special    domain.SpecialCategory
	confidence domain.Confidence
	reason     string
	kind       string
	via        string
}

var keywords = map[string]bool{"new": true, "return": true, "await": true, "typeof": true, "not": true, "and": true, "or": true, "in": true, "is": true, "none": true, "null": true, "nil": true, "true": true, "false": true, "undefined": true, "this": true, "self": true, "lambda": true, "fn": true, "func": true, "function": true}

// judge decides whether an expression holds personal data.
func (s *Scanner) judge(segs []string) (verdict, bool) {
	for len(segs) > 1 && serializers[strings.ToLower(clean(segs[len(segs)-1]))] {
		segs = segs[:len(segs)-1]
	}
	if len(segs) == 1 && keywords[strings.ToLower(clean(segs[0]))] {
		return verdict{}, false
	}
	if v, ok := s.requestBody(segs); ok {
		return v, true
	}
	last := clean(segs[len(segs)-1])
	if v, ok := field(segs[:len(segs)-1], getterField(last)); ok {
		return v, true
	}
	if person(last) {
		return verdict{confidence: domain.Low, kind: "object", reason: fmt.Sprintf("%s looks like a whole user-like object, which usually holds personal data", last)}, true
	}
	return verdict{}, false
}

// getterField is the field a getter returns: getEmail and get_email give email.
func getterField(name string) string {
	if strings.HasPrefix(name, "get_") && len(name) > 4 {
		return name[4:]
	}
	if strings.HasPrefix(name, "get") && len(name) > 3 && unicode.IsUpper(rune(name[3])) {
		return name[3:]
	}
	return name
}

// clean drops the sigils of PHP, Ruby and private fields: $user, @user, _user.
func clean(s string) string { return strings.TrimLeft(s, "$@_") }

// flagFirst and flagLast are words that make a name about a value, not the value: isEmail, email_count,
// emailTemplate, EMAIL_REGEX.
var flagFirst = map[string]bool{"is": true, "has": true, "should": true, "can": true, "must": true, "valid": true, "validate": true, "verify": true, "num": true, "count": true, "total": true, "max": true, "min": true, "send": true, "parse": true, "format": true, "normalize": true, "mask": true, "masked": true, "hash": true, "hashed": true, "encrypt": true, "encrypted": true, "on": true, "handle": true, "update": true, "set": true}

var flagLast = map[string]bool{"verified": true, "valid": true, "validated": true, "validation": true, "verification": true, "count": true, "sent": true, "exists": true, "present": true, "enabled": true, "disabled": true, "type": true, "kind": true, "status": true, "state": true, "template": true, "templates": true, "service": true, "sender": true, "client": true, "provider": true, "format": true, "pattern": true, "regex": true, "regexp": true, "re": true, "field": true, "fields": true, "column": true, "col": true, "key": true, "keys": true, "label": true, "error": true, "errors": true, "err": true, "required": true, "masked": true, "mask": true, "hash": true, "hashed": true, "digest": true, "length": true, "len": true, "size": true, "changed": true, "updated": true, "confirmed": true, "ids": true, "encrypted": true, "enc": true, "token": true, "subject": true, "body": true, "content": true, "html": true, "text": true, "message": true, "msg": true, "settings": true, "setting": true, "config": true, "prefix": true, "suffix": true, "domain": true, "host": true, "server": true, "port": true, "url": true, "link": true, "queue": true, "job": true, "task": true, "event": true, "notification": true, "notifications": true, "consent": true, "optin": true, "at": true, "flag": true, "enum": true, "schema": true, "validator": true, "handler": true, "verifier": true}

// ambiguous are words that hold personal data only when they belong to a person: user.address but not
// server.address, customer.age but not cache.age.
var ambiguous = map[string]bool{"address": true, "age": true, "sex": true, "mobile": true, "zip": true, "gps": true, "coordinates": true, "street": true, "denomination": true, "faith": true, "race": true, "inps": true}

// field classifies a field name; receivers are the path in front of it (user in user.email).
func field(receivers []string, name string) (verdict, bool) {
	c, ok := classify.Field(name)
	if !ok {
		return verdict{}, false
	}
	ws := words(name)
	if len(ws) == 0 || flagFirst[ws[0]] || flagLast[ws[len(ws)-1]] {
		return verdict{}, false
	}
	if len(c.Matched) < 3 || ambiguous[strings.ToLower(c.Matched)] && !personContext(receivers) {
		return verdict{}, false
	}
	v := verdict{category: c.Category, special: c.Special, confidence: domain.Medium, kind: "field", reason: fmt.Sprintf("the name contains %q", c.Matched)}
	if c.Category == domain.CategoryOnline {
		v.confidence = domain.Low
	}
	return v, true
}

func personContext(receivers []string) bool {
	for _, r := range receivers {
		for _, w := range words(clean(r)) {
			if persons[strings.TrimSuffix(w, "s")] || persons[w] {
				return true
			}
		}
	}
	return false
}

// persons are the words for an object that describes a person.
var persons = map[string]bool{"user": true, "customer": true, "patient": true, "employee": true, "member": true, "person": true, "people": true, "profile": true, "applicant": true, "subscriber": true, "recipient": true, "contact": true, "buyer": true, "citizen": true, "passenger": true, "student": true, "candidate": true, "beneficiary": true, "cardholder": true, "guest": true, "visitor": true, "shopper": true, "consumer": true, "traveller": true, "traveler": true, "worker": true, "persona": true, "usuario": true, "utilisateur": true, "benutzer": true, "kunde": true, "utilizator": true}

// modifiers may sit next to a person word in an object's name: currentUser, user_data, customerDTO.
var modifiers = map[string]bool{"current": true, "logged": true, "auth": true, "authenticated": true, "new": true, "old": true, "existing": true, "updated": true, "created": true, "the": true, "my": true, "data": true, "info": true, "details": true, "detail": true, "record": true, "obj": true, "object": true, "dto": true, "entity": true, "model": true, "json": true, "dict": true, "map": true, "raw": true, "full": true, "payload": true, "doc": true, "document": true, "row": true, "in": true, "input": true}

// person reports whether a name is a whole person-like object: user, currentUser, customer_data, users.
func person(name string) bool {
	ws := words(name)
	if len(ws) == 0 {
		return false
	}
	last := ws[len(ws)-1]
	if !persons[last] && !(strings.HasSuffix(last, "s") && persons[strings.TrimSuffix(last, "s")]) {
		found := -1
		for i, w := range ws {
			if persons[w] {
				found = i
			}
		}
		if found < 0 {
			return false
		}
		for _, w := range ws[found+1:] {
			if !modifiers[w] {
				return false
			}
		}
		ws = ws[:found]
	} else {
		ws = ws[:len(ws)-1]
	}
	for _, w := range ws {
		if !modifiers[w] {
			return false
		}
	}
	return true
}

var requestNames = map[string]bool{"req": true, "request": true, "httprequest": true, "servletrequest": true}

var bodyNames = map[string]bool{"body": true, "json": true, "form": true, "data": true, "post": true, "params": true, "all": true, "getcontent": true, "get_json": true, "getbody": true, "raw_post": true, "rawpost": true, "payload": true, "input": true, "get_data": true, "getparametermap": true}

// requestBody recognises a whole request body: req.body, request.json, request.POST, $request->all(),
// $_POST, and params in Ruby.
func (s *Scanner) requestBody(segs []string) (verdict, bool) {
	v := verdict{confidence: domain.Medium, kind: "body", reason: "the whole request body is passed, which usually holds what a person typed in"}
	first := segs[0]
	if len(segs) == 1 && (first == "$_POST" || first == "$_REQUEST") {
		return v, true
	}
	if s.lang.name == "ruby" && first == "params" && (len(segs) == 1 || len(segs) == 2 && (segs[1] == "to_unsafe_h" || segs[1] == "permit")) {
		return v, true
	}
	n := len(segs)
	if n >= 2 && requestNames[strings.ToLower(clean(segs[n-2]))] && bodyNames[strings.ToLower(segs[n-1])] {
		return v, true
	}
	return verdict{}, false
}

// words splits a name into lower-case words at underscores, dashes, digits and camelCase humps.
func words(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case !unicode.IsLetter(r):
			flush()
		case unicode.IsUpper(r) && i > 0 && (unicode.IsLower(rs[i-1]) || i+1 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsUpper(rs[i-1])):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return out
}

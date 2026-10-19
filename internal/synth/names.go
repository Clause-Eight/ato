package synth

import (
	"fmt"
	"math/rand"
)

// A few common names per country, chosen by hand, so the generator does not read the detector's list.
var (
	givenNames  = []string{"Maria", "Ana", "Andrei", "Alexandru", "Klaus", "Anna", "Jan", "Piotr", "Marie", "Martin", "Giuseppe", "Javier", "Juan", "Lars", "Erik", "László", "Petra", "Ivan", "Georgi", "Pieter", "Sean", "Mikko", "João", "Elena", "Sofia", "Иван", "Βασίλειος"}
	familyNames = []string{"Popescu", "Ionescu", "Müller", "Schmidt", "Kowalski", "Nowak", "Dubois", "Rossi", "Bianchi", "García", "López", "Jensen", "Nielsen", "Johansson", "Nagy", "Kovács", "Novák", "Petrov", "Dimitrov", "Jansen", "Murphy", "Virtanen", "Tamm", "Bērziņš", "Kazlauskas", "Horvat", "Borg", "Silva", "Петров", "Παπαδόπουλος"}
	givenKeys   = []string{"first_name", "firstName", "FIRST_NAME", "given_name", "vorname", "prenume", "prénom", "nombre", "imię", "voornaam", "keresztnév", "fornavn", "etunimi", "jméno", "nome", "име", "όνομα"}
	familyKeys  = []string{"last_name", "lastName", "surname", "family_name", "nachname", "nume", "nom", "apellido", "nazwisko", "cognome", "achternaam", "vezetéknév", "efternamn", "sukunimi", "příjmení", "prezime", "pavardė", "фамилия", "επώνυμο"}
)

func init() {
	// A name field the way it appears in code, configuration and fixtures: the key, a separator, the value.
	Register("person-name", func(r *rand.Rand) string {
		key, value := givenKeys[r.Intn(len(givenKeys))], givenNames[r.Intn(len(givenNames))]
		switch r.Intn(3) {
		case 0:
			key, value = familyKeys[r.Intn(len(familyKeys))], familyNames[r.Intn(len(familyNames))]
		case 1:
			key, value = "full_name", givenNames[r.Intn(len(givenNames))]+" "+familyNames[r.Intn(len(familyNames))]
		}
		switch r.Intn(5) {
		case 0:
			return fmt.Sprintf(`%s": "%s`, key, value)
		case 1:
			return fmt.Sprintf(`%s: %s`, key, value)
		case 2:
			return fmt.Sprintf(`%s='%s`, key, value)
		case 3:
			return fmt.Sprintf(`<%s>%s`, key, value)[1:]
		}
		return fmt.Sprintf(`%s = "%s`, key, value)
	})
}

package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("postcode", func(r *rand.Rand) string {
		const eir = "ACDEFHKNPRTVWXY0123456789"
		switch r.Intn(12) {
		case 0:
			code := fmt.Sprint(10000 + r.Intn(89999)) // DE, FR, IT, ES, HR, EE, FI
			if r.Intn(2) == 0 {
				return code + " " + []string{"Berlin", "Lyon", "Torino", "Sevilla", "Tampere"}[r.Intn(5)]
			}
			return code
		case 1:
			code := fmt.Sprint(1000 + r.Intn(8999)) // AT, BE, BG, CY, DK, HU, LU, SI
			if r.Intn(2) == 0 {
				return code + ", " + []string{"Wien", "Gent", "Plovdiv", "Odense", "Szeged"}[r.Intn(5)]
			}
			return code
		case 2:
			return fmt.Sprintf("%d %s", 1000+r.Intn(8999), Letters(r, 2, "ABCEGHJKLMNPRTVWXZ")) // NL
		case 3:
			return Digits(r, 2) + "-" + Digits(r, 3) // PL
		case 4:
			return fmt.Sprintf("%d-%s", 1000+r.Intn(8999), Digits(r, 3)) // PT
		case 5:
			return fmt.Sprintf("%d%s %s", 1+r.Intn(9), Digits(r, 2), Digits(r, 2)) // CZ, SK, GR, SE
		case 6:
			return Letters(r, 1, "ACDEFHKNPRTVWXY") + Digits(r, 2) + " " + Letters(r, 4, eir) // IE Eircode
		case 7:
			localities := []string{"VLT", "SLM", "BKR", "MST", "QRM", "ZBR", "GZR", "SPB"}
			return localities[r.Intn(len(localities))] + " " + fmt.Sprint(1000+r.Intn(8999)) // MT
		case 8:
			return fmt.Sprintf("L-%d", 1000+r.Intn(8999)) // LU
		case 9:
			return fmt.Sprintf("LV-%d", 1000+r.Intn(8999)) // LV
		case 10:
			return fmt.Sprintf("LT-%d", 10000+r.Intn(89999)) // LT
		}
		return Pad(10000+r.Intn(900000), 6) // RO
	})

	Register("street-address", func(r *rand.Rand) string {
		names := []string{"Lalelelor", "Kwiatowa", "Mayor", "Roma", "Augusta", "Unirii", "Victoriei", "Lipowa", "Garibaldi", "Verde"}
		name := names[r.Intn(len(names))]
		number := fmt.Sprint(1 + r.Intn(250))
		if r.Intn(5) == 0 {
			number += Letters(r, 1, "ab")
		}
		switch r.Intn(10) {
		case 0:
			return "Strada " + name + " " + number
		case 1:
			return "ul. " + name + " " + number
		case 2:
			return "Calle " + name + ", " + number
		case 3:
			return "Via " + name + " " + number
		case 4:
			return number + " rue de la " + name
		case 5:
			return "Rua " + name + ", " + number
		case 6:
			stems := []string{"Haupt", "Linden", "Berg", "Kirch"}
			suffixes := []string{"straße", "strasse", "gasse", "weg", "straat", "gatan", "vej", "katu"}
			return stems[r.Intn(len(stems))] + suffixes[r.Intn(len(suffixes))] + " " + number
		case 7:
			return name + " " + []string{"utca", "iela", "ulica", "cesta"}[r.Intn(4)] + " " + number
		case 8:
			return number + " " + name + " " + []string{"Street", "Road", "Avenue", "Lane"}[r.Intn(4)]
		}
		return "Bulevardul " + name + " nr. " + number
	})
}

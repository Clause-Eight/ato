package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("birth-date", func(r *rand.Rand) string {
		year, month, day := Date(r)
		months := []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
		switch r.Intn(8) {
		case 0:
			return fmt.Sprintf("%d-%02d-%02d", year, month, day)
		case 1:
			return fmt.Sprintf("%02d.%02d.%d", day, month, year)
		case 2:
			return fmt.Sprintf("%02d/%02d/%d", day, month, year)
		case 3:
			return fmt.Sprintf("%02d-%02d-%d", day, month, year)
		case 4:
			return fmt.Sprintf("%d.%d.%d", day, month, year)
		case 5:
			return fmt.Sprintf("%d.%02d.%02d.", year, month, day)
		case 6:
			return fmt.Sprintf("%d. %d. %d", day, month, year)
		}
		return fmt.Sprintf("%d %s %d", day, months[month-1], year)
	})
}

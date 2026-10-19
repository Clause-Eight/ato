package synth

import (
	"fmt"
	"math/rand"
	"strings"
)

func init() {
	Register("gps-coordinates", func(r *rand.Rand) string {
		lat := float64(r.Intn(1800000000)-900000000) / 1e7
		lon := float64(r.Intn(3600000000)-1800000000) / 1e7
		if lat == 0 && lon == 0 {
			lat = 1.5
		}
		places := 4 + r.Intn(4)
		latS, lonS := fmt.Sprintf("%.*f", places, lat), fmt.Sprintf("%.*f", places, lon)
		switch r.Intn(4) {
		case 0:
			return latS + ", " + lonS
		case 1:
			return "lat=" + latS + " lng=" + lonS
		case 2:
			return latS + "," + lonS
		}
		return strings.Join([]string{"lat: " + latS, "lon: " + lonS}, ", ")
	})
}

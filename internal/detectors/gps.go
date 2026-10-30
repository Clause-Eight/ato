package detectors

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// GPS coordinates: a latitude and a longitude in decimal degrees with four or more decimals each (about 11 m),
// latitude within ±90 and longitude within ±180, either as a bare pair or keyed (lat=..., lng=...), as in
// ISO 6709 (https://www.iso.org/standard/75147.html). (0, 0) and pairs of whole degrees are not reported.
func init() {
	Register(Detector{
		Name:         "gps-coordinates",
		Label:        "GPS coordinates (latitude and longitude)",
		Country:      "EU",
		OnByDefault:  false,
		Pattern:      gpsPattern,
		Validate:     validGPS,
		Keywords:     []string{"gps", "lat", "lng", "lon", "latitude", "longitude", "coords", "geo"},
		Concepts:     []Concept{Location},
		Base:         domain.Low,
		Validation:   "latitude and longitude within range, four or more decimals",
		NeedsKeyword: true,
		Needs:        Needs{AnyOf: []string{"."}},
	})
}

var gpsPattern = func() *regexp.Regexp {
	lat := `-?\d{1,2}\.\d{4,15}`
	lon := `-?\d{1,3}\.\d{4,15}`
	q := `["']?`
	pair := lat + `(?:\s*[,;]\s*|\s+)` + lon
	keyed := `(?i:lat(?:itude)?)` + q + `\s*[:=]\s*` + q + lat + q + `\s*[,;&]?\s*` + q + `(?i:lng|lon|long|longitude)` + q + `\s*[:=]\s*` + q + lon
	return regexp.MustCompile(keyed + `|` + pair)
}()

var gpsNumber = regexp.MustCompile(`-?\d+\.(\d+)`)

func validGPS(s string) bool {
	m := gpsNumber.FindAllStringSubmatch(s, -1)
	if len(m) != 2 {
		return false
	}
	lat, err1 := strconv.ParseFloat(m[0][0], 64)
	lon, err2 := strconv.ParseFloat(m[1][0], 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return false
	}
	if len(m[0][1]) < 4 || len(m[1][1]) < 4 {
		return false
	}
	zeros := func(f string) bool { return strings.Trim(f, "0") == "" }
	return !zeros(m[0][1]) || !zeros(m[1][1])
}

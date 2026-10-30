//go:build race

package fix

// raceDetector is true under -race, where the exhaustive replacement tests check one value per detector.
const raceDetector = true

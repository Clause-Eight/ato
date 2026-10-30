package main

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/internal/config"
	"github.com/Clause-Eight/ato/internal/detectors"
	"github.com/Clause-Eight/ato/internal/engine"
)

// recallDetectors are the identifiers inserted into the application-style corpus, as in bench/compare.
var recallDetectors = []string{"ro-cnp", "bg-egn", "hu-taj", "pl-pesel", "cz-birth-number", "hr-oib", "de-tax-id", "es-dni",
	"it-fiscal-code", "fr-nir", "nl-bsn", "iban", "card", "email", "eu-phone"}

// appFile is one file of the corpus: where it sits, how it writes a key and a value, and what fills it.
type appFile struct {
	path  string
	entry func(key, value string) string
	noise func(r *rand.Rand) string
}

var appFiles = []appFile{
	{"tests/fixtures/customers.json", func(k, v string) string { return fmt.Sprintf("    %q: %q,", k, v) }, func(r *rand.Rand) string {
		return fmt.Sprintf(`    {"id": %d, "plan": %q, "created_at": %d, "order_id": %d},`, r.Intn(100000), []string{"pro", "free", "team"}[r.Intn(3)], 1_600_000_000_000+r.Int63n(200_000_000_000), 1_000_000_000_000+r.Int63n(9_000_000_000_000))
	}},
	{"db/migrations/0042_seed_accounts.py", func(k, v string) string { return fmt.Sprintf("        %s=%q,", k, v) }, func(r *rand.Rand) string {
		return fmt.Sprintf("        migrations.AddField(model_name=%q, name=%q, field=models.IntegerField(default=%d)),", "order", "total", r.Intn(1000))
	}},
	{"database/seeders/AccountSeeder.php", func(k, v string) string { return fmt.Sprintf("            '%s' => '%s',", k, v) }, func(r *rand.Rand) string {
		return fmt.Sprintf("            'sku' => 'SKU-%06d', 'price' => %d.%02d, 'stock' => %d,", r.Intn(1_000_000), r.Intn(500), r.Intn(100), r.Intn(300))
	}},
	{"spec/factories/users.rb", func(k, v string) string { return fmt.Sprintf("    %s { %q }", k, v) }, func(r *rand.Rand) string {
		return fmt.Sprintf("    sequence(:number) { |n| \"R%09d#{n}\" }", r.Intn(1_000_000_000))
	}},
	{"logs/app.log", func(k, v string) string { return fmt.Sprintf("2026-10-29T10:41:07.412Z INFO checkout %s=%s", k, v) }, func(r *rand.Rand) string {
		return fmt.Sprintf("2026-10-29T%02d:%02d:%02d.%03dZ INFO cart request_id=%016x ts=%d duration_ms=%d user_id=%d status=%d",
			r.Intn(24), r.Intn(60), r.Intn(60), r.Intn(1000), r.Uint64(), 1_700_000_000_000+r.Int63n(100_000_000_000), r.Intn(4000), r.Intn(9_999_999), []int{200, 201, 404, 500}[r.Intn(4)])
	}},
	{"config/settings.yml", func(k, v string) string { return fmt.Sprintf("  %s: %q", k, v) }, func(r *rand.Rand) string {
		return fmt.Sprintf("  timeout_ms: %d\n  retries: %d\n  build: \"%d.%d.%d\"", r.Intn(30000), r.Intn(5), r.Intn(9), r.Intn(30), r.Intn(99))
	}},
}

var neutralKeys = []string{"ref", "value", "code", "external_id", "data", "number"}

// TestRecallOnApplicationFiles is the regression check for #269: application-style files (fixtures,
// migrations, seeders, factories, a log, settings) with generated values inserted, scanned with the default
// detectors. The thresholds sit a few points below the measured values so that a change that loses recall or
// precision fails here before the published benchmark notices.
func TestRecallOnApplicationFiles(t *testing.T) {
	cfg := config.Default()
	var dets []detectors.Detector
	for _, d := range detectors.All() {
		if cfg.Enabled(d.Name, d.Countries(), d.OnByDefault) {
			dets = append(dets, d)
		}
	}
	r := rand.New(rand.NewSource(7))
	inserted, caught, alarms, reported := 0, 0, 0, 0
	for _, f := range appFiles {
		type row struct {
			text  string
			value bool
		}
		var rows []row
		for i := 0; i < 600; i++ {
			for _, l := range strings.Split(f.noise(r), "\n") {
				rows = append(rows, row{text: l})
			}
		}
		for _, name := range recallDetectors {
			d, _ := detectors.Get(name)
			for i := 0; i < 6; i++ {
				key := neutralKeys[r.Intn(len(neutralKeys))]
				if i%2 == 0 && len(d.Keywords) > 0 {
					key = d.Keywords[0]
				}
				at := r.Intn(len(rows) + 1)
				rows = append(rows[:at], append([]row{{text: f.entry(key, valid(r, name)), value: true}}, rows[at:]...)...)
				inserted++
			}
		}
		lines := make([]string, len(rows))
		labelled := map[int]bool{}
		for i, rw := range rows {
			lines[i] = rw.text
			if rw.value {
				labelled[i+1] = true
			}
		}
		found, err := engine.ScanReader(f.path, strings.NewReader(strings.Join(lines, "\n")+"\n"), dets)
		if err != nil {
			t.Fatal(err)
		}
		hit := map[int]bool{}
		for _, fd := range found {
			if fd.Confidence < cfg.MinConfidence || fd.Sample {
				continue
			}
			reported++
			if labelled[fd.Location.Line] {
				hit[fd.Location.Line] = true
			} else {
				alarms++
			}
		}
		caught += len(hit)
	}
	recall := float64(caught) / float64(inserted)
	precision := float64(reported-alarms) / float64(reported)
	t.Logf("caught %d of %d (%.1f%%), %d alarms, %.1f%% of warnings real", caught, inserted, 100*recall, alarms, 100*precision)
	if recall < 0.60 || precision < 0.95 {
		t.Fatalf("recall %.1f%% (want at least 60%%) or precision %.1f%% (want at least 95%%) fell", 100*recall, 100*precision)
	}
}

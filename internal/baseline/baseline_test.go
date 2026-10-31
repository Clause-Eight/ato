package baseline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

func TestSaveLoadFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	old := []domain.Finding{{Fingerprint: "b"}, {Fingerprint: "a"}, {Fingerprint: "a"}}
	if err := Save(path, old); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"a",`) || strings.Count(string(data), `"a"`) != 1 {
		t.Fatalf("not sorted or not deduplicated:\n%s", data)
	}
	b, err := Load(path, true)
	if err != nil {
		t.Fatal(err)
	}
	kept, dropped := b.Filter([]domain.Finding{{Fingerprint: "a"}, {Fingerprint: "c"}})
	if dropped != 1 || len(kept) != 1 || kept[0].Fingerprint != "c" {
		t.Fatalf("kept %v dropped %d", kept, dropped)
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	if b, err := Load(filepath.Join(dir, "none.json"), false); err != nil || len(b) != 0 {
		t.Fatal("a missing default baseline is empty")
	}
	if _, err := Load(filepath.Join(dir, "none.json"), true); err == nil {
		t.Fatal("a missing explicit baseline is an error")
	}
	bad := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(bad, []byte(`{"version": 9, "fingerprints": []}`), 0o600)
	if _, err := Load(bad, true); err == nil || !strings.Contains(err.Error(), "version 9") {
		t.Fatalf("got %v", err)
	}
}

package cli

import (
	"strings"
	"testing"
)

func TestLogSinkFindingsAndSetting(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/handler.py", "def signup(user):\n    logger.info(\"new user %s\", user.email)\n")
	r := fixScanJSON(t, dir)
	if len(r.Findings) != 1 || r.Findings[0].Detector != "log-sink" || r.Findings[0].Confidence != "medium" {
		t.Fatalf("got %+v", r.Findings)
	}
	code, out, _ := run("scan", "--no-git", "--format", "sarif", dir)
	if code != ExitFindings || !strings.Contains(out, `"ruleId": "log-sink/log"`) {
		t.Fatalf("sarif: %d %s", code, out)
	}
	writeFile(t, dir, ".pii-eu.toml", "[detectors]\nlog-sink = false\n")
	if code, out, errOut := run("scan", "--no-git", dir); code != ExitClean {
		t.Fatalf("log-sink = false should switch sink findings off: %d %q %q", code, out, errOut)
	}
}

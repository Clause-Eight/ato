package engine

import (
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
	"github.com/Clause-Eight/ato/v2/internal/sinks"
)

// SinkDetector names the findings for log, error-tracker and analytics calls that are passed personal data.
// It can be switched off in settings like a detector.
const SinkDetector = sinks.Detector

// sinkScanner adds what every finding needs to the sink package's findings: test paths, ignore comments and
// fingerprints.
type sinkScanner struct {
	s *sinks.Scanner
}

func newSinkScanner(path string) *sinkScanner {
	return &sinkScanner{s: sinks.NewScanner(path)}
}

func (c *sinkScanner) line(ctx lineContext) []domain.Finding {
	fs := c.s.Line(ctx.number, ctx.line)
	if len(fs) == 0 {
		return nil
	}
	shape := strings.TrimSpace(ctx.line)
	for i := range fs {
		f := &fs[i]
		f.TestPath = ctx.testPath
		if f.TestPath {
			f.Confidence = lower1(f.Confidence)
			f.Reasons = append(f.Reasons, "in a test, fixture or example path")
		}
		f.Fingerprint = fingerprint(SinkDetector, ctx.path, string(f.Sink)+"\x00"+f.Masked+"\x00"+shape, i)
	}
	applyIgnore(fs, ctx)
	return fs
}

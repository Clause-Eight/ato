package art9

import (
	"fmt"
	"regexp"
	"strings"
)

// VCF (The Variant Call Format Specification, VCFv4.3, https://samtools.github.io/hts-specs/VCFv4.3.pdf) starts
// with a ##fileformat=VCFv4.x line. Its #CHROM header names eight fixed columns, then FORMAT and one column per
// sample; each data line gives each sample's genotype under the GT key of FORMAT, such as 0/1 or 1|1, with . for
// a call that was not made. A file is reported once, at its header, when it has sample columns and at least one
// sample has a called genotype: a variant file without samples, such as a reference catalogue, is left alone.
// Compressed .vcf.gz and BCF files are binary and not read.

// referenceSample matches the public reference samples of the 1000 Genomes and HapMap projects (NA12878,
// HG00096), which are published with consent and fill tool test data.
var referenceSample = regexp.MustCompile(`^(NA|HG)\d{5}$`)

type vcfScanner struct {
	vcf, done bool
	samples   []string
	header    place
}

func (v *vcfScanner) line(s *Scanner, at place) []Hit {
	if v.done {
		return nil
	}
	if at.line == 1 {
		v.vcf = strings.HasPrefix(at.text, "##fileformat=VCFv4")
	}
	if !v.vcf {
		v.done = true
		return nil
	}
	if strings.HasPrefix(at.text, "##") {
		return nil
	}
	cols := strings.Split(strings.TrimRight(at.text, "\r"), "\t")
	if strings.HasPrefix(at.text, "#CHROM") {
		if len(cols) <= 9 {
			v.done = true
			return nil
		}
		v.samples, v.header = cols[9:], at
		return nil
	}
	if v.samples == nil || len(cols) <= 9 {
		return nil
	}
	gt := -1
	for i, k := range strings.Split(cols[8], ":") {
		if k == "GT" {
			gt = i
		}
	}
	if gt < 0 {
		return nil
	}
	for _, c := range cols[9:] {
		parts := strings.Split(c, ":")
		if gt < len(parts) && strings.ContainsAny(parts[gt], "0123456789") {
			v.done = true
			return []Hit{s.hit(VCF, v.header, fmt.Sprintf("VCF header (%d samples)", len(v.samples)),
				[]string{fmt.Sprintf("a VCF file with %d sample columns and called genotypes", len(v.samples))}, v.sample())}
		}
	}
	return nil
}

func (v *vcfScanner) sample() string {
	for _, name := range v.samples {
		if !referenceSample.MatchString(strings.TrimSpace(name)) {
			return ""
		}
	}
	return "every sample is a public reference sample (1000 Genomes or HapMap)"
}

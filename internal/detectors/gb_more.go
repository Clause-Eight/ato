package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// UK Unique Taxpayer Reference (UTR): ten digits, the first a check digit over the other nine weighted 6, 7, 8, 9,
// 10, 5, 4, 3, 2: the sum mod 11 picks it from 2,1,9,8,7,6,5,4,3,2,1. Ten digits are everywhere and the check lets
// one number in eleven through, so it is reported only with a keyword nearby.
// Source: HMRC, UK TIN sheet published by the OECD,
// https://www.oecd.org/tax/automatic-exchange/crs-implementation-and-assistance/tax-identification-numbers/UK-TIN.pdf
//
// English Unique Pupil Number (UPN): thirteen characters, a check letter, the three-digit number of the local
// authority that issued it, the school's four-digit number, two digits of the year and a three-digit serial
// (temporary numbers end in a letter instead of the last digit). The check letter is the other twelve characters,
// read in ABCDEFGHJKLMNPQRTUVWXYZ0123456789 and weighted 2 to 13, mod 23, read back in that alphabet. A pupil
// number belongs to a child (GDPR Art. 8).
// Source: Department for Education, Unique pupil numbers (UPN), https://www.gov.uk/government/publications/unique-pupil-numbers
func init() {
	Register(Detector{
		Name:         "gb-utr",
		Label:        "UK Unique Taxpayer Reference (UTR)",
		Country:      "GB",
		OnByDefault:  false,
		Pattern:      regexp.MustCompile(`\d{5} ?\d{5}`),
		Normalize:    DigitsOnly,
		Validate:     validUTR,
		NeedsKeyword: true,
		Keywords:     []string{"utr", "unique taxpayer reference", "self assessment", "taxpayer reference"},
		Concepts:     []Concept{TaxID},
		Base:         domain.Low,
		Validation:   "mod-11 check digit is valid",
	})
	Register(Detector{
		Name:        "gb-upn",
		Label:       "English Unique Pupil Number (UPN)",
		Country:     "GB",
		OnByDefault: false,
		Pattern:     regexp.MustCompile(`[A-HJ-NPQRT-Z]\d{11}[0-9A-HJ-NPQRT-Z]`),
		Validate:    validUPN,
		Needs:       Needs{Check: func(line string) bool { return capsThenDigit(line, 1, 0) }},
		Keywords:    []string{"upn", "unique pupil number", "pupil number", "pupil_number"},
		Base:        domain.Low,
		Validation:  "local authority number and check letter are valid",
	})
}

var utrCheck = "21987654321"

func validUTR(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 10 {
		return false
	}
	sum := 0
	for i, w := range []int{6, 7, 8, 9, 10, 5, 4, 3, 2} {
		sum += w * d[i+1]
	}
	return int(utrCheck[sum%11]-'0') == d[0]
}

const upnAlphabet = "ABCDEFGHJKLMNPQRTUVWXYZ0123456789"

// upnAuthorities are the local authority numbers that issue pupil numbers.
var upnAuthorities = map[string]bool{}

func init() {
	for _, la := range strings.Fields(`201 202 203 204 205 206 207 208 209 210 211 212 213 301 302 303 304 305 306 307
		308 309 310 311 312 313 314 315 316 317 318 319 320 330 331 332 333 334 335 336 340 341 342 343 344 350 351
		352 353 354 355 356 357 358 359 370 371 372 373 380 381 382 383 384 390 391 392 393 394 420 800 801 802 803
		805 806 807 808 810 811 812 813 815 816 821 822 823 825 826 830 831 835 836 837 840 841 845 846 850 851 852
		855 856 857 860 861 865 866 867 868 869 870 871 872 873 874 876 877 878 879 880 881 882 883 884 885 886 887
		888 889 890 891 892 893 894 895 896 908 909 916 919 921 925 926 928 929 931 933 935 936 937 938`) {
		upnAuthorities[la] = true
	}
}

func validUPN(s string) bool {
	if len(s) != 13 || !upnAuthorities[s[1:4]] {
		return false
	}
	sum := 0
	for i := 1; i < 13; i++ {
		v := strings.IndexByte(upnAlphabet, s[i])
		if v < 0 {
			return false
		}
		sum += v * (i + 1)
	}
	return upnAlphabet[sum%23] == s[0]
}

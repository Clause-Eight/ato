package detectors

import (
	"crypto/sha256"
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// Cryptocurrency wallet addresses that carry their own checksum. A wallet address is pseudonymous personal
// data once it can be tied to a person (payments, KYC records, support tickets).
//
//   - Bitcoin P2PKH (1...) and P2SH (3...): Base58Check, 25 bytes of version (0x00 or 0x05), a 20-byte hash and
//     the first four bytes of a double SHA-256 over the first 21 bytes.
//     https://en.bitcoin.it/wiki/Base58Check_encoding
//   - Bitcoin native segwit (bc1...): Bech32 for witness version 0 (BIP-173) and Bech32m for versions 1 to 16
//     (BIP-350): a BCH checksum over the expanded "bc" prefix and the 5-bit data; the program is 2 to 40 bytes,
//     20 or 32 for version 0; upper and lower case are never mixed.
//     https://github.com/bitcoin/bips/blob/master/bip-0173.mediawiki
//     https://github.com/bitcoin/bips/blob/master/bip-0350.mediawiki
//   - Ethereum (0x + 40 hex digits) in EIP-55 mixed case: each letter is upper case exactly when the matching
//     hex digit of Keccak-256 over the lower-case address is 8 or more. https://eips.ethereum.org/EIPS/eip-55
//
// All-lower or all-upper Ethereum addresses have no checksum and look like any 160-bit hash, so they are
// not reported. The checksums are 32 bits (Base58Check), 30 bits (Bech32) and about 15 bits (EIP-55, one bit
// per letter), so no keyword is required; the detector is off by default.
func init() {
	Register(Detector{
		Name:        "crypto-wallet",
		Label:       "Cryptocurrency wallet address (Bitcoin, Ethereum)",
		Country:     "EU",
		OnByDefault: false,
		Pattern:     regexp.MustCompile(`[13][1-9A-HJ-NP-Za-km-z]{25,34}|(?:bc1|BC1)[02-9ac-hj-np-zAC-HJ-NP-Z]{11,71}|0x[0-9a-fA-F]{40}`),
		Validate:    validWallet,
		Keywords:    []string{"wallet", "bitcoin", "btc", "ethereum", "eth", "crypto address", "wallet address"},
		Base:        domain.Medium,
		Validation:  "address checksum is valid",
		Needs:       Needs{Check: longAlnumRun(26)},
	})
}

// longAlnumRun reports whether a line holds n or more letters and digits in a row.
func longAlnumRun(n int) func(string) bool {
	return func(line string) bool {
		run := 0
		for i := 0; i < len(line); i++ {
			c := line[i]
			if c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
				if run++; run >= n {
					return true
				}
			} else {
				run = 0
			}
		}
		return false
	}
}

func validWallet(s string) bool {
	switch {
	case strings.HasPrefix(s, "0x"):
		return validEIP55(s[2:])
	case strings.HasPrefix(s, "bc1") || strings.HasPrefix(s, "BC1"):
		return validSegwit(s)
	case strings.HasPrefix(s, "1") || strings.HasPrefix(s, "3"):
		return validBase58Check(s)
	}
	return false
}

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// base58Decode turns a Base58 string into bytes; each leading '1' is a leading zero byte.
func base58Decode(s string) ([]byte, bool) {
	var num []byte // big-endian
	for i := 0; i < len(s); i++ {
		v := strings.IndexByte(base58Alphabet, s[i])
		if v < 0 {
			return nil, false
		}
		carry := v
		for j := len(num) - 1; j >= 0; j-- {
			carry += int(num[j]) * 58
			num[j] = byte(carry)
			carry >>= 8
		}
		for carry > 0 {
			num = append([]byte{byte(carry)}, num...)
			carry >>= 8
		}
	}
	zeros := 0
	for zeros < len(s) && s[zeros] == '1' {
		zeros++
	}
	return append(make([]byte, zeros), num...), true
}

func validBase58Check(s string) bool {
	b, ok := base58Decode(s)
	if !ok || len(b) != 25 {
		return false
	}
	if (s[0] == '1') != (b[0] == 0x00) || (s[0] == '3') != (b[0] == 0x05) {
		return false
	}
	first := sha256.Sum256(b[:21])
	second := sha256.Sum256(first[:])
	return string(second[:4]) == string(b[21:])
}

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

const (
	bech32Const  = 1
	bech32mConst = 0x2bc830a3
)

func bech32Polymod(values []byte) uint32 {
	gen := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := uint32(1)
	for _, v := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint32(v)
		for i := 0; i < 5; i++ {
			if top>>i&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

func validSegwit(s string) bool {
	if strings.ToLower(s) != s && strings.ToUpper(s) != s {
		return false
	}
	s = strings.ToLower(s)
	if len(s) < 14 || len(s) > 74 || !strings.HasPrefix(s, "bc1") {
		return false
	}
	hrp, data := s[:2], s[3:]
	values := make([]byte, 0, 2*len(hrp)+1+len(data))
	for i := 0; i < len(hrp); i++ {
		values = append(values, hrp[i]>>5)
	}
	values = append(values, 0)
	for i := 0; i < len(hrp); i++ {
		values = append(values, hrp[i]&31)
	}
	five := make([]byte, len(data))
	for i := 0; i < len(data); i++ {
		v := strings.IndexByte(bech32Charset, data[i])
		if v < 0 {
			return false
		}
		five[i] = byte(v)
	}
	values = append(values, five...)
	version := five[0]
	if version > 16 {
		return false
	}
	want := uint32(bech32mConst)
	if version == 0 {
		want = bech32Const
	}
	if bech32Polymod(values) != want {
		return false
	}
	program, ok := fiveToEight(five[1 : len(five)-6])
	if !ok || len(program) < 2 || len(program) > 40 {
		return false
	}
	return version != 0 || len(program) == 20 || len(program) == 32
}

// fiveToEight regroups 5-bit values into bytes; leftover bits must be fewer than five and zero.
func fiveToEight(in []byte) ([]byte, bool) {
	acc, nbits := 0, 0
	var out []byte
	for _, v := range in {
		acc = acc<<5 | int(v)
		nbits += 5
		for nbits >= 8 {
			nbits -= 8
			out = append(out, byte(acc>>nbits))
		}
		acc &= 1<<nbits - 1
	}
	if nbits >= 5 || acc != 0 {
		return nil, false
	}
	return out, true
}

// validEIP55 checks the mixed-case checksum of 40 hex digits; an address with no letters of both cases has none.
func validEIP55(hex string) bool {
	if len(hex) != 40 {
		return false
	}
	lower := strings.ToLower(hex)
	if hex == lower || hex == strings.ToUpper(hex) {
		return false
	}
	hash := keccak256([]byte(lower))
	for i := 0; i < 40; i++ {
		c := hex[i]
		nibble := hash[i/2] >> 4
		if i%2 == 1 {
			nibble = hash[i/2] & 0x0f
		}
		switch {
		case c >= 'a' && c <= 'f':
			if nibble >= 8 {
				return false
			}
		case c >= 'A' && c <= 'F':
			if nibble < 8 {
				return false
			}
		case c < '0' || c > '9':
			return false
		}
	}
	return true
}

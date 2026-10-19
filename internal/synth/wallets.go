package synth

import (
	"crypto/sha256"
	"math/rand"
	"strings"
)

func init() {
	Register("crypto-wallet", func(r *rand.Rand) string {
		switch r.Intn(4) {
		case 0:
			return base58Check(0x00, randomBytes(r, 20))
		case 1:
			return base58Check(0x05, randomBytes(r, 20))
		case 2:
			if r.Intn(2) == 0 {
				return segwit(0, randomBytes(r, 20))
			}
			return segwit(byte(r.Intn(2)), randomBytes(r, 32))
		}
		return eip55(r)
	})
}

func randomBytes(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.Intn(256))
	}
	return b
}

// base58Check encodes version, payload and the first four bytes of a double SHA-256.
func base58Check(version byte, payload []byte) string {
	data := append([]byte{version}, payload...)
	h1 := sha256.Sum256(data)
	h2 := sha256.Sum256(h1[:])
	data = append(data, h2[:4]...)
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	// Repeated division of the big-endian number by 58.
	num := append([]byte{}, data...)
	var out []byte
	for len(num) > 0 {
		var quotient []byte
		rem := 0
		for _, b := range num {
			acc := rem*256 + int(b)
			if q := acc / 58; q > 0 || len(quotient) > 0 {
				quotient = append(quotient, byte(q))
			}
			rem = acc % 58
		}
		out = append(out, alphabet[rem])
		num = quotient
	}
	for _, b := range data {
		if b != 0 {
			break
		}
		out = append(out, '1')
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

// segwit writes a mainnet bc1 address: Bech32 for version 0, Bech32m after.
func segwit(version byte, program []byte) string {
	var data []byte
	data = append(data, version)
	acc, n := 0, 0
	for _, b := range program {
		acc = acc<<8 | int(b)
		n += 8
		for n >= 5 {
			n -= 5
			data = append(data, byte(acc>>n&31))
		}
	}
	if n > 0 {
		data = append(data, byte(acc<<(5-n)&31))
	}
	constant := uint32(1)
	if version > 0 {
		constant = 0x2bc830a3
	}
	values := []byte{3, 3, 0, 2, 3} // "bc": high bits 3,3, a zero, low bits 2,3
	values = append(values, data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	mod := polymod(values) ^ constant
	for i := 0; i < 6; i++ {
		data = append(data, byte(mod>>(5*(5-i))&31))
	}
	const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	var b strings.Builder
	b.WriteString("bc1")
	for _, v := range data {
		b.WriteByte(charset[v])
	}
	return b.String()
}

func polymod(values []byte) uint32 {
	gens := []uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	c := uint32(1)
	for _, v := range values {
		b := c >> 25
		c = (c&0x1ffffff)<<5 ^ uint32(v)
		for i, g := range gens {
			if b&(1<<i) != 0 {
				c ^= g
			}
		}
	}
	return c
}

// eip55 writes a random Ethereum address in EIP-55 mixed case, retrying until it has letters of both cases.
func eip55(r *rand.Rand) string {
	const hexDigits = "0123456789abcdef"
	for {
		lower := make([]byte, 40)
		for i := range lower {
			lower[i] = hexDigits[r.Intn(16)]
		}
		h := keccak256(lower)
		out := make([]byte, 40)
		upper, small := false, false
		for i, c := range lower {
			nib := h[i/2]
			if i%2 == 0 {
				nib >>= 4
			}
			nib &= 15
			switch {
			case c >= 'a' && nib >= 8:
				out[i] = c - 'a' + 'A'
				upper = true
			case c >= 'a':
				out[i] = c
				small = true
			default:
				out[i] = c
			}
		}
		if upper && small {
			return "0x" + string(out)
		}
	}
}

// keccak256 is the original Keccak with rate 1088 and padding 0x01. It derives the round constants (from the
// LFSR) and the rotation offsets (from the (x,y) walk) as the Keccak reference describes, instead of using
// tables, so it is independent of the detector's implementation.
func keccak256(msg []byte) [32]byte {
	var st [5][5]uint64
	const rate = 136
	padded := append([]byte{}, msg...)
	padded = append(padded, 0x01)
	for len(padded)%rate != 0 {
		padded = append(padded, 0)
	}
	padded[len(padded)-1] |= 0x80
	for off := 0; off < len(padded); off += rate {
		for i := 0; i < rate/8; i++ {
			var lane uint64
			for j := 0; j < 8; j++ {
				lane |= uint64(padded[off+8*i+j]) << (8 * j)
			}
			st[i%5][i/5] ^= lane
		}
		permute(&st)
	}
	var out [32]byte
	for i := 0; i < 32; i++ {
		lane := st[(i/8)%5][(i/8)/5]
		out[i] = byte(lane >> (8 * (i % 8)))
	}
	return out
}

func rcBit(t int) uint64 {
	if t%255 == 0 {
		return 1
	}
	r := uint16(1)
	for i := 0; i < t%255; i++ {
		r <<= 1
		if r&0x100 != 0 {
			r ^= 0x171
		}
	}
	return uint64(r & 1)
}

func rotl(v uint64, n int) uint64 {
	n %= 64
	if n == 0 {
		return v
	}
	return v<<n | v>>(64-n)
}

func permute(a *[5][5]uint64) {
	for round := 0; round < 24; round++ {
		var c [5]uint64
		for x := 0; x < 5; x++ {
			c[x] = a[x][0] ^ a[x][1] ^ a[x][2] ^ a[x][3] ^ a[x][4]
		}
		for x := 0; x < 5; x++ {
			d := c[(x+4)%5] ^ rotl(c[(x+1)%5], 1)
			for y := 0; y < 5; y++ {
				a[x][y] ^= d
			}
		}
		// rho and pi together: walk (x,y) -> (y, 2x+3y), the offset of step t is (t+1)(t+2)/2.
		var b [5][5]uint64
		b[0][0] = a[0][0]
		x, y := 1, 0
		for t := 0; t < 24; t++ {
			nx, ny := y, (2*x+3*y)%5
			b[nx][ny] = rotl(a[x][y], (t+1)*(t+2)/2)
			x, y = nx, ny
		}
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				a[x][y] = b[x][y] ^ (^b[(x+1)%5][y] & b[(x+2)%5][y])
			}
		}
		var rc uint64
		for j := 0; j < 7; j++ {
			rc |= rcBit(j+7*round) << ((1 << j) - 1)
		}
		a[0][0] ^= rc
	}
}

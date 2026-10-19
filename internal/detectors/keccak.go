package detectors

import "math/bits"

// Keccak-256 as used by Ethereum: the original Keccak submission with rate 1088 bits, capacity 512 bits and
// the padding byte 0x01 (FIPS 202 SHA3-256 uses 0x06 instead). Written from the Keccak team's
// specification summary, https://keccak.team/keccak_specs_summary.html.

var keccakRC = [24]uint64{
	0x0000000000000001, 0x0000000000008082, 0x800000000000808A, 0x8000000080008000,
	0x000000000000808B, 0x0000000080000001, 0x8000000080008081, 0x8000000000008009,
	0x000000000000008A, 0x0000000000000088, 0x0000000080008009, 0x000000008000000A,
	0x000000008000808B, 0x800000000000008B, 0x8000000000008089, 0x8000000000008003,
	0x8000000000008002, 0x8000000000000080, 0x000000000000800A, 0x800000008000000A,
	0x8000000080008081, 0x8000000000008080, 0x0000000080000001, 0x8000000080008008,
}

// keccakRot is the rotation offset r[x][y].
var keccakRot = [5][5]int{
	{0, 36, 3, 41, 18},
	{1, 44, 10, 45, 2},
	{62, 6, 43, 15, 61},
	{28, 55, 25, 21, 56},
	{27, 20, 39, 8, 14},
}

// keccakF1600 permutes the state; lane (x, y) is a[x+5*y].
func keccakF1600(a *[25]uint64) {
	var b [25]uint64
	var c, d [5]uint64
	for round := 0; round < 24; round++ {
		for x := 0; x < 5; x++ {
			c[x] = a[x] ^ a[x+5] ^ a[x+10] ^ a[x+15] ^ a[x+20]
		}
		for x := 0; x < 5; x++ {
			d[x] = c[(x+4)%5] ^ bits.RotateLeft64(c[(x+1)%5], 1)
		}
		for i := range a {
			a[i] ^= d[i%5]
		}
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				b[y+5*((2*x+3*y)%5)] = bits.RotateLeft64(a[x+5*y], keccakRot[x][y])
			}
		}
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				a[x+5*y] = b[x+5*y] ^ (^b[(x+1)%5+5*y] & b[(x+2)%5+5*y])
			}
		}
		a[0] ^= keccakRC[round]
	}
}

// keccak256 hashes msg with legacy Keccak-256.
func keccak256(msg []byte) [32]byte {
	const rate = 136
	var a [25]uint64
	block := make([]byte, rate)
	for {
		n := copy(block, msg)
		msg = msg[n:]
		if n < rate {
			for i := n; i < rate; i++ {
				block[i] = 0
			}
			block[n] ^= 0x01
			block[rate-1] ^= 0x80
		}
		for i := 0; i < rate/8; i++ {
			var lane uint64
			for j := 7; j >= 0; j-- {
				lane = lane<<8 | uint64(block[8*i+j])
			}
			a[i] ^= lane
		}
		keccakF1600(&a)
		if n < rate {
			break
		}
	}
	var out [32]byte
	for i := 0; i < 4; i++ {
		for j := 0; j < 8; j++ {
			out[8*i+j] = byte(a[i] >> (8 * j))
		}
	}
	return out
}

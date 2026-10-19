package detectors

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestKeccak256(t *testing.T) {
	vectors := map[string]string{
		"":    "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
		"abc": "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45",
	}
	for in, want := range vectors {
		got := keccak256([]byte(in))
		if hex.EncodeToString(got[:]) != want {
			t.Errorf("keccak256 of a %d-byte input = %x", len(in), got)
		}
	}
	// A message longer than one 136-byte block, and one of exactly one block.
	long := make([]byte, 300)
	a, b := keccak256(long), keccak256(long[:136])
	if a == b || a == keccak256(nil) {
		t.Error("multi-block hashing is broken")
	}
}

func TestCryptoWallet(t *testing.T) {
	d := detector(t, "crypto-wallet")
	cases := map[string]bool{
		// EIP-55 test vectors from the specification.
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed": true,
		"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359": true,
		"0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB": true,
		"0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb": true,
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAeD": false, // one letter's case flipped
		"0x52908400098527886E0F7030069857D2E4169EE7": false, // all upper case: no checksum
		"0xde709f2102306220921060314715629080e2fb77": false, // all lower case: no checksum
		// BIP-173 and BIP-350 test vectors.
		"BC1QW508D6QEJXTDG4Y5R3ZARVARY0C5XW7KV8F3T4":                     true,
		"bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4":                     true,
		"bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t5":                     false,
		"bc1qrp33g0q5c5txsp9arysrx4k6zdkfs4nce4xj0gdcccefvpysxf3qccfmv3": true,
		"bc1p0xlxvlhemja6c4dqv22uapctqupfhlxm9h8z3k2e72q4k9hcz7vqzk5jj0": true,
		"bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kemeawh":                     false, // v0 with the Bech32m constant
		"bc1QW508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4":                     false, // mixed case
	}
	for i, v := range sortedKeys(cases) {
		want := cases[v]
		if got := d.Validate(v); got != want {
			t.Errorf("wallet case %d: valid = %v, want %v", i, got, want)
		}
	}
	// Base58Check: build one from a synthetic payload and break it.
	p2pkh := testBase58Check(0x00, make([]byte, 20))
	if p2pkh != "1111111111111111111114oLvT2" || !d.Validate(p2pkh) {
		t.Errorf("the all-zero P2PKH address is not valid")
	}
	if d.Validate(p2pkh[:len(p2pkh)-1] + "3") {
		t.Error("a changed Base58Check character is accepted")
	}
	if d.Validate("3" + p2pkh[1:]) {
		t.Error("a P2PKH payload behind a 3 is accepted")
	}
	if !longAlnumRun(26)("to 0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed") || longAlnumRun(26)("commit: 1f2e3d4c5b6a7980") {
		t.Error("the wallet prefilter is wrong")
	}
}

// testBase58Check encodes the way the Bitcoin wiki describes, for the test above.
func testBase58Check(version byte, payload []byte) string {
	data := append([]byte{version}, payload...)
	sum := doubleSHA(data)
	data = append(data, sum[:4]...)
	n := make([]byte, len(data))
	copy(n, data)
	out := ""
	for !allZero(n) {
		rem := 0
		for i := range n {
			cur := rem*256 + int(n[i])
			n[i] = byte(cur / 58)
			rem = cur % 58
		}
		out = string(base58Alphabet[rem]) + out
	}
	for _, b := range data {
		if b != 0 {
			break
		}
		out = "1" + out
	}
	return out
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

func doubleSHA(b []byte) [32]byte {
	first := sha256.Sum256(b)
	return sha256.Sum256(first[:])
}

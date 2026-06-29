package statsig

import (
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"testing"
	"time"
)

// refHEX is the genuine seed[5]%4=2 fingerprint for the embedded default seed,
// used by the byte-exact cross-check.
const refHEX = defaultHEX

// TestGenerateValid proves Generate yields a well-formed 70-byte statsig that is
// internally self-consistent — exactly what grok recomputes and validates.
func TestGenerateValid(t *testing.T) {
	out, err := Generate("/rest/app-chat/conversations/new", "POST", time.Now().Unix())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	t.Logf("x-statsig-id = %s", out)

	raw, err := base64.RawStdEncoding.DecodeString(out)
	if err != nil || len(raw) != 70 {
		t.Fatalf("bad statsig (%d bytes): %v", len(raw), err)
	}
	key := raw[0]
	seed := mustDecodeSeed(defaultSeedB64)
	for i := 0; i < 48; i++ {
		if raw[1+i]^key != seed[i] {
			t.Fatalf("seed byte %d mismatch", i)
		}
	}
	number := uint32(raw[49]^key) | uint32(raw[50]^key)<<8 | uint32(raw[51]^key)<<16 | uint32(raw[52]^key)<<24
	input := "POST!/rest/app-chat/conversations/new!" + strconv.FormatUint(uint64(number), 10) + statsigSalt + curHEX
	sum := sha256.Sum256([]byte(input))
	for i := 0; i < 16; i++ {
		if raw[53+i]^key != sum[i] {
			t.Fatalf("sha byte %d mismatch — would be code:7", i)
		}
	}
	if raw[69]^key != statsigMark {
		t.Fatalf("tail marker = %d want 3", raw[69]^key)
	}
	t.Logf("OK: valid & SHA-consistent (number=%d, key=%d)", number, key)
}

// TestTwoCallsDiffer ensures a random key per call.
func TestTwoCallsDiffer(t *testing.T) {
	a, _ := Generate("/rest/app-chat/conversations/new", "POST", time.Now().Unix())
	b, _ := Generate("/rest/app-chat/conversations/new", "POST", time.Now().Unix())
	if a == b {
		t.Fatal("two calls produced identical statsig (expected random key)")
	}
}

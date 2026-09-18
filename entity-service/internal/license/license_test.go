// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

// Synthetic keys. Every character position is identifiable, so a chunk landing
// in the wrong place is visible in the failure message rather than being a wall
// of hex. Real key material never appears in this repository.
const (
	testPrimary   = "0000aaaa1111bbbb2222cccc3333dddd4444eeee5555ffff6666aaaa7777bbbb"
	testSecondary = "8888aaaa9999bbbbaaaaccccbbbbddddccccEEEEddddffffeeeeaaaaffffbbbb"
)

// The real key's four chunks must land at exactly these positions. This is the
// property that makes a licence usable at all: a consumer recovers the signing
// key from these four positions and nowhere else.
func TestShuffle_PlacesRealKeyChunksAtKnownPositions(t *testing.T) {
	shuffled, err := Shuffle(testPrimary, testSecondary)
	if err != nil {
		t.Fatalf("Shuffle: %v", err)
	}
	primaryChunks := splitIntoChunks(testPrimary, chunksPerKey)

	for position, wantChunk := range map[int]int{1: 0, 6: 1, 10: 2, 3: 3} {
		// Undo this position's sub-chunk reorder and compare to the chunk it
		// is supposed to be carrying.
		got := make([]string, subChunksPerChunk)
		for storedIdx, originalIdx := range chunkShufflePatterns[position] {
			got[originalIdx] = shuffled[position][storedIdx]
		}
		if joined := strings.Join(got, ""); joined != primaryChunks[wantChunk] {
			t.Errorf("position %d carries %q, want primary chunk %d (%q)",
				position, joined, wantChunk, primaryChunks[wantChunk])
		}
	}
}

// The four filler positions must be fresh on every call, which is also why a
// licence cannot be verified by regenerating its secrets and comparing.
func TestShuffle_FillerPositionsAreRandomPerCall(t *testing.T) {
	first, err := Shuffle(testPrimary, testSecondary)
	if err != nil {
		t.Fatalf("Shuffle: %v", err)
	}
	second, err := Shuffle(testPrimary, testSecondary)
	if err != nil {
		t.Fatalf("Shuffle: %v", err)
	}

	for _, position := range []int{2, 5, 7, 11} {
		if strings.Join(first[position], "") == strings.Join(second[position], "") {
			t.Errorf("filler position %d repeated across calls", position)
		}
	}
	// Everything else must be identical for the same pair of keys.
	for _, position := range []int{0, 1, 3, 4, 6, 8, 9, 10} {
		if strings.Join(first[position], "") != strings.Join(second[position], "") {
			t.Errorf("position %d is not deterministic for a fixed key pair", position)
		}
	}
}

func TestFlatten_LengthAndComposition(t *testing.T) {
	shuffled, err := Shuffle(testPrimary, testSecondary)
	if err != nil {
		t.Fatalf("Shuffle: %v", err)
	}
	flat := Flatten(shuffled)
	if len(flat) != FlatSecretsLen {
		t.Fatalf("flattened length %d, want %d", len(flat), FlatSecretsLen)
	}
	// Each position occupies its own 16 characters, in order.
	for i, position := range shuffled {
		want := strings.Join(position, "")
		if got := flat[i*16 : (i+1)*16]; got != want {
			t.Errorf("flat position %d = %q, want %q", i, got, want)
		}
	}
}

// The round trip is the contract: whatever a licence carries, the signing key
// must come back out of it.
func TestRecoverPrimaryKey_RoundTrip(t *testing.T) {
	primary, secondary, secrets, err := GenerateSecrets()
	if err != nil {
		t.Fatalf("GenerateSecrets: %v", err)
	}
	if primary == secondary {
		t.Fatal("the decoy key must be generated independently of the real one")
	}
	if len(secrets) != FlatSecretsLen {
		t.Fatalf("secrets length %d, want %d", len(secrets), FlatSecretsLen)
	}

	recovered, err := RecoverPrimaryKey(secrets)
	if err != nil {
		t.Fatalf("RecoverPrimaryKey: %v", err)
	}
	if recovered != primary {
		t.Fatalf("recovered %q, want %q", recovered, primary)
	}
	if strings.Contains(secrets, secondary) {
		t.Error("the decoy key should be split across positions, not stored contiguously")
	}
}

func TestGenerateKey_ShapeAndUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		k, err := GenerateKey()
		if err != nil {
			t.Fatalf("GenerateKey: %v", err)
		}
		if len(k) != keyHexLen {
			t.Fatalf("key length %d, want %d", len(k), keyHexLen)
		}
		if _, err := hex.DecodeString(k); err != nil {
			t.Fatalf("key is not hex: %v", err)
		}
		if seen[k] {
			t.Fatal("GenerateKey repeated a value")
		}
		seen[k] = true
	}
}

// Canonicalization is key-then-value with no separators, keys sorted
// lexicographically. The absence of separators is what makes this fragile, so
// it is pinned exactly rather than described.
func TestCanonicalize_SortsKeysAndConcatenatesWithoutSeparators(t *testing.T) {
	got, err := Canonicalize(map[string]any{
		"subscriptionKey":        "SUBKEY",
		"clientId":               "CID",
		"deploymentId":           "42",
		"clientSecret":           "CSECRET",
		"usageDataPublishingUrl": "https://example.invalid/usage",
		"deploymentName":         "Development",
		"secrets":                "abcd",
	})
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	const want = "clientIdCID" +
		"clientSecretCSECRET" +
		"deploymentId42" +
		"deploymentNameDevelopment" +
		"secretsabcd" +
		"subscriptionKeySUBKEY" +
		"usageDataPublishingUrlhttps://example.invalid/usage"
	if got != want {
		t.Errorf("canonical string =\n  %q\nwant\n  %q", got, want)
	}
}

// Sorting is lexicographic, not numeric — secretKey10 and secretKey11 fall
// between secretKey1 and secretKey2. This only bites for the superseded object
// form of secrets, but the rule is the same for any nested object.
func TestCanonicalize_LexicographicNotNumericOrdering(t *testing.T) {
	got, err := Canonicalize(map[string]any{
		"secretKey0":  "a",
		"secretKey1":  "b",
		"secretKey2":  "c",
		"secretKey10": "d",
		"secretKey11": "e",
	})
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	const want = "secretKey0asecretKey1bsecretKey10dsecretKey11esecretKey2c"
	if got != want {
		t.Errorf("canonical string = %q, want %q", got, want)
	}
}

func TestCanonicalize_JSONEncodesNonStringValues(t *testing.T) {
	got, err := Canonicalize(map[string]any{
		"secrets": map[string]any{"secretKey0": []string{"aaaa", "bbbb"}},
		"active":  true,
	})
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	const want = `activetruesecrets{"secretKey0":["aaaa","bbbb"]}`
	if got != want {
		t.Errorf("canonical string = %q, want %q", got, want)
	}
}

// The MAC is keyed on the key's raw 32 bytes. ServiceNow's generateMac accepts
// that key base64-encoded, but base64 is the transport, not the MAC input —
// keying on the hex text or on its base64 form yields a signature no customer
// product accepts. The expected value here is built independently of Sign so a
// change to either the canonical string or the key handling fails this test.
func TestSign_KeysOnRawKeyBytesOverTheCanonicalString(t *testing.T) {
	payload := map[string]any{"deploymentId": "42", "secrets": "abcd"}

	raw, err := hex.DecodeString(testPrimary)
	if err != nil {
		t.Fatalf("decode test key: %v", err)
	}
	mac := hmac.New(sha256.New, raw)
	mac.Write([]byte("deploymentId42secretsabcd"))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	got, err := Sign(payload, testPrimary)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if got != want {
		t.Fatalf("signature = %q, want %q", got, want)
	}

	// Keying on the hex text instead of the decoded bytes is the most likely
	// wrong turn; make sure it produces something different.
	wrong := hmac.New(sha256.New, []byte(testPrimary))
	wrong.Write([]byte("deploymentId42secretsabcd"))
	if got == base64.StdEncoding.EncodeToString(wrong.Sum(nil)) {
		t.Error("signature matches a MAC keyed on the hex text, not the raw key bytes")
	}
}

func TestVerify_AcceptsOwnSignatureAndRejectsTampering(t *testing.T) {
	payload := map[string]any{
		"deploymentId":   "42",
		"deploymentName": "Development",
		"secrets":        "abcd",
	}
	sig, err := Sign(payload, testPrimary)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	ok, err := Verify(payload, testPrimary, sig)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Fatal("a freshly signed payload must verify")
	}

	// Any change to a signed field must invalidate the signature — this is the
	// whole point of signing, and the reason a dropped field breaks a licence.
	tampered := map[string]any{
		"deploymentId":   "43",
		"deploymentName": "Development",
		"secrets":        "abcd",
	}
	if ok, _ := Verify(tampered, testPrimary, sig); ok {
		t.Error("a modified payload must not verify")
	}

	// A field removed in transit is the failure this whole design guards
	// against: the verifier computes a different canonical string.
	missing := map[string]any{"deploymentId": "42", "secrets": "abcd"}
	if ok, _ := Verify(missing, testPrimary, sig); ok {
		t.Error("a payload missing a signed field must not verify")
	}
}

// A licence signs with the key its own secrets carry, so the two must agree.
func TestEndToEnd_SecretsCarryTheKeyThatSigns(t *testing.T) {
	primary, _, secrets, err := GenerateSecrets()
	if err != nil {
		t.Fatalf("GenerateSecrets: %v", err)
	}
	payload := map[string]any{
		"deploymentId":           "42",
		"deploymentName":         "Development",
		"subscriptionKey":        "SUBKEY",
		"clientId":               "CID",
		"clientSecret":           "CSECRET",
		"usageDataPublishingUrl": "https://example.invalid/usage",
		"secrets":                secrets,
	}
	sig, err := Sign(payload, primary)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Exactly what a customer's product does: recover the key from the licence
	// it holds, then verify the signature with it.
	recovered, err := RecoverPrimaryKey(secrets)
	if err != nil {
		t.Fatalf("RecoverPrimaryKey: %v", err)
	}
	ok, err := Verify(payload, recovered, sig)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Fatal("a licence must verify under the key recovered from its own secrets")
	}
}

func TestShuffle_RejectsWrongLengthKeys(t *testing.T) {
	if _, err := Shuffle("tooshort", testSecondary); err == nil {
		t.Error("expected an error for a short primary key")
	}
	if _, err := Shuffle(testPrimary, "tooshort"); err == nil {
		t.Error("expected an error for a short secondary key")
	}
}

func TestSign_RejectsMalformedKey(t *testing.T) {
	payload := map[string]any{"deploymentId": "42"}
	if _, err := Sign(payload, "tooshort"); err == nil {
		t.Error("expected an error for a short key")
	}
	if _, err := Sign(payload, strings.Repeat("z", keyHexLen)); err == nil {
		t.Error("expected an error for a non-hex key")
	}
}

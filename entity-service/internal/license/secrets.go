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

// Package license implements the deployment licence file's secret obfuscation
// and signature, mirroring the ServiceNow script includes that produce them
// today.
//
// The output is a live contract with deployed customer products: they hold
// licences issued by ServiceNow and verify the signature by recomputing it. Any
// deviation here — a different shuffle position, a different canonical string,
// a different key encoding — produces a licence those products reject. Treat
// the constants in this file as fixed data, not as choices.
package license

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// secretKeyBytes is the size of each generated key. Hex-encoded, that is
	// keyHexLen characters.
	secretKeyBytes = 32
	// keyHexLen is the hex-encoded length of one key: 64 characters.
	keyHexLen = secretKeyBytes * 2
	// chunksPerKey splits each key into 4 chunks of chunkLen characters.
	chunksPerKey = 4
	// chunkLen is the length of one chunk: 16 characters.
	chunkLen = keyHexLen / chunksPerKey
	// subChunksPerChunk splits each chunk into 4 sub-chunks of subChunkLen.
	subChunksPerChunk = 4
	// subChunkLen is the length of one sub-chunk: 4 characters.
	subChunkLen = chunkLen / subChunksPerChunk
	// positions is the number of secretKeyN entries the shuffle produces.
	positions = 12
	// FlatSecretsLen is the length of the flattened secrets string:
	// 12 positions x 4 sub-chunks x 4 characters.
	FlatSecretsLen = positions * subChunksPerChunk * subChunkLen
)

// source identifies where one shuffle position's chunk comes from.
type source int

const (
	fromPrimary source = iota
	fromSecondary
	fromRandom
)

// shuffleEntry is one position in the shuffle map.
type shuffleEntry struct {
	src source
	// chunk is the index into the source key's chunks; ignored for fromRandom.
	chunk int
}

// shuffleMap mixes the real (primary) and decoy (secondary) key chunks with
// random filler across the 12 positions. Positions 1, 6, 10 and 3 carry the
// real key's chunks 0, 1, 2 and 3 respectively — that is what makes the primary
// key recoverable from a licence, and why the flattened result must be treated
// as secret-equivalent rather than as a public artefact.
var shuffleMap = [positions]shuffleEntry{
	0:  {fromSecondary, 1},
	1:  {fromPrimary, 0},
	2:  {fromRandom, 0},
	3:  {fromPrimary, 3},
	4:  {fromSecondary, 0},
	5:  {fromRandom, 0},
	6:  {fromPrimary, 1},
	7:  {fromRandom, 0},
	8:  {fromSecondary, 3},
	9:  {fromSecondary, 2},
	10: {fromPrimary, 2},
	11: {fromRandom, 0},
}

// chunkShufflePatterns reorders each position's 4 sub-chunks. Read as: the
// sub-chunk emitted at index i is the original sub-chunk at patterns[n][i].
var chunkShufflePatterns = [positions][subChunksPerChunk]int{
	0:  {2, 0, 3, 1},
	1:  {1, 3, 0, 2},
	2:  {0, 2, 1, 3},
	3:  {3, 1, 2, 0},
	4:  {2, 3, 0, 1},
	5:  {1, 0, 3, 2},
	6:  {3, 0, 1, 2},
	7:  {0, 3, 2, 1},
	8:  {2, 1, 3, 0},
	9:  {1, 2, 0, 3},
	10: {2, 0, 1, 3},
	11: {0, 1, 2, 3},
}

// hexAlphabet is the character set the random filler is drawn from, so filler
// is indistinguishable from real key material by inspection.
const hexAlphabet = "0123456789abcdef"

// GenerateKey returns a cryptographically random 32-byte key, hex-encoded.
//
// Two are needed per deployment: a primary (the real key) and a secondary (a
// decoy). They are generated independently — the secondary is not derived from
// the primary.
func GenerateKey() (string, error) {
	b := make([]byte, secretKeyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("license: generate key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// randomFiller returns n random hex characters for a decoy position.
func randomFiller(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("license: generate filler: %w", err)
	}
	var sb strings.Builder
	sb.Grow(n)
	for _, v := range b {
		sb.WriteByte(hexAlphabet[int(v)%len(hexAlphabet)])
	}
	return sb.String(), nil
}

// splitIntoChunks divides s into n equal chunks.
func splitIntoChunks(s string, n int) []string {
	size := (len(s) + n - 1) / n // ceil, matching the reference implementation
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		start := i * size
		if start >= len(s) {
			out = append(out, "")
			continue
		}
		end := min(start+size, len(s))
		out = append(out, s[start:end])
	}
	return out
}

// shuffleChunk splits a chunk into 4 sub-chunks and reorders them by pattern.
func shuffleChunk(chunk string, pattern [subChunksPerChunk]int) []string {
	sub := splitIntoChunks(chunk, subChunksPerChunk)
	out := make([]string, subChunksPerChunk)
	for i, idx := range pattern {
		out[i] = sub[idx]
	}
	return out
}

// Shuffle mixes the two keys into the 12 shuffle positions, returning each
// position's 4 reordered sub-chunks.
//
// Positions 2, 5, 7 and 11 are fresh random filler on every call, so the result
// is NOT reproducible from the two keys alone — a licence cannot be verified by
// regenerating its secrets and comparing. Only the real-key positions are
// stable, and those are all a consumer needs.
func Shuffle(primaryKey, secondaryKey string) ([positions][]string, error) {
	var out [positions][]string
	if len(primaryKey) != keyHexLen || len(secondaryKey) != keyHexLen {
		return out, fmt.Errorf("license: keys must be %d hex characters, got %d and %d",
			keyHexLen, len(primaryKey), len(secondaryKey))
	}

	primary := splitIntoChunks(primaryKey, chunksPerKey)
	secondary := splitIntoChunks(secondaryKey, chunksPerKey)

	for i, entry := range shuffleMap {
		var chunk string
		switch entry.src {
		case fromPrimary:
			chunk = primary[entry.chunk]
		case fromSecondary:
			chunk = secondary[entry.chunk]
		case fromRandom:
			filler, err := randomFiller(chunkLen)
			if err != nil {
				return out, err
			}
			chunk = filler
		}
		out[i] = shuffleChunk(chunk, chunkShufflePatterns[i])
	}
	return out, nil
}

// Flatten concatenates the shuffled positions into the single hex string the
// licence file carries.
//
// This replaces the earlier {"secretKey0": [...], ...} object form. Note that
// the change is not cosmetic: the signature is computed over the canonicalised
// payload, so flattening changes the canonical string and therefore every
// signature. A verifier built for the object form will reject a licence built
// this way, and vice versa.
func Flatten(shuffled [positions][]string) string {
	var sb strings.Builder
	sb.Grow(FlatSecretsLen)
	for _, position := range shuffled {
		for _, sub := range position {
			sb.WriteString(sub)
		}
	}
	return sb.String()
}

// GenerateSecrets generates both keys, shuffles them and returns the flattened
// secrets string alongside the keys themselves, which must be stored so the
// licence can be reissued.
func GenerateSecrets() (primaryKey, secondaryKey, secrets string, err error) {
	if primaryKey, err = GenerateKey(); err != nil {
		return "", "", "", err
	}
	if secondaryKey, err = GenerateKey(); err != nil {
		return "", "", "", err
	}
	shuffled, err := Shuffle(primaryKey, secondaryKey)
	if err != nil {
		return "", "", "", err
	}
	return primaryKey, secondaryKey, Flatten(shuffled), nil
}

// RecoverPrimaryKey reconstructs the real key from a flattened secrets string,
// reversing the shuffle.
//
// It exists because the relationship has to be testable in both directions: a
// licence whose secrets do not yield back the key that signed it is broken, and
// this is the only way to assert that. It is also a plain statement of what the
// obfuscation does and does not provide — anyone holding the flattened string
// and this file's constants can do exactly this.
func RecoverPrimaryKey(secrets string) (string, error) {
	if len(secrets) != FlatSecretsLen {
		return "", fmt.Errorf("license: secrets must be %d characters, got %d", FlatSecretsLen, len(secrets))
	}
	positionLen := subChunksPerChunk * subChunkLen

	chunks := make([]string, chunksPerKey)
	for i, entry := range shuffleMap {
		if entry.src != fromPrimary {
			continue
		}
		stored := secrets[i*positionLen : (i+1)*positionLen]
		sub := splitIntoChunks(stored, subChunksPerChunk)

		// Undo the reorder: the sub-chunk stored at index i belongs at
		// pattern[i] in the original chunk.
		original := make([]string, subChunksPerChunk)
		for storedIdx, originalIdx := range chunkShufflePatterns[i] {
			original[originalIdx] = sub[storedIdx]
		}
		chunks[entry.chunk] = strings.Join(original, "")
	}

	key := strings.Join(chunks, "")
	if len(key) != keyHexLen {
		return "", fmt.Errorf("license: recovered key is %d characters, want %d", len(key), keyHexLen)
	}
	return key, nil
}

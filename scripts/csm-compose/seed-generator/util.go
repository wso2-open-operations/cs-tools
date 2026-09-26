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
//
// Small randomness/formatting helpers shared by generate.go. Uses crypto/rand
// throughout (not math/rand): this is local-dev-only dummy data with no need
// for reproducibility across runs (the marker table in main.go is what makes
// re-running docker compose up idempotent, not a fixed seed), and crypto/rand
// keeps this package clean under gosec's weak-RNG rule (G404) with no
// suppressions needed.
package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// randIntn returns a uniform random int in [0, n). Panics if n <= 0 --
// every call site in this package passes a compile-time-known positive
// length, so a panic here would mean a real bug in this tool, not bad input.
func randIntn(n int) int {
	if n <= 0 {
		panic("randIntn: n must be positive")
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(fmt.Sprintf("randIntn: crypto/rand: %v", err))
	}
	return int(v.Int64())
}

// randRange returns a uniform random int in [min, max].
func randRange(min, max int) int {
	if max < min {
		panic("randRange: max < min")
	}
	return min + randIntn(max-min+1)
}

// randBool returns true with roughly the given probability (0..1).
func randBool(probability float64) bool {
	return randIntn(1000) < int(probability*1000)
}

// pick returns a uniformly random element of s. Panics on an empty slice --
// every call site passes one of this package's own fixed word lists.
func pick[T any](s []T) T {
	if len(s) == 0 {
		panic("pick: empty slice")
	}
	return s[randIntn(len(s))]
}

// pickN returns n distinct random elements of s (n must be <= len(s)).
func pickN[T any](s []T, n int) []T {
	if n > len(s) {
		n = len(s)
	}
	idx := make([]int, len(s))
	for i := range idx {
		idx[i] = i
	}
	// Fisher-Yates partial shuffle.
	for i := 0; i < n; i++ {
		j := i + randIntn(len(idx)-i)
		idx[i], idx[j] = idx[j], idx[i]
	}
	out := make([]T, n)
	for i := 0; i < n; i++ {
		out[i] = s[idx[i]]
	}
	return out
}

// newUUID returns a random RFC 4122 version-4 UUID string, generated from
// crypto/rand -- this package's only source of randomness (see the package
// doc comment above). Postgres's UUID column type only cares about the
// textual shape, but the version/variant bits are set anyway to keep these
// values indistinguishable from any other UUIDv4.
func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("newUUID: crypto/rand: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// daysAgo returns a timestamp n days before now, for spreading created_on/
// updated_on values across a believable recent history instead of every
// generated row sharing the same now().
func daysAgo(n int) time.Time {
	return time.Now().Add(-time.Duration(n) * 24 * time.Hour)
}

// randRecentTime returns a random timestamp between minDaysAgo and
// maxDaysAgo days in the past.
func randRecentTime(minDaysAgo, maxDaysAgo int) time.Time {
	return daysAgo(randRange(minDaysAgo, maxDaysAgo))
}

// notAfterNow clamps t to now when a randomized offset from an already-past
// timestamp (e.g. createdOn + a random hour delta) pushes it into the
// future. The schema doesn't constrain these columns, and dashboard
// consumers format/sort them directly, so a future-dated updated_on (or
// anything derived from it, like a CLOSED case's closed_on/resolved_on)
// would distort relative-time display and recency ordering.
func notAfterNow(t time.Time) time.Time {
	if now := time.Now(); t.After(now) {
		return now
	}
	return t
}

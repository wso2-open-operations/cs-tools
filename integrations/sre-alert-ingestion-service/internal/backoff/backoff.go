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

// Package backoff computes exponential retry delays for the buffer worker.
// It is pure and I/O-free by design (no clock reads beyond the "now" a
// caller passes in) so the retry-due decision is testable without a real
// Postgres — see internal/worker, which is the only caller.
package backoff

import "time"

// BaseDelay is the delay before the first retry (retryCount == 0 at the time
// of that retry's attempt, i.e. after exactly one prior failure).
const BaseDelay = 30 * time.Second

// MaxDelay caps the computed delay so a buffered alert is never retried less
// often than once every MaxDelay, no matter how many attempts have failed —
// an unbounded exponential backoff would otherwise leave a very old row
// retried for the first time in practice, hours after CSM recovers.
const MaxDelay = 30 * time.Minute

// Delay returns the backoff delay to wait after retryCount prior failed
// attempts before the next one is due: BaseDelay * 2^retryCount, capped at
// MaxDelay. retryCount is clamped to 0 for any negative input.
func Delay(retryCount int) time.Duration {
	if retryCount < 0 {
		retryCount = 0
	}
	d := BaseDelay
	for i := 0; i < retryCount; i++ {
		d *= 2
		if d >= MaxDelay {
			return MaxDelay
		}
	}
	return d
}

// Due reports whether a row with the given retryCount and lastAttemptAt (nil
// if never attempted) is due for its next attempt as of now. A row that has
// never been attempted is always due — its buffered receipt, not a prior
// attempt, is what makes it eligible.
func Due(now time.Time, lastAttemptAt *time.Time, retryCount int) bool {
	if lastAttemptAt == nil {
		return true
	}
	return !now.Before(lastAttemptAt.Add(Delay(retryCount)))
}

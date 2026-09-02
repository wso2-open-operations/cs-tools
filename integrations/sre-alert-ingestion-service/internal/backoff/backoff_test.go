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

package backoff_test

import (
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/backoff"
)

func TestDelay_ExponentialGrowth(t *testing.T) {
	t.Parallel()

	cases := []struct {
		retryCount int
		want       time.Duration
	}{
		{-5, 30 * time.Second}, // negative clamps to 0
		{0, 30 * time.Second},
		{1, 60 * time.Second},
		{2, 120 * time.Second},
		{3, 240 * time.Second},
		{4, 480 * time.Second},
		{5, 960 * time.Second}, // 16 min, still under the 30 min cap
		{6, 30 * time.Minute},  // 32 min would exceed the cap; clamped
		{20, 30 * time.Minute}, // far beyond the cap, still clamped
	}
	for _, tc := range cases {
		if got := backoff.Delay(tc.retryCount); got != tc.want {
			t.Errorf("Delay(%d) = %v, want %v", tc.retryCount, got, tc.want)
		}
	}
}

func TestDue_NeverAttemptedIsAlwaysDue(t *testing.T) {
	t.Parallel()

	now := time.Now()
	if !backoff.Due(now, nil, 0) {
		t.Error("Due() = false for a never-attempted row, want true")
	}
	if !backoff.Due(now, nil, 7) {
		t.Error("Due() = false for a never-attempted row regardless of retryCount, want true")
	}
}

func TestDue_RespectsBackoffWindow(t *testing.T) {
	t.Parallel()

	now := time.Now()
	lastAttempt := now.Add(-45 * time.Second) // 45s ago

	// retryCount 0 -> 30s delay -> due (45s > 30s elapsed).
	if !backoff.Due(now, &lastAttempt, 0) {
		t.Error("Due() = false, want true: elapsed 45s exceeds the 30s delay for retryCount 0")
	}

	// retryCount 1 -> 60s delay -> not yet due (only 45s elapsed).
	if backoff.Due(now, &lastAttempt, 1) {
		t.Error("Due() = true, want false: elapsed 45s is under the 60s delay for retryCount 1")
	}
}

func TestDue_ExactBoundaryIsDue(t *testing.T) {
	t.Parallel()

	now := time.Now()
	lastAttempt := now.Add(-30 * time.Second)
	if !backoff.Due(now, &lastAttempt, 0) {
		t.Error("Due() = false at the exact 30s boundary, want true (inclusive)")
	}
}

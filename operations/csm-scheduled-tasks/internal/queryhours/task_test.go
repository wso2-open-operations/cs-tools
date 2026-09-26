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

package queryhours

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeSweeper struct {
	res          SweepResult
	err          error
	gotStaleFor  time.Duration
	gotLimit     int
	callCount    int
	contextIsSet bool
}

func (f *fakeSweeper) Sweep(ctx context.Context, staleFor time.Duration, limit int) (SweepResult, error) {
	f.callCount++
	f.gotStaleFor = staleFor
	f.gotLimit = limit
	f.contextIsSet = ctx != nil
	return f.res, f.err
}

func TestRecomputeQueryHours_CleanSweepSucceeds(t *testing.T) {
	sw := &fakeSweeper{res: SweepResult{Requested: 12, Succeeded: 12}}
	if err := RecomputeQueryHours(sw, time.Hour, 200)(context.Background()); err != nil {
		t.Fatalf("handler returned %v, want nil", err)
	}
	if sw.callCount != 1 {
		t.Fatalf("Sweep called %d times, want 1", sw.callCount)
	}
}

// A sweep where nothing was stale is a success, not a no-op worth alerting on.
func TestRecomputeQueryHours_EmptySweepSucceeds(t *testing.T) {
	sw := &fakeSweeper{res: SweepResult{}}
	if err := RecomputeQueryHours(sw, time.Hour, 200)(context.Background()); err != nil {
		t.Fatalf("handler returned %v, want nil", err)
	}
}

// Partial failure must surface as a task failure so SUB_CRON_RECIPIENTS are
// alerted — the successful projects are already committed server-side, so
// this costs an alert rather than the run's work.
func TestRecomputeQueryHours_PartialFailureIsReported(t *testing.T) {
	sw := &fakeSweeper{res: SweepResult{
		Requested: 10, Succeeded: 8, Failed: 2,
		Errors: map[string]string{"p1": "boom", "p2": "bang"},
	}}
	err := RecomputeQueryHours(sw, time.Hour, 200)(context.Background())
	if err == nil {
		t.Fatal("handler returned nil, want an error for the 2 failed projects")
	}
	if !strings.Contains(err.Error(), "2 of 10") {
		t.Fatalf("error = %q, want it to name the failure count", err.Error())
	}
}

func TestRecomputeQueryHours_TransportErrorIsWrapped(t *testing.T) {
	sentinel := errors.New("connection refused")
	sw := &fakeSweeper{err: sentinel}
	err := RecomputeQueryHours(sw, time.Hour, 200)(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want it to wrap the transport error", err)
	}
}

// Zero/negative configuration falls back to the documented defaults rather
// than asking entity-service to sweep 0 projects.
func TestRecomputeQueryHours_AppliesDefaultsForNonPositiveConfig(t *testing.T) {
	sw := &fakeSweeper{}
	if err := RecomputeQueryHours(sw, 0, 0)(context.Background()); err != nil {
		t.Fatalf("handler returned %v, want nil", err)
	}
	if sw.gotStaleFor != DefaultStaleFor {
		t.Fatalf("staleFor = %v, want %v", sw.gotStaleFor, DefaultStaleFor)
	}
	if sw.gotLimit != DefaultLimit {
		t.Fatalf("limit = %d, want %d", sw.gotLimit, DefaultLimit)
	}
}

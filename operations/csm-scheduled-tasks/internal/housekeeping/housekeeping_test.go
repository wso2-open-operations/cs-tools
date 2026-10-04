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

package housekeeping

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeClient struct {
	cutoff time.Time
	err    error
}

func (f *fakeClient) DeleteResolvedBefore(_ context.Context, cutoff time.Time) (int, error) {
	f.cutoff = cutoff
	return 3, f.err
}

func TestCleanupResolvedRuns_CutoffIsRetentionAgo(t *testing.T) {
	f := &fakeClient{}
	before := time.Now()
	if err := CleanupResolvedRuns(f, 30*24*time.Hour)(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := before.Add(-30 * 24 * time.Hour)
	if d := f.cutoff.Sub(want); d < 0 || d > time.Minute {
		t.Fatalf("cutoff should be about 30 days ago, off by %s", d)
	}
}

func TestCleanupResolvedRuns_PropagatesError(t *testing.T) {
	if err := CleanupResolvedRuns(&fakeClient{err: errors.New("down")}, time.Hour)(context.Background()); err == nil {
		t.Fatal("a ledger error must be returned")
	}
}

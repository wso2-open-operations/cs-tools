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

package acp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeReopenChecker struct {
	pages    []reopenCheckResponse
	cursors  []string
	limits   []int
	err      error
	requests int
}

func (f *fakeReopenChecker) ReopenCheck(_ context.Context, body []byte) ([]byte, error) {
	var req reopenCheckRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	f.cursors = append(f.cursors, req.Cursor)
	f.limits = append(f.limits, req.Limit)
	if f.err != nil {
		return nil, f.err
	}
	page := f.pages[f.requests]
	f.requests++
	return json.Marshal(page)
}

func TestRunReopenCheck_FollowsTheCursorToTheEnd(t *testing.T) {
	f := &fakeReopenChecker{pages: []reopenCheckResponse{
		{Checked: 5, Reopened: 1, NextCursor: "p5", More: true},
		{Checked: 1, Finished: 1, NextCursor: "p6", More: true}, // stopped early, short of time
		{Checked: 4, NextCursor: "p10", More: true},
		{Checked: 2},
	}}
	if err := RunReopenCheck(f)(context.Background()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if strings.Join(f.cursors, ",") != ",p5,p6,p10" {
		t.Errorf("cursors = %q", f.cursors)
	}
	for _, l := range f.limits {
		if l != reopenCheckPageSize {
			t.Errorf("limit = %d, want %d", l, reopenCheckPageSize)
		}
	}
}

func TestRunReopenCheck_FailuresFailTheRunByProjectID(t *testing.T) {
	f := &fakeReopenChecker{pages: []reopenCheckResponse{
		{Checked: 5, Failed: 1, FailedProjectIDs: []string{"p3"}, NextCursor: "p5", More: true},
		{Checked: 1, Failed: 1, FailedProjectIDs: []string{"p6"}},
	}}
	err := RunReopenCheck(f)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "2 project(s) failed: p3, p6") {
		t.Fatalf("err = %v", err)
	}
	if f.requests != 2 {
		t.Errorf("a failure stopped the run early: %d pages", f.requests)
	}
}

func TestRunReopenCheck_StopsOnAnErrorOrNoProgress(t *testing.T) {
	f := &fakeReopenChecker{err: errors.New("entity-service down")}
	if err := RunReopenCheck(f)(context.Background()); err == nil || !strings.Contains(err.Error(), "entity-service down") {
		t.Errorf("err = %v", err)
	}
	f = &fakeReopenChecker{pages: []reopenCheckResponse{{Checked: 0, NextCursor: ""}}}
	f.pages[0].NextCursor = "" // first page empty: done
	if err := RunReopenCheck(f)(context.Background()); err != nil {
		t.Errorf("an empty first page: err = %v", err)
	}
	f = &fakeReopenChecker{pages: []reopenCheckResponse{{Checked: 3, NextCursor: "p3", More: true}, {Checked: 0, NextCursor: "p3", More: true}}}
	if err := RunReopenCheck(f)(context.Background()); err == nil || !strings.Contains(err.Error(), "no progress") {
		t.Errorf("a page that made no progress: err = %v", err)
	}
}

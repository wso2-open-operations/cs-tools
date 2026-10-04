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

package service

import (
	"context"
	"testing"
	"time"
)

// TestMapSNActivitiesToDomain_AcceptsAlternateDateLayout proves the activity
// feed goes through the shared datetime helper: an entry whose createdOn
// arrives in the alternate MM-DD-YYYY rendering is mapped instead of failing
// the whole page, and a malformed value still fails.
func TestMapSNActivitiesToDomain_AcceptsAlternateDateLayout(t *testing.T) {
	got, err := mapSNActivitiesToDomain(context.Background(), []snActivity{
		{ID: "a1", Type: "comment", CreatedOn: "2026-10-02 08:30:00"},
		{ID: "a2", Type: "comment", CreatedOn: "10-02-2026 08:30:00"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 10, 2, 8, 30, 0, 0, time.UTC)
	if len(got) != 2 || !got[0].CreatedOn.Equal(want) || !got[1].CreatedOn.Equal(want) {
		t.Fatalf("got %+v, want both entries at %v", got, want)
	}

	if _, err := mapSNActivitiesToDomain(context.Background(), []snActivity{{ID: "a3", CreatedOn: "not a date"}}); err == nil {
		t.Fatal("a malformed createdOn must still fail")
	}
}

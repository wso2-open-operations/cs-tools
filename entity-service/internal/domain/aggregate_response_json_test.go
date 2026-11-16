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

package domain

import (
	"encoding/json"
	"testing"
)

// TestAggregateResponse_JSONWithoutMinutesIsUnchanged: the minute fields are
// set only by the time card aggregate, so every other aggregate's JSON keeps
// exactly its old shape.
func TestAggregateResponse_JSONWithoutMinutesIsUnchanged(t *testing.T) {
	b, err := json.Marshal(AggregateResponse{
		Groups:       []AggregateBucket{{Key: "open", Label: "Open", Count: 3}},
		OthersCount:  1,
		TotalRecords: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"groups":[{"key":"open","label":"Open","count":3}],"othersCount":1,"totalRecords":4}`
	if string(b) != want {
		t.Errorf("JSON = %s, want %s", b, want)
	}
}

// TestAggregateResponse_JSONKeepsZeroMinutes: on the time card aggregate a
// bucket of cards with no time logged reads 0, not a missing field.
func TestAggregateResponse_JSONKeepsZeroMinutes(t *testing.T) {
	zero := 0
	b, err := json.Marshal(AggregateResponse{
		Groups:        []AggregateBucket{{Key: "2026-03-01", Label: "2026-03-01", TotalMinutes: &zero}},
		OthersMinutes: &zero,
		TotalMinutes:  &zero,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"groups":[{"key":"2026-03-01","label":"2026-03-01","count":0,"totalMinutes":0}],"othersCount":0,"totalRecords":0,"othersMinutes":0,"totalMinutes":0}`
	if string(b) != want {
		t.Errorf("JSON = %s, want %s", b, want)
	}
}

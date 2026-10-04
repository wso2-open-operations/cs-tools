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
	"encoding/json"
	"testing"
)

func TestSNFlexibleInt(t *testing.T) {
	cases := map[string]int{`3`: 3, `"3"`: 3, `" 7 "`: 7, `""`: 0, `null`: 0}
	for in, want := range cases {
		var got snFlexibleInt
		if err := json.Unmarshal([]byte(in), &got); err != nil || int(got) != want {
			t.Errorf("%s: got %d, %v; want %d, nil", in, got, err, want)
		}
	}
	for _, bad := range []string{`"three"`, `3.5`, `true`} {
		var got snFlexibleInt
		if err := json.Unmarshal([]byte(bad), &got); err == nil {
			t.Errorf("%s: want an error", bad)
		}
	}
}

// TestSNChoiceIDs_QuotedIDDoesNotFailThePage proves a page in which one row
// carries a quoted choice id decodes in full.
func TestSNChoiceIDs_QuotedIDDoesNotFailThePage(t *testing.T) {
	var rows []struct {
		State snCaseState `json:"state"`
	}
	if err := json.Unmarshal([]byte(`[{"state":{"id":1,"label":"Open"}},{"state":{"id":"3","label":"Closed"}}]`), &rows); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 || rows[0].State.ID != 1 || rows[1].State.ID != 3 {
		t.Fatalf("got %+v", rows)
	}
}

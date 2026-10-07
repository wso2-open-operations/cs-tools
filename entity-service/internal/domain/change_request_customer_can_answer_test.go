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
	"strings"
	"testing"
)

// customerCanAnswer is a tri-state on the wire: absent (not computed -- staff,
// the ServiceNow data source, a check that could not be made), false, true. A
// client that cannot tell absent from false would hide the customer's buttons
// whenever the server simply did not know.
func TestChangeRequest_CustomerCanAnswerJSONContract(t *testing.T) {
	yes, no := true, false
	for _, tc := range []struct {
		name string
		val  *bool
		want string // "" = the key must be absent
	}{
		{"not computed", nil, ""},
		{"false", &no, `"customerCanAnswer":false`},
		{"true", &yes, `"customerCanAnswer":true`},
	} {
		raw, err := json.Marshal(ChangeRequest{CustomerCanAnswer: tc.val})
		if err != nil {
			t.Fatalf("%s: marshal: %v", tc.name, err)
		}
		got := string(raw)
		if tc.want == "" {
			if strings.Contains(got, "customerCanAnswer") {
				t.Errorf("%s: the key must be omitted, got %s", tc.name, got)
			}
			continue
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: want %s in %s", tc.name, tc.want, got)
		}
	}

	var back ChangeRequest
	if err := json.Unmarshal([]byte(`{"customerCanAnswer":false}`), &back); err != nil || back.CustomerCanAnswer == nil || *back.CustomerCanAnswer {
		t.Errorf("a false on the wire must read back as a present false, got %v (%v)", back.CustomerCanAnswer, err)
	}
	if err := json.Unmarshal([]byte(`{}`), &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

// The answer is per viewer and belongs to the detail alone: the rows of a
// search never carry it.
func TestSearchChangeRequestView_HasNoCustomerCanAnswer(t *testing.T) {
	raw, err := json.Marshal(SearchChangeRequestView{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "customerCanAnswer") {
		t.Fatalf("a search row must not carry customerCanAnswer: %s", raw)
	}
}

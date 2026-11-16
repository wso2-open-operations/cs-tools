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
	"os"
	"testing"
	"time"
)

// TestResolveRelativeDate_Parity runs testdata/relative_date_parity.json, the
// cases the CSM backend's relativeDate and the webapp's resolver run too, so
// the three agree on every token.
func TestResolveRelativeDate_Parity(t *testing.T) {
	raw, err := os.ReadFile("testdata/relative_date_parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var parity struct {
		Cases []struct {
			Now   time.Time `json:"now"`
			Token string    `json:"token"`
			Date  *string   `json:"date"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &parity); err != nil {
		t.Fatal(err)
	}
	for _, c := range parity.Cases {
		resolved, matched, err := resolveRelativeDate(c.Token, c.Now.UTC())
		switch {
		case c.Date == nil && err == nil && matched:
			t.Errorf("%s at %s = %s, want it unresolved", c.Token, c.Now.Format(time.RFC3339), resolved)
		case c.Date != nil && (err != nil || !matched || resolved != *c.Date):
			t.Errorf("%s at %s = %q %v %v, want %s", c.Token, c.Now.Format(time.RFC3339), resolved, matched, err, *c.Date)
		}
	}
}

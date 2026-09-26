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

package handler

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const (
	bodyTestSysID  = "1c03cc8b3b538f1091404c6aa5e45af6"
	bodyTestDashed = "1c03cc8b-3b53-8f10-9140-4c6aa5e45af6"
)

func TestNormalizeBodyIDs_RewritesIDKeys(t *testing.T) {
	in := `{
		"projectId": "` + bodyTestSysID + `",
		"filters": {"deploymentIds": ["` + bodyTestSysID + `", "` + bodyTestDashed + `"]},
		"items": [{"caseId": "` + bodyTestSysID + `"}],
		"ProjectID": "` + bodyTestSysID + `"
	}`
	var got, want any
	if err := json.Unmarshal(normalizeBodyIDs([]byte(in)), &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	wantJSON := `{
		"projectId": "` + bodyTestDashed + `",
		"filters": {"deploymentIds": ["` + bodyTestDashed + `", "` + bodyTestDashed + `"]},
		"items": [{"caseId": "` + bodyTestDashed + `"}],
		"ProjectID": "` + bodyTestDashed + `"
	}`
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNormalizeBodyIDs_LeavesOtherValuesAlone(t *testing.T) {
	// applicationId comes from another service and must go back as issued;
	// free text that happens to be 32 hex must not be rewritten either.
	in := `{"applicationId":"` + bodyTestSysID + `","description":"` + bodyTestSysID + `","tags":["` + bodyTestSysID + `"]}`
	if got := string(normalizeBodyIDs([]byte(in))); got != in {
		t.Errorf("body without an id key must be returned byte-for-byte:\n got %s\nwant %s", got, in)
	}
}

func TestNormalizeBodyIDs_UnchangedBodyIsByteIdentical(t *testing.T) {
	in := ` { "projectId" : "` + bodyTestDashed + `",  "limit": 10 } `
	if got := string(normalizeBodyIDs([]byte(in))); got != in {
		t.Errorf("got %q, want the original bytes", got)
	}
}

func TestNormalizeBodyIDs_PreservesNumbersAndHTML(t *testing.T) {
	in := `{"projectId":"` + bodyTestSysID + `","big":12345678901234567890,"f":1.50,"note":"a<b&c"}`
	got := string(normalizeBodyIDs([]byte(in)))
	for _, want := range []string{`"big":12345678901234567890`, `"f":1.50`, `"note":"a<b&c"`, bodyTestDashed} {
		if !strings.Contains(got, want) {
			t.Errorf("result %s is missing %s", got, want)
		}
	}
}

func TestNormalizeBodyIDs_NonObjectBodies(t *testing.T) {
	for _, in := range []string{`[]`, `"` + bodyTestSysID + `"`, `null`, `42`, `{}`} {
		if got := string(normalizeBodyIDs([]byte(in))); got != in {
			t.Errorf("normalizeBodyIDs(%s) = %s, want it unchanged", in, got)
		}
	}
}

func TestDashIfSysID(t *testing.T) {
	if got := dashIfSysID(bodyTestSysID); got != bodyTestDashed {
		t.Errorf("got %q, want %q", got, bodyTestDashed)
	}
	for _, in := range []string{"", bodyTestDashed, "not-an-id", bodyTestSysID + "0"} {
		if got := dashIfSysID(in); got != in {
			t.Errorf("dashIfSysID(%q) = %q, want it unchanged", in, got)
		}
	}
}

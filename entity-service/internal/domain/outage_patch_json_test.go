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
	"bytes"
	"encoding/json"
	"testing"
)

// decodeStrict mirrors handler.decodeRequest: DisallowUnknownFields on the
// outer decoder.
func decodeStrict(t *testing.T, body string) (PatchOutageRequest, error) {
	t.Helper()
	var req PatchOutageRequest
	dec := json.NewDecoder(bytes.NewReader([]byte(body)))
	dec.DisallowUnknownFields()
	err := dec.Decode(&req)
	return req, err
}

func TestPatchOutageRequestEnd(t *testing.T) {
	// The portal's Reopen button, verbatim.
	req, err := decodeStrict(t, `{"end": null}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.End == nil || *req.End != nil {
		t.Fatalf(`{"end": null} must decode as reopen (non-nil End holding nil), got %v`, req.End)
	}

	req, err = decodeStrict(t, `{"end": "2026-10-03T14:37:20Z"}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.End == nil || *req.End == nil || **req.End != "2026-10-03T14:37:20Z" {
		t.Fatalf("a value must decode as close, got %v", req.End)
	}

	req, err = decodeStrict(t, `{"shortDescription": "x"}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.End != nil {
		t.Fatal("an omitted end must stay nil (leave end alone)")
	}
	if req.ShortDescription == nil || *req.ShortDescription != "x" {
		t.Fatal("other fields must still decode")
	}
}

func TestPatchOutageRequestEndCaseInsensitive(t *testing.T) {
	// encoding/json binds "End" to the End field, so the reopen check must too.
	req, err := decodeStrict(t, `{"End": null}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.End == nil || *req.End != nil {
		t.Fatalf(`{"End": null} must decode as reopen, got %v`, req.End)
	}

	// Two spellings: the struct decode would keep the last (a close) while a
	// key lookup saw the null (a reopen). Neither is safe to guess.
	for _, body := range []string{
		`{"end": null, "End": "2026-10-03T14:37:20Z"}`,
		`{"End": "2026-10-03T14:37:20Z", "end": null}`,
	} {
		if _, err := decodeStrict(t, body); err == nil {
			t.Errorf("%s: conflicting spellings of end must be rejected", body)
		}
	}
}

func TestPatchOutageRequestRejectsUnknownFields(t *testing.T) {
	if _, err := decodeStrict(t, `{"end": null, "bogus": 1}`); err == nil {
		t.Fatal("an unknown field must still be rejected")
	}
}

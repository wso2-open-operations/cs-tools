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

package dto

import (
	"encoding/json"
	"testing"
)

// TestReferenceItem_AbbreviationKeyAndOmission covers both halves of the
// contract: the key name the frontend reads, and that it disappears when absent
// rather than serialising as an empty string.
//
// The frontend prefers the abbreviation over the label where a compact name is
// wanted (recommendedLevelsProductKey in ManageProductModal.tsx) and falls back
// to the label when it is missing, so an empty-string abbreviation would be worse
// than no key at all — it would win the fallback and render nothing.
func TestReferenceItem_AbbreviationKeyAndOmission(t *testing.T) {
	abbr := "APIM"
	withAbbr, err := json.Marshal(ReferenceItem{ID: "1", Label: "API Manager", Abbreviation: &abbr})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// A fresh map per payload: unmarshalling into a non-nil map merges into it
	// rather than replacing it, so reusing one would carry the first payload's
	// abbreviation into the second assertion and hide a real regression.
	decode := func(t *testing.T, payload []byte) map[string]any {
		t.Helper()
		out := map[string]any{}
		if err := json.Unmarshal(payload, &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return out
	}

	if got := decode(t, withAbbr)["abbreviation"]; got != "APIM" {
		t.Errorf("abbreviation = %v, want APIM", got)
	}

	without, err := json.Marshal(ReferenceItem{ID: "2", Label: "Identity Server"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, present := decode(t, without)["abbreviation"]; present {
		t.Error("abbreviation present when nil; it must be omitted so the frontend falls back to the label")
	}
}

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

import "testing"

func TestStampBannerID(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	html := `<div data-banner="1" data-id="preview" data-title="Hello"></div>`
	got := StampBannerID(html, id)
	want := `<div data-banner="1" data-id="` + id + `" data-title="Hello"></div>`
	if got != want {
		t.Fatalf("stamp existing id: got %s", got)
	}
	bare := `<div data-banner="1" data-title="Hello"></div>`
	got = StampBannerID(bare, id)
	if got != want {
		t.Fatalf("stamp missing id: got %s", got)
	}
	if StampBannerID("not a banner", id) != "not a banner" {
		t.Fatal("html without a banner root should be left alone")
	}
}

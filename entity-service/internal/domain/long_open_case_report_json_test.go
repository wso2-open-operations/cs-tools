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

package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The long-open report row carries the case UUID as "id", separate from the
// display "internalId", so a consumer can link to the case page.
func TestLongOpenCaseReportRowJSONCarriesCaseUUID(t *testing.T) {
	const id = "00000000-0000-0000-0000-000000000001"
	raw, err := json.Marshal(domain.LongOpenCaseReportRow{ID: id, Number: "CS0000001", InternalID: "ABC-1"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["id"] != id {
		t.Errorf("id = %v, want %s", got["id"], id)
	}
	if got["internalId"] != "ABC-1" || got["id"] == got["internalId"] {
		t.Errorf("internalId = %v: must stay the display reference and differ from id", got["internalId"])
	}
}

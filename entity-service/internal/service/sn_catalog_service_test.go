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
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNCatalogService_GetCatalogItemVariables_MapsChoices verifies that the
// selectable options on a choice-based variable reach the response in the backing
// data source's own order, and that a free-text variable carries no choices at all
// rather than an empty list.
func TestSNCatalogService_GetCatalogItemVariables_MapsChoices(t *testing.T) {
	tests := []struct {
		name        string
		choicesJSON string
		want        []domain.CatalogVariableChoice
	}{
		{
			name: "choice variable keeps order",
			choicesJSON: `, "choices": [
				{"value": "aws", "text": "AWS", "order": 100},
				{"value": "azure", "text": "Azure", "order": 200},
				{"value": "gcp", "text": "GCP", "order": 300}
			]`,
			want: []domain.CatalogVariableChoice{
				{Value: strPtr("aws"), Text: strPtr("AWS"), Order: intPtr(100)},
				{Value: strPtr("azure"), Text: strPtr("Azure"), Order: intPtr(200)},
				{Value: strPtr("gcp"), Text: strPtr("GCP"), Order: intPtr(300)},
			},
		},
		{
			name:        "free-text variable has no choices",
			choicesJSON: ``,
			want:        nil,
		},
		{
			name:        "empty choices list",
			choicesJSON: `, "choices": []`,
			want:        nil,
		},
		{
			name:        "null choices",
			choicesJSON: `, "choices": null`,
			want:        nil,
		},
		{
			name:        "null option fields stay null",
			choicesJSON: `, "choices": [{"value": null, "text": null, "order": null}]`,
			want:        []domain.CatalogVariableChoice{{}},
		},
	}

	catalogUUID := sysidToUUID(sysid32('1'))
	catalogItemUUID := sysidToUUID(sysid32('2'))
	variableSysid := sysid32('3')

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"variables": [
				{"id": "` + variableSysid + `", "questionText": "Target cloud", "order": 100, "type": "select"` + tt.choicesJSON + `}
			]}`

			client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", r.Method)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			})

			svc := NewServiceNowCatalogService(client)

			resp, err := svc.GetCatalogItemVariables(contextWithUserIDToken("token"), catalogUUID, catalogItemUUID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(resp.Variables) != 1 {
				t.Fatalf("expected 1 variable, got %d", len(resp.Variables))
			}

			got := resp.Variables[0]
			if got.ID != sysidToUUID(variableSysid) || got.QuestionText != "Target cloud" ||
				got.Order != 100 || got.Type != "select" {
				t.Fatalf("unexpected variable mapping: %+v", got)
			}
			if len(got.Choices) != len(tt.want) {
				t.Fatalf("Choices = %+v, want %+v", got.Choices, tt.want)
			}
			for i, want := range tt.want {
				assertOptStr(t, "Choices["+itoa(i)+"].Value", got.Choices[i].Value, want.Value)
				assertOptStr(t, "Choices["+itoa(i)+"].Text", got.Choices[i].Text, want.Text)
				assertOptInt(t, "Choices["+itoa(i)+"].Order", got.Choices[i].Order, want.Order)
			}
		})
	}
}

func assertOptStr(t *testing.T, field string, got, want *string) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Fatalf("%s: got %v, want %v", field, deref(got), deref(want))
	case *got != *want:
		t.Fatalf("%s: got %q, want %q", field, *got, *want)
	}
}

func assertOptInt(t *testing.T, field string, got, want *int) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Fatalf("%s: got %v, want %v", field, got, want)
	case *got != *want:
		t.Fatalf("%s: got %d, want %d", field, *got, *want)
	}
}

func intPtr(n int) *int { return &n }

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

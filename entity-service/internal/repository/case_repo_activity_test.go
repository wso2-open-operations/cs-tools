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

package repository

import (
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestCaseActivityFieldChangeLabel(t *testing.T) {
	tests := []struct {
		fieldName string
		want      string
	}{
		{"severity", "Severity"},
		{"assigned_to_id", "Assigned To Id"},
		{"state", "State"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := caseActivityFieldChangeLabel(tt.fieldName); got != tt.want {
			t.Errorf("caseActivityFieldChangeLabel(%q) = %q, want %q", tt.fieldName, got, tt.want)
		}
	}
}

// fakeCaseActivityRow feeds scanCaseActivity fixed values without a real
// database connection, exercising the exact Scan destination order
// scanCaseActivity itself uses (id, kind, content, created_on, email,
// first_name, last_name, name, comment_type, file_name, content_type,
// size_bytes, field_name, old_value, new_value).
type fakeCaseActivityRow struct {
	id, kind, content                string
	createdOn                        time.Time
	email, firstName, lastName, name *string
	commentType                      *string
	fileName, contentType            *string
	sizeBytes                        *int64
	fieldName, oldValue, newValue    *string
}

func (f fakeCaseActivityRow) Scan(dest ...any) error {
	*dest[0].(*string) = f.id
	*dest[1].(*string) = f.kind
	*dest[2].(*string) = f.content
	*dest[3].(*time.Time) = f.createdOn
	*dest[4].(**string) = f.email
	*dest[5].(**string) = f.firstName
	*dest[6].(**string) = f.lastName
	*dest[7].(**string) = f.name
	*dest[8].(**string) = f.commentType
	*dest[9].(**string) = f.fileName
	*dest[10].(**string) = f.contentType
	*dest[11].(**int64) = f.sizeBytes
	*dest[12].(**string) = f.fieldName
	*dest[13].(**string) = f.oldValue
	*dest[14].(**string) = f.newValue
	return nil
}

func TestScanCaseActivity(t *testing.T) {
	now := time.Now()

	t.Run("comment", func(t *testing.T) {
		row := fakeCaseActivityRow{
			id: "a1", kind: "comment", content: "hello",
			createdOn: now, email: strPtr("jane@example.com"), name: strPtr("Jane Doe"),
			commentType: strPtr("COMMENT"),
		}
		got, err := scanCaseActivity(row)
		if err != nil {
			t.Fatalf("scanCaseActivity() error = %v", err)
		}
		if got.Type != domain.ActivityTypeComment {
			t.Errorf("Type = %v, want %v", got.Type, domain.ActivityTypeComment)
		}
		if got.CommentType == nil || *got.CommentType != domain.CommentTypeComment {
			t.Errorf("CommentType = %v, want %v", got.CommentType, domain.CommentTypeComment)
		}
		if len(got.Changes) != 0 {
			t.Errorf("Changes = %v, want empty", got.Changes)
		}
	})

	t.Run("attachment", func(t *testing.T) {
		row := fakeCaseActivityRow{
			id: "a2", kind: "attachment", content: "",
			createdOn: now, email: strPtr("jane@example.com"), name: strPtr("Jane Doe"),
			fileName: strPtr("report.pdf"), contentType: strPtr("application/pdf"), sizeBytes: func() *int64 { v := int64(1024); return &v }(),
		}
		got, err := scanCaseActivity(row)
		if err != nil {
			t.Fatalf("scanCaseActivity() error = %v", err)
		}
		if got.Type != domain.ActivityTypeAttachment {
			t.Errorf("Type = %v, want %v", got.Type, domain.ActivityTypeAttachment)
		}
		if got.FileName != "report.pdf" || got.ContentType != "application/pdf" || got.SizeBytes != 1024 {
			t.Errorf("attachment fields = %+v", got)
		}
	})

	t.Run("field_change", func(t *testing.T) {
		row := fakeCaseActivityRow{
			id: "a3", kind: "field_change", content: "",
			createdOn: now, email: strPtr("jane@example.com"), name: strPtr("Jane Doe"),
			fieldName: strPtr("severity"), oldValue: strPtr("high"), newValue: strPtr("critical"),
		}
		got, err := scanCaseActivity(row)
		if err != nil {
			t.Fatalf("scanCaseActivity() error = %v", err)
		}
		if got.Type != domain.ActivityTypeFieldChange {
			t.Errorf("Type = %v, want %v", got.Type, domain.ActivityTypeFieldChange)
		}
		if len(got.Changes) != 1 {
			t.Fatalf("Changes = %v, want exactly 1 entry", got.Changes)
		}
		want := domain.FieldChange{Field: "severity", FieldLabel: "Severity", PreviousValue: "high", NewValue: "critical"}
		if got.Changes[0] != want {
			t.Errorf("Changes[0] = %+v, want %+v", got.Changes[0], want)
		}
	})

	t.Run("field_change with no matching field_name renders empty label", func(t *testing.T) {
		row := fakeCaseActivityRow{
			id: "a4", kind: "field_change", content: "",
			createdOn: now,
		}
		got, err := scanCaseActivity(row)
		if err != nil {
			t.Fatalf("scanCaseActivity() error = %v", err)
		}
		if len(got.Changes) != 1 || got.Changes[0].Field != "" || got.Changes[0].FieldLabel != "" {
			t.Errorf("Changes = %+v, want a single empty-field entry", got.Changes)
		}
	})
}

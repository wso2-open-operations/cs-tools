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
	"context"
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// assertErrType asserts that err matches the concrete pointer type of want.
func assertErrType(t *testing.T, want, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	switch want.(type) {
	case *apierror.UnauthorizedError:
		if _, ok := err.(*apierror.UnauthorizedError); !ok {
			t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
		}
	case *apierror.ValidationError:
		if _, ok := err.(*apierror.ValidationError); !ok {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	}
}

func TestSNCallRequestService_UpdateCallRequest_Validation(t *testing.T) {
	validID := "11111111-1111-1111-1111-111111111111"
	negDuration := -5
	badDate := "not-a-date"
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name    string
		ctx     context.Context
		req     domain.UpdateCallRequestRequest
		wantErr error
	}{
		{
			name:    "missing token",
			ctx:     context.Background(),
			req:     domain.UpdateCallRequestRequest{ID: validID, State: domain.CallRequestStateScheduled},
			wantErr: &apierror.UnauthorizedError{},
		},
		{
			name:    "invalid id format",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.UpdateCallRequestRequest{ID: "not-a-uuid", State: domain.CallRequestStateScheduled},
			wantErr: &apierror.ValidationError{},
		},
		{
			name:    "invalid state",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.UpdateCallRequestRequest{ID: validID, State: domain.CallRequestStateType("bogus")},
			wantErr: &apierror.ValidationError{},
		},
		{
			name:    "scheduled with unparseable meetingDate",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.UpdateCallRequestRequest{ID: validID, State: domain.CallRequestStateScheduled, MeetingDate: &badDate},
			wantErr: &apierror.ValidationError{},
		},
		{
			name:    "concluded with non-positive actualDurationMin",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.UpdateCallRequestRequest{ID: validID, State: domain.CallRequestStateConcluded, Notes: strPtr("n"), ActualDurationMin: &negDuration},
			wantErr: &apierror.ValidationError{},
		},
	}

	// client is intentionally nil: every case must fail validation before touching it.
	svc := NewServiceNowCallRequestService(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateCallRequest(tt.ctx, tt.req)
			assertErrType(t, tt.wantErr, err)
		})
	}
}

// TestToDomainCallRequestState verifies the READ normalization keeps the numeric
// SN choice-list key out of the domain view: an integer id is mapped to the string
// enum, a string id passes through, and the label is preserved.
func TestToDomainCallRequestState(t *testing.T) {
	tests := []struct {
		name   string
		in     snCallRequestState
		wantID string
	}{
		{
			name:   "integer key maps to string enum",
			in:     snCallRequestState{ID: json.RawMessage(`3`), Label: "Scheduled"},
			wantID: string(domain.CallRequestStateScheduled),
		},
		{
			name:   "string id passes through",
			in:     snCallRequestState{ID: json.RawMessage(`"concluded"`), Label: "Concluded"},
			wantID: string(domain.CallRequestStateConcluded),
		},
		{
			name:   "unknown integer yields empty id",
			in:     snCallRequestState{ID: json.RawMessage(`99`), Label: "Mystery"},
			wantID: "",
		},
		{
			name:   "empty id yields empty id",
			in:     snCallRequestState{},
			wantID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toDomainCallRequestState(tt.in)
			if got.ID != tt.wantID {
				t.Fatalf("id: got %q, want %q", got.ID, tt.wantID)
			}
			if got.Label != tt.in.Label {
				t.Fatalf("label: got %q, want %q", got.Label, tt.in.Label)
			}
		})
	}
}

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

func TestSNCallRequestService_ScheduleCallRequest_Validation(t *testing.T) {
	validID := "11111111-1111-1111-1111-111111111111"
	validDate := "2026-08-01T10:00:00Z"

	tests := []struct {
		name    string
		ctx     context.Context
		req     domain.ScheduleCallRequestRequest
		wantErr error
	}{
		{
			name:    "missing token",
			ctx:     context.Background(),
			req:     domain.ScheduleCallRequestRequest{ID: validID, MeetingDate: validDate, DurationMinutes: 30},
			wantErr: &apierror.UnauthorizedError{},
		},
		{
			name:    "invalid id format",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.ScheduleCallRequestRequest{ID: "not-a-uuid", MeetingDate: validDate, DurationMinutes: 30},
			wantErr: &apierror.ValidationError{},
		},
		{
			name:    "missing meetingDate",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.ScheduleCallRequestRequest{ID: validID, MeetingDate: "", DurationMinutes: 30},
			wantErr: &apierror.ValidationError{},
		},
		{
			name:    "unparseable meetingDate",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.ScheduleCallRequestRequest{ID: validID, MeetingDate: "not-a-date", DurationMinutes: 30},
			wantErr: &apierror.ValidationError{},
		},
		{
			name:    "duration below minimum",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.ScheduleCallRequestRequest{ID: validID, MeetingDate: validDate, DurationMinutes: 10},
			wantErr: &apierror.ValidationError{},
		},
		{
			name:    "duration above maximum",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.ScheduleCallRequestRequest{ID: validID, MeetingDate: validDate, DurationMinutes: 300},
			wantErr: &apierror.ValidationError{},
		},
	}

	// client is intentionally nil: every case must fail validation before touching it.
	svc := NewServiceNowCallRequestService(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ScheduleCallRequest(tt.ctx, tt.req)
			assertErrType(t, tt.wantErr, err)
		})
	}
}

func TestSNCallRequestService_RejectCallRequest_Validation(t *testing.T) {
	validID := "11111111-1111-1111-1111-111111111111"

	tests := []struct {
		name    string
		ctx     context.Context
		req     domain.RejectCallRequestRequest
		wantErr error
	}{
		{
			name:    "missing token",
			ctx:     context.Background(),
			req:     domain.RejectCallRequestRequest{ID: validID},
			wantErr: &apierror.UnauthorizedError{},
		},
		{
			name:    "invalid id format",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.RejectCallRequestRequest{ID: "not-a-uuid"},
			wantErr: &apierror.ValidationError{},
		},
	}

	svc := NewServiceNowCallRequestService(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.RejectCallRequest(tt.ctx, tt.req)
			assertErrType(t, tt.wantErr, err)
		})
	}
}

func TestSNCallRequestService_SendCallRequestNotes_Validation(t *testing.T) {
	validID := "11111111-1111-1111-1111-111111111111"
	negDuration := -5

	tests := []struct {
		name    string
		ctx     context.Context
		req     domain.SendCallRequestNotesRequest
		wantErr error
	}{
		{
			name:    "missing token",
			ctx:     context.Background(),
			req:     domain.SendCallRequestNotesRequest{ID: validID, Notes: "n"},
			wantErr: &apierror.UnauthorizedError{},
		},
		{
			name:    "invalid id format",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.SendCallRequestNotesRequest{ID: "not-a-uuid", Notes: "n"},
			wantErr: &apierror.ValidationError{},
		},
		{
			name:    "missing notes",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.SendCallRequestNotesRequest{ID: validID, Notes: ""},
			wantErr: &apierror.ValidationError{},
		},
		{
			name:    "non-positive actualDurationMin",
			ctx:     contextWithUserIDToken("token"),
			req:     domain.SendCallRequestNotesRequest{ID: validID, Notes: "n", ActualDurationMin: &negDuration},
			wantErr: &apierror.ValidationError{},
		},
	}

	svc := NewServiceNowCallRequestService(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.SendCallRequestNotes(tt.ctx, tt.req)
			assertErrType(t, tt.wantErr, err)
		})
	}
}

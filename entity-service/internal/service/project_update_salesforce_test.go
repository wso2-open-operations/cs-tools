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
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// recordingProjectStatusWriter stands in for the Sales Entity client.
type recordingProjectStatusWriter struct {
	mu    sync.Mutex
	err   error
	calls []string // "<sfId> <status>"
}

func (w *recordingProjectStatusWriter) UpdateProjectStatus(_ context.Context, sfID, status string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, sfID+" "+status)
	return w.err
}

func (w *recordingProjectStatusWriter) snapshot() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.calls...)
}

// projectUpdateWithSalesforce builds the Postgres project update service with
// the Salesforce writeback attached, over a repository whose project currently
// has the given overall closure state and Salesforce id.
func projectUpdateWithSalesforce(closureState, sfID *string, writer *recordingProjectStatusWriter, failures *recordingSNWritebackFailures) ProjectUpdateService {
	repo := &stubProjectUpdateRepo{sfID: sfID, closureState: closureState}
	svc := NewProjectUpdateService(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{})
	return WithSalesforceProjectStateWriteback(svc, NewSNWritebackDispatcher(failures), writer)
}

// TestProjectUpdate_WritesOverallClosureStateToSalesforce: a change to a
// closure field sends the project's overall state to Salesforce State__c.
func TestProjectUpdate_WritesOverallClosureStateToSalesforce(t *testing.T) {
	sfID := "a0dE200000RgxAbIAJ"
	for _, tc := range []struct {
		name         string
		req          domain.ProjectUpdateRequest
		closureState string
		want         string
	}{
		{"end-date suspension", domain.ProjectUpdateRequest{EndDateClosureState: sPtr("Closed")}, "Suspended", sfID + " Suspended"},
		{"invoice suspension", domain.ProjectUpdateRequest{InvoiceDueDateClosureState: sPtr("Suspended")}, "Suspended", sfID + " Suspended"},
		{"reopen", domain.ProjectUpdateRequest{EndDateClosureState: sPtr("Open"), InvoiceDueDateClosureState: sPtr("Open")}, "Open", sfID + " Open"},
		{"compliance", domain.ProjectUpdateRequest{ComplianceViolationClosureState: sPtr("Suspended")}, "Suspended", sfID + " Suspended"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := &recordingProjectStatusWriter{}
			svc := projectUpdateWithSalesforce(sPtr(tc.closureState), &sfID, writer, &recordingSNWritebackFailures{})
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

			if _, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", tc.req); err != nil {
				t.Fatalf("UpdateProject: %v", err)
			}
			waitFor(t, func() bool { return len(writer.snapshot()) == 1 })
			if got := writer.snapshot()[0]; got != tc.want {
				t.Errorf("Salesforce write = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestProjectUpdate_SkipsSalesforceWhenThereIsNothingToSend: no closure field
// in the request, a Restricted (or unknown) overall state, or no Salesforce id.
func TestProjectUpdate_SkipsSalesforceWhenThereIsNothingToSend(t *testing.T) {
	sfID := "a0dE200000RgxAbIAJ"
	blank := "  "
	for _, tc := range []struct {
		name         string
		req          domain.ProjectUpdateRequest
		closureState *string
		sfID         *string
	}{
		{"no closure field", domain.ProjectUpdateRequest{SuspensionProcessState: []byte(`{}`)}, sPtr("Suspended"), &sfID},
		{"restricted", domain.ProjectUpdateRequest{EndDateClosureState: sPtr("Restricted")}, sPtr("Restricted"), &sfID},
		{"no overall state", domain.ProjectUpdateRequest{EndDateClosureState: sPtr("Closed")}, nil, &sfID},
		{"no salesforce id", domain.ProjectUpdateRequest{EndDateClosureState: sPtr("Closed")}, sPtr("Suspended"), nil},
		{"blank salesforce id", domain.ProjectUpdateRequest{EndDateClosureState: sPtr("Closed")}, sPtr("Suspended"), &blank},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := &recordingProjectStatusWriter{}
			failures := &recordingSNWritebackFailures{}
			svc := projectUpdateWithSalesforce(tc.closureState, tc.sfID, writer, failures)
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

			if _, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", tc.req); err != nil {
				t.Fatalf("UpdateProject: %v", err)
			}
			time.Sleep(50 * time.Millisecond)
			if got := writer.snapshot(); len(got) != 0 {
				t.Errorf("Salesforce writes = %v, want none", got)
			}
			if failures.count() != 0 {
				t.Errorf("failures recorded = %d, want 0", failures.count())
			}
		})
	}
}

// TestProjectUpdate_SalesforceFailureIsRecordedNotReturned: Postgres has
// committed, so a failed Salesforce write never fails the request; it is
// recorded for an operator to replay.
func TestProjectUpdate_SalesforceFailureIsRecordedNotReturned(t *testing.T) {
	sfID := "a0dE200000RgxAbIAJ"
	writer := &recordingProjectStatusWriter{err: errors.New("sales entity unreachable")}
	failures := &recordingSNWritebackFailures{}
	svc := projectUpdateWithSalesforce(sPtr("Suspended"), &sfID, writer, failures)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	id := "11111111-1111-1111-1111-111111111111"
	if _, err := svc.UpdateProject(ctx, id, domain.ProjectUpdateRequest{EndDateClosureState: sPtr("Closed")}); err != nil {
		t.Fatalf("UpdateProject() error = %v, want nil: the Salesforce write is best effort", err)
	}
	waitFor(t, func() bool { return failures.count() == 1 })
	failures.mu.Lock()
	got := failures.calls[0]
	failures.mu.Unlock()
	if got.EntityType != salesforceProjectWritebackEntity || got.EntityID != id || got.Operation != "update_state" {
		t.Errorf("failure = %s/%s/%s, want %s/%s/update_state", got.EntityType, got.EntityID, got.Operation, salesforceProjectWritebackEntity, id)
	}
}

// TestProjectUpdate_WithoutWritebackNeverCallsSalesforce: the switch off (the
// default) leaves PATCH /projects/{id} exactly as before.
func TestProjectUpdate_WithoutWritebackNeverCallsSalesforce(t *testing.T) {
	writer := &recordingProjectStatusWriter{}
	svc := WithSalesforceProjectStateWriteback(
		NewProjectUpdateService(&stubProjectUpdateRepo{}, testProjectUserRepo(), alwaysUnrestrictedAccess{}), nil, writer)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	if _, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", domain.ProjectUpdateRequest{EndDateClosureState: sPtr("Closed")}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if got := writer.snapshot(); len(got) != 0 {
		t.Errorf("Salesforce writes = %v, want none without a dispatcher", got)
	}
}

// TestProjectUpdate_SendsTheStateCurrentWhenTheJobRuns: the job sends the
// project's state at the time it runs, not the one its update committed. A
// suspend whose job runs after a later reopen has committed therefore sends
// Open, so Salesforce ends on CSM's latest state whatever the dispatch order.
func TestProjectUpdate_SendsTheStateCurrentWhenTheJobRuns(t *testing.T) {
	sfID := "a0dE200000RgxAbIAJ"
	writer := &recordingProjectStatusWriter{}
	repo := &stubProjectUpdateRepo{
		// This update committed Suspended...
		updateProject: func(_ context.Context, id string, _ domain.ProjectUpdateRequest, by string) (domain.ProjectUpdateResult, error) {
			return domain.ProjectUpdateResult{ID: id, UpdatedBy: by, UpdatedOn: time.Now(), ClosureState: sPtr("Suspended")}, nil
		},
		// ...but a reopen has committed since, so the project is Open now.
		sfID: &sfID, closureState: sPtr("Open"),
	}
	svc := WithSalesforceProjectStateWriteback(
		NewProjectUpdateService(repo, testProjectUserRepo(), alwaysUnrestrictedAccess{}),
		NewSNWritebackDispatcher(&recordingSNWritebackFailures{}), writer)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	if _, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", domain.ProjectUpdateRequest{EndDateClosureState: sPtr("Closed")}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	waitFor(t, func() bool { return len(writer.snapshot()) == 1 })
	if got, want := writer.snapshot()[0], sfID+" Open"; got != want {
		t.Errorf("Salesforce write = %q, want %q (the current state, not the committed one)", got, want)
	}
}

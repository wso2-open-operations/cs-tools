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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

const (
	testLinkSfID        = "a3UE2000008btovMAA"
	testLinkProjectSfID = "a0dE200000RgyJZIAZ"
	testLinkOppRowID    = "55555555-6666-7777-8888-999999999999"
)

type fakeLinkSalesEntity struct {
	link   salesentity.LinkedOpportunity
	err    error
	calls  int
	lastID string
}

func (f *fakeLinkSalesEntity) GetLinkedOpportunity(_ context.Context, id string) (salesentity.LinkedOpportunity, error) {
	f.calls++
	f.lastID = id
	return f.link, f.err
}

type fakeLinkRepo struct {
	opportunities map[string]string
	upserts       []domain.SalesforceOpportunityLinkUpsert
	upsertState   []domain.UpsertSalesforceIngestStateRequest
	upsertErr     error
	deletes       []string
	deleteState   []domain.UpsertSalesforceIngestStateRequest
	deleteRows    int64
}

func (f *fakeLinkRepo) UpsertFromSalesforce(_ context.Context, row domain.SalesforceOpportunityLinkUpsert, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	f.upserts = append(f.upserts, row)
	f.upsertState = append(f.upsertState, state)
	return true, f.upsertErr
}

func (f *fakeLinkRepo) DeleteByLinkSfID(_ context.Context, sfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	f.deletes = append(f.deletes, sfID)
	f.deleteState = append(f.deleteState, state)
	return f.deleteRows, nil
}

func (f *fakeLinkRepo) LookupOpportunityIDBySfID(_ context.Context, sfID string) (*string, error) {
	if id, ok := f.opportunities[sfID]; ok {
		return &id, nil
	}
	return nil, nil
}

// linkOppRepo is the opportunity repository of a link test: an upsert makes
// the opportunity visible to the link repository's lookup, as the database
// would.
type linkOppRepo struct {
	fakeOpportunityRepo
	links *fakeLinkRepo
}

func (r *linkOppRepo) UpsertFromSalesforce(ctx context.Context, row domain.SalesforceOpportunityUpsert, state domain.UpsertSalesforceIngestStateRequest) (domain.SalesforceOpportunityUpsertResult, error) {
	res, err := r.fakeOpportunityRepo.UpsertFromSalesforce(ctx, row, state)
	if err == nil {
		r.links.opportunities[row.SfID] = testLinkOppRowID
	}
	return res, err
}

func sampleLink() salesentity.LinkedOpportunity {
	return salesentity.LinkedOpportunity{
		ID: testLinkSfID, Name: sampleStr("LO-26-09-N-00034824"),
		ProjectID: sampleStr(testLinkProjectSfID), OpportunityID: sampleStr(testOpportunitySfID),
		LastModifiedDate: sampleStr("2026-09-29T09:28:21.000+0000"),
	}
}

type linkHarness struct {
	linkSE   *fakeLinkSalesEntity
	links    *fakeLinkRepo
	opps     *linkOppRepo
	oppSE    *fakeOpportunitySalesEntity
	projects *fakeProjectRepo
	projSE   *fakeProjectSalesEntity
	states   *fakeIngestStateRepo
	svc      *salesforceEventService
}

// newLinkHarness builds a service with the Opportunity family on (links
// included), the account in CSM, and — when projectIngest — the Project
// ingest on with the given insert switch.
func newLinkHarness(opportunities, projects map[string]string, projectIngest, insert bool) *linkHarness {
	if opportunities == nil {
		opportunities = map[string]string{}
	}
	h := &linkHarness{
		linkSE:   &fakeLinkSalesEntity{link: sampleLink()},
		links:    &fakeLinkRepo{opportunities: opportunities, deleteRows: 1},
		oppSE:    &fakeOpportunitySalesEntity{opp: sampleOpportunity()},
		projects: newProjectRepo(projects),
		states:   &fakeIngestStateRepo{},
	}
	h.opps = &linkOppRepo{links: h.links}
	p := sampleProject()
	p.ID, p.Key = testLinkProjectSfID, sampleStr("CSMSYNCTESTEVAL")
	h.projSE = &fakeProjectSalesEntity{project: p}
	lookup := &stubSalesforceAccountRepo{accountsBySfID: map[string]string{testOppAccountSfID: testOppAccountRowID, testProjectAccountSf: testProjectAccountID}}
	svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{Accounts: lookup, States: h.states, Projects: h.projects})
	svc = WithOpportunityIngest(svc, OpportunityIngest{Opportunities: h.opps, SalesEntity: h.oppSE})
	svc = WithLinkedOpportunityIngest(svc, LinkedOpportunityIngest{Links: h.links, SalesEntity: h.linkSE})
	if projectIngest {
		svc = WithProjectIngest(svc, ProjectIngest{Projects: h.projects, SalesEntity: h.projSE, InsertEnabled: insert})
	}
	h.svc = svc.(*salesforceEventService)
	return h
}

func linkEvent(eventType string) domain.SalesforceEventRequest {
	return domain.SalesforceEventRequest{Entity: "Linked_Opportunity__c", EventType: eventType, ReferenceID: testLinkSfID}
}

func TestLinkedOpportunityIngest_DisabledIgnoresEvents(t *testing.T) {
	se := &fakeLinkSalesEntity{link: sampleLink()}
	// The link branch alone, without the Opportunity ingest, is off too.
	svc := WithLinkedOpportunityIngest(
		NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{States: &fakeIngestStateRepo{}}),
		LinkedOpportunityIngest{Links: &fakeLinkRepo{}, SalesEntity: se})
	for _, et := range []string{"CREATED", "UPDATED", "DELETED", "RESTORED"} {
		if err := svc.HandleEvent(context.Background(), linkEvent(et)); err != nil {
			t.Errorf("%s: err = %v, want nil", et, err)
		}
	}
	if se.calls != 0 {
		t.Errorf("fetches = %d, want 0", se.calls)
	}
	if err := svc.(*salesforceEventService).RetryLinkedOpportunityIngest(context.Background(), testLinkSfID); !errors.Is(err, errLinkedOpportunityIngestDisabled) {
		t.Errorf("retry err = %v", err)
	}
}

// TestLinkedOpportunityIngest_UpsertsWithBothParents: both parents present;
// CREATED/UPDATED/RESTORED (and the suffix-less entity name) write one link.
func TestLinkedOpportunityIngest_UpsertsWithBothParents(t *testing.T) {
	for _, entity := range []string{"Linked_Opportunity__c", "Linked_Opportunity"} {
		for _, et := range []string{"CREATED", "UPDATED", "RESTORED"} {
			h := newLinkHarness(map[string]string{testOpportunitySfID: testLinkOppRowID}, map[string]string{testLinkProjectSfID: testProjectRowID}, false, false)
			req := linkEvent(et)
			req.Entity = entity
			if err := h.svc.HandleEvent(context.Background(), req); err != nil {
				t.Fatalf("%s %s: %v", entity, et, err)
			}
			if h.oppSE.calls != 0 || len(h.links.upserts) != 1 {
				t.Fatalf("%s %s: opportunity fetches %d, link upserts %d", entity, et, h.oppSE.calls, len(h.links.upserts))
			}
			row, st := h.links.upserts[0], h.links.upsertState[0]
			if row.LinkSfID != testLinkSfID || derefString(row.Number) != "LO-26-09-N-00034824" || row.OpportunityID != testLinkOppRowID || row.ProjectID != testProjectRowID {
				t.Errorf("%s %s: row = %+v", entity, et, row)
			}
			if st.Entity != domain.SalesforceIngestEntityLinkedOpportunity || st.EventType != et || st.Status != domain.SalesforceIngestSucceeded {
				t.Errorf("%s %s: state = %+v", entity, et, st)
			}
		}
	}
}

// TestLinkedOpportunityIngest_IngestsMissingOpportunity: a link whose
// opportunity is not in CSM runs the Opportunity ingest first.
func TestLinkedOpportunityIngest_IngestsMissingOpportunity(t *testing.T) {
	h := newLinkHarness(nil, map[string]string{testLinkProjectSfID: testProjectRowID}, false, false)
	if err := h.svc.HandleEvent(context.Background(), linkEvent("CREATED")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if h.oppSE.lastID != testOpportunitySfID || len(h.opps.upserts) != 1 || len(h.links.upserts) != 1 || h.links.upserts[0].OpportunityID != testLinkOppRowID {
		t.Errorf("opportunity fetch %q upserts %d, links %+v", h.oppSE.lastID, len(h.opps.upserts), h.links.upserts)
	}
}

// The link resolves its opportunity through the shared ensureOpportunity, so
// a stale Opportunity ledger row (version V SUCCEEDED, row gone) does not
// stop the inline re-ingest.
func TestLinkedOpportunityIngest_StaleOpportunityLedgerReingestsMissingRow(t *testing.T) {
	h := newLinkHarness(nil, map[string]string{testLinkProjectSfID: testProjectRowID}, false, false)
	h.oppSE.opp.LastModifiedDate = sampleStr("2026-09-18T06:37:07.000+0000")
	h.states.apply(domain.UpsertSalesforceIngestStateRequest{Entity: domain.SalesforceIngestEntityOpportunity, SfID: testOpportunitySfID,
		EventModifiedOn: time.Date(2026, 9, 18, 6, 37, 7, 0, time.UTC), EventType: "UPDATED", Status: domain.SalesforceIngestSucceeded})
	if err := h.svc.HandleEvent(context.Background(), linkEvent("UPDATED")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(h.opps.upserts) != 1 || len(h.links.upserts) != 1 || h.links.upserts[0].OpportunityID != testLinkOppRowID {
		t.Errorf("opportunity upserts %d, links %+v", len(h.opps.upserts), h.links.upserts)
	}
}

// TestLinkedOpportunityIngest_MissingProject: without project inserts a
// missing project fails the link with a retryable missing-parent error; with
// them, EnsureProject ingests the project first.
func TestLinkedOpportunityIngest_MissingProject(t *testing.T) {
	t.Run("project ingest off", func(t *testing.T) {
		h := newLinkHarness(map[string]string{testOpportunitySfID: testLinkOppRowID}, nil, false, false)
		var nf *apierror.NotFoundError
		if err := h.svc.HandleEvent(context.Background(), linkEvent("UPDATED")); !errors.As(err, &nf) {
			t.Fatalf("err = %v, want NotFoundError", err)
		}
		if len(h.links.upserts) != 0 || len(h.states.upserts) != 1 || h.states.upserts[0].Entity != domain.SalesforceIngestEntityLinkedOpportunity ||
			!repository.IsMissingParentError(derefString(h.states.upserts[0].LastError)) {
			t.Errorf("links %d ledger %+v", len(h.links.upserts), h.states.upserts)
		}
	})
	t.Run("inserts off", func(t *testing.T) {
		h := newLinkHarness(map[string]string{testOpportunitySfID: testLinkOppRowID}, nil, true, false)
		var nf *apierror.NotFoundError
		if err := h.svc.HandleEvent(context.Background(), linkEvent("UPDATED")); !errors.As(err, &nf) || h.projSE.calls != 0 {
			t.Fatalf("err = %v project fetches %d", err, h.projSE.calls)
		}
	})
	t.Run("inserts on", func(t *testing.T) {
		h := newLinkHarness(map[string]string{testOpportunitySfID: testLinkOppRowID}, nil, true, true)
		if err := h.svc.HandleEvent(context.Background(), linkEvent("UPDATED")); err != nil {
			t.Fatalf("HandleEvent: %v", err)
		}
		if h.projSE.lastID != testLinkProjectSfID || len(h.links.upserts) != 1 || h.links.upserts[0].ProjectID != testProjectRowID {
			t.Errorf("project fetch %q links %+v", h.projSE.lastID, h.links.upserts)
		}
	})
}

func TestLinkedOpportunityIngest_MissingParentIDAcknowledged(t *testing.T) {
	h := newLinkHarness(nil, nil, false, false)
	h.linkSE.link.ProjectID = nil
	if err := h.svc.HandleEvent(context.Background(), linkEvent("UPDATED")); err != nil {
		t.Fatalf("err = %v, want nil (acknowledged)", err)
	}
	if len(h.links.upserts) != 0 || len(h.states.upserts) != 1 || h.states.upserts[0].Status != domain.SalesforceIngestFailed {
		t.Errorf("links %d ledger %+v", len(h.links.upserts), h.states.upserts)
	}
}

func TestLinkedOpportunityIngest_GuardSkipsSameVersion(t *testing.T) {
	h := newLinkHarness(map[string]string{testOpportunitySfID: testLinkOppRowID}, map[string]string{testLinkProjectSfID: testProjectRowID}, false, false)
	h.states.rows = map[string]domain.SalesforceIngestState{domain.SalesforceIngestEntityLinkedOpportunity + "/" + testLinkSfID: {
		Status: domain.SalesforceIngestSucceeded, EventType: "UPDATED", EventModifiedOn: time.Date(2026, 9, 29, 9, 28, 21, 0, time.UTC),
	}}
	if err := h.svc.HandleEvent(context.Background(), linkEvent("UPDATED")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(h.links.upserts) != 0 {
		t.Errorf("upserts = %d, want 0", len(h.links.upserts))
	}
}

func TestLinkedOpportunityIngest_UpsertErrorRecordsFailed(t *testing.T) {
	h := newLinkHarness(map[string]string{testOpportunitySfID: testLinkOppRowID}, map[string]string{testLinkProjectSfID: testProjectRowID}, false, false)
	h.links.upsertErr = errors.New("boom")
	if err := h.svc.HandleEvent(context.Background(), linkEvent("UPDATED")); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
	if len(h.states.upserts) != 1 || h.states.upserts[0].Status != domain.SalesforceIngestFailed {
		t.Errorf("ledger %+v", h.states.upserts)
	}
}

// TestLinkedOpportunityIngest_Deleted: DELETED hard-deletes by the
// 18-character link_sf_id with a DELETED ledger row and no fetch; a missing
// row is acknowledged.
func TestLinkedOpportunityIngest_Deleted(t *testing.T) {
	for _, rows := range []int64{1, 0} {
		h := newLinkHarness(nil, nil, false, false)
		h.links.deleteRows = rows
		h.linkSE.err = salesentity.NotFound("salesentity: linked opportunity not in search results")
		req := linkEvent("DELETED")
		req.ReferenceID = testLinkSfID[:15]
		if err := h.svc.HandleEvent(context.Background(), req); err != nil {
			t.Fatalf("rows=%d: %v", rows, err)
		}
		if h.linkSE.calls != 1 || len(h.links.deletes) != 1 || h.links.deletes[0] != testLinkSfID {
			t.Errorf("rows=%d: fetches %d deletes %v", rows, h.linkSE.calls, h.links.deletes)
		}
		st := h.links.deleteState[0]
		if st.Entity != domain.SalesforceIngestEntityLinkedOpportunity || st.EventType != domain.SalesforceEventDeleted || st.SfID != testLinkSfID {
			t.Errorf("rows=%d: state %+v", rows, st)
		}
	}
}

// A DELETED ledger row is stamped with the recorded version when that is
// ahead of now (Salesforce's clock runs ahead), so the row is not dropped and
// a later RESTORED is not skipped.
func TestLinkedOpportunityIngest_DeletedStampsLaterRecordedVersion(t *testing.T) {
	h := newLinkHarness(nil, nil, false, false)
	ahead := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	h.states.rows = map[string]domain.SalesforceIngestState{domain.SalesforceIngestEntityLinkedOpportunity + "/" + testLinkSfID: {
		Status: domain.SalesforceIngestSucceeded, EventType: "UPDATED", EventModifiedOn: ahead,
	}}
	h.linkSE.err = salesentity.NotFound("salesentity: linked opportunity not in search results")
	if err := h.svc.HandleEvent(context.Background(), linkEvent("DELETED")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if got := h.links.deleteState[0].EventModifiedOn; !got.Equal(ahead) {
		t.Errorf("DELETED version = %v, want the recorded %v", got, ahead)
	}
}

// A keyless parent project (D5) is recorded FAILED and acknowledged, not
// returned as a 400 that Service Bus would redeliver and dead-letter.
func TestLinkedOpportunityIngest_KeylessProjectAcknowledged(t *testing.T) {
	h := newLinkHarness(map[string]string{testOpportunitySfID: testLinkOppRowID}, nil, true, true)
	h.projSE.project.Key = nil
	if err := h.svc.HandleEvent(context.Background(), linkEvent("UPDATED")); err != nil {
		t.Fatalf("err = %v, want nil (acknowledged)", err)
	}
	if len(h.links.upserts) != 0 {
		t.Errorf("links upserted = %d, want 0", len(h.links.upserts))
	}
	var linkFailed bool
	for _, st := range h.states.upserts {
		if st.Entity == domain.SalesforceIngestEntityLinkedOpportunity && st.Status == domain.SalesforceIngestFailed {
			linkFailed = true
		}
	}
	if !linkFailed {
		t.Errorf("no FAILED linked opportunity ledger row: %+v", h.states.upserts)
	}
}

func TestRetryWorker_LinkedOpportunityRetrier(t *testing.T) {
	msg := fmt.Sprintf("project not found for sfId %q", testLinkProjectSfID)
	states := &fakeIngestStateRepo{failed: []domain.SalesforceIngestState{{
		Entity: domain.SalesforceIngestEntityLinkedOpportunity, SfID: testLinkSfID, Status: domain.SalesforceIngestFailed, LastError: &msg, AttemptCount: 1,
	}}}
	h := newLinkHarness(map[string]string{testOpportunitySfID: testLinkOppRowID}, map[string]string{testLinkProjectSfID: testProjectRowID}, false, false)
	w := NewSalesforceIngestRetryWorker(nil, nil, states, time.Minute)
	w.EntityRetriers[domain.SalesforceIngestEntityLinkedOpportunity] = h.svc.RetryLinkedOpportunityIngest
	w.RunOnce(context.Background())
	if len(h.links.upserts) != 1 || h.links.upsertState[0].EventType != domain.SalesforceEventUpdated {
		t.Errorf("link upserts %+v", h.links.upsertState)
	}
}

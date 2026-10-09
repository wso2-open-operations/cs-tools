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

package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/statuspage"
)

const statusPageOutageID = "23247554-19c0-48b3-a8be-8956cead8e2f"

func statusPageRecord(event string) eventbus.Record {
	return eventbus.Record{Value: []byte(`{"type":"outage.status_page_due","entityId":"` + statusPageOutageID + `","payload":{` +
		`"webhookId":"8b1a7c1e-0000-0000-0000-000000000001","claimToken":"tok-1","outageId":"` + statusPageOutageID + `","number":"OUT0010021",` +
		`"cloud":"choreo","event":"` + event + `","timestamp":"2026-10-09T06:54:00.000Z"}}`)}
}

type fakeStatusPage struct {
	err   error
	posts []string
}

func (f *fakeStatusPage) Post(_ context.Context, cloud, event, ts string) error {
	f.posts = append(f.posts, cloud+"|"+event+"|"+ts)
	return f.err
}

type fakeDeliveryReports struct {
	claimErr error
	err      error
	claims   []string
	reports  []string
}

func (f *fakeDeliveryReports) ClaimCloudStatusWebhook(_ context.Context, id, token string) error {
	f.claims = append(f.claims, id+"|"+token)
	return f.claimErr
}

func (f *fakeDeliveryReports) RecordCloudStatusDelivery(_ context.Context, id, token string, d entity.CloudStatusDelivery) error {
	state := "failed:" + d.Error
	if d.Unknown {
		state = "unknown:" + d.Error
	}
	if d.Delivered {
		state = "delivered"
	}
	f.reports = append(f.reports, id+"|"+token+"|"+state)
	return f.err
}

func TestDispatcher_Handle_StatusPageDue_PostsAndReportsDelivered(t *testing.T) {
	page, reports := &fakeStatusPage{}, &fakeDeliveryReports{}
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).WithStatusPage(page, reports)

	if err := d.Handle(context.Background(), statusPageRecord("outage_begin")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(page.posts) != 1 || page.posts[0] != "choreo|outage_begin|2026-10-09T06:54:00.000Z" {
		t.Errorf("posts = %v, want the payload's cloud, event and timestamp verbatim", page.posts)
	}
	if len(reports.reports) != 1 || reports.reports[0] != "8b1a7c1e-0000-0000-0000-000000000001|tok-1|delivered" {
		t.Errorf("reports = %v, want the webhook reported delivered", reports.reports)
	}
}

// A failed post is one failed attempt, reported, and the record is
// acknowledged: csm-scheduled-tasks is the one retrier.
func TestDispatcher_Handle_StatusPageDue_FailureIsReportedNotRetried(t *testing.T) {
	page := &fakeStatusPage{err: errors.New("dashboard for choreo returned 503")}
	reports := &fakeDeliveryReports{}
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).WithStatusPage(page, reports)

	if err := d.Handle(context.Background(), statusPageRecord("outage_end")); err != nil {
		t.Fatalf("a failed post must not be retried by the consumer, got %v", err)
	}
	if len(reports.reports) != 1 || !strings.Contains(reports.reports[0], "failed:dashboard for choreo returned 503") {
		t.Errorf("reports = %v, want one failed attempt with the reason", reports.reports)
	}
}

// Not configured here: claimed and reported as a definite failure (nothing was
// sent), so the scheduled task posts it on its next tick.
func TestDispatcher_Handle_StatusPageDue_UnconfiguredIsHandedBack(t *testing.T) {
	reports := &fakeDeliveryReports{}
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).WithStatusPage(nil, reports)

	if err := d.Handle(context.Background(), statusPageRecord("outage_begin")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(reports.reports) != 1 || !strings.Contains(reports.reports[0], "not configured") {
		t.Errorf("reports = %v, want it reported undelivered as not configured", reports.reports)
	}
}

// A report that cannot reach entity-service is dropped, not retried: the
// lease hands the row to the scheduled task.
func TestDispatcher_Handle_StatusPageDue_ReportFailureIsNotAnError(t *testing.T) {
	page, reports := &fakeStatusPage{}, &fakeDeliveryReports{err: errors.New("entity down")}
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).WithStatusPage(page, reports)

	if err := d.Handle(context.Background(), statusPageRecord("outage_begin")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(page.posts) != 1 {
		t.Errorf("posts = %v, want exactly one", page.posts)
	}
}

// A redelivered event, or one whose reservation the scheduled task took over,
// loses the claim and posts nothing.
func TestDispatcher_Handle_StatusPageDue_LostClaimPostsNothing(t *testing.T) {
	page, reports := &fakeStatusPage{}, &fakeDeliveryReports{claimErr: entity.ErrWebhookNotClaimable}
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).WithStatusPage(page, reports)

	if err := d.Handle(context.Background(), statusPageRecord("outage_begin")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(page.posts) != 0 || len(reports.reports) != 0 {
		t.Errorf("a lost claim must post and report nothing, got posts=%v reports=%v", page.posts, reports.reports)
	}
	if len(reports.claims) != 1 || reports.claims[0] != "8b1a7c1e-0000-0000-0000-000000000001|tok-1" {
		t.Errorf("claims = %v, want the event's webhook id and token", reports.claims)
	}
}

// A claim that cannot be made (entity-service unreachable) posts nothing.
func TestDispatcher_Handle_StatusPageDue_ClaimErrorPostsNothing(t *testing.T) {
	page, reports := &fakeStatusPage{}, &fakeDeliveryReports{claimErr: errors.New("entity down")}
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).WithStatusPage(page, reports)

	if err := d.Handle(context.Background(), statusPageRecord("outage_begin")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(page.posts) != 0 {
		t.Errorf("unclaimed must not post, got %v", page.posts)
	}
}

// A post that reached the dashboard with no answer is reported unknown, so it
// is never re-sent.
func TestDispatcher_Handle_StatusPageDue_TimeoutIsUnknown(t *testing.T) {
	page := &fakeStatusPage{err: &statuspage.UnknownOutcomeError{Err: errors.New("context deadline exceeded")}}
	reports := &fakeDeliveryReports{}
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).WithStatusPage(page, reports)

	if err := d.Handle(context.Background(), statusPageRecord("outage_end")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(reports.reports) != 1 || !strings.Contains(reports.reports[0], "|unknown:") {
		t.Errorf("reports = %v, want an unknown outcome", reports.reports)
	}
}

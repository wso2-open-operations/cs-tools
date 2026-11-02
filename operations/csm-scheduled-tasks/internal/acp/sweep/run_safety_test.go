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

package sweep

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/notify"
)

// csmWriterFunc is a CSMWriter answering every write with err.
type csmWriterFunc func() error

func (f csmWriterFunc) UpdateProject(context.Context, string, []byte) ([]byte, error) {
	return nil, f()
}

// threeDueProjects lists three projects each due its 90-day notice at now.
func threeDueProjects(now time.Time) *mockEntityReader {
	end := now.AddDate(0, 0, 89).Format(time.RFC3339)
	return &mockEntityReader{
		searchProjectsFn: func(context.Context, []byte) ([]byte, error) {
			return []byte(`{"projects":[
				{"id":"p1","endDate":"` + end + `"},
				{"id":"p2","endDate":"` + end + `"},
				{"id":"p3","endDate":"` + end + `"}
			],"total":3,"hasMore":false}`), nil
		},
	}
}

func delivering() *mockNotifier {
	return &mockNotifier{sendFn: func(context.Context, notify.Notice) (bool, error) { return true, nil }}
}

// When CSM refuses the record of a notice already sent, for a reason every
// project's write would hit (401/403, 429, 5xx, no response), the run stops:
// carrying on would send every remaining due notice unrecorded, and the next
// run would send them all again.
func TestRun_CSMNotTakingWritesStopsTheRun(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"401", &apierror.Error{StatusCode: 401}},
		{"403", &apierror.Error{StatusCode: 403}},
		{"429", &apierror.Error{StatusCode: 429}},
		{"503", &apierror.Error{StatusCode: 503}},
		{"no response", errors.New("dial tcp: connection refused")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := threeDueProjects(now)
			ntf := delivering()
			updater := NewWriter(csmWriterFunc(func() error { return tc.err }), nil)

			result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, now, "", nil)
			var stopped *writesFailingError
			if !errors.As(err, &stopped) || !strings.Contains(err.Error(), "2 project(s) not evaluated") {
				t.Fatalf("err = %v, want the run stopped with 2 projects left", err)
			}
			if len(ntf.sent) != 1 {
				t.Errorf("notices sent = %d, want only the first project's", len(ntf.sent))
			}
			if len(result.Failures) != 1 || result.Failures[0].ProjectID != "p1" {
				t.Errorf("failures = %+v, want p1 only", result.Failures)
			}
		})
	}
}

// A write refused for the project's own reason (a 4xx other than 401, 403
// and 429) fails that project only; the run goes on.
func TestRun_AProjectsOwnWriteErrorDoesNotStopTheRun(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	reader := threeDueProjects(now)
	ntf := delivering()
	updater := NewWriter(csmWriterFunc(func() error { return &apierror.Error{StatusCode: 422} }), nil)

	result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, now, "", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(ntf.sent) != 3 || len(result.Failures) != 3 {
		t.Errorf("sent=%d failures=%d, want every project tried", len(ntf.sent), len(result.Failures))
	}
}

// Each listed project is read again from CSM right before it is decided:
// the listing can be as old as the run. Here the 90-day notice was recorded
// after the listing was read, so it is not sent again.
func TestRun_DecidesFromAFreshCSMRead(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := now.AddDate(0, 0, 89).Format(time.RFC3339)
	reader := &mockEntityReader{
		searchProjectsFn: func(context.Context, []byte) ([]byte, error) {
			return []byte(`{"projects":[{"id":"p1","endDate":"` + end + `"}],"total":1,"hasMore":false}`), nil
		},
		getProjectFn: func(context.Context, string) ([]byte, error) {
			return []byte(`{"id":"p1","endDate":"` + end + `","suspensionProcessState":` +
				`{"based_on_subscription_end_date":{"event_type":"90_days_notice","actionSendEmailNotification":"SUCCESSFUL"}}}`), nil
		},
	}
	ntf := delivering()

	if _, err := Run(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, ntf, now, "", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(reader.getProjectCalls) != 1 || reader.getProjectCalls[0] != "p1" {
		t.Errorf("re-reads = %v, want p1", reader.getProjectCalls)
	}
	if len(ntf.sent) != 0 {
		t.Errorf("notices sent = %d, want none: the fresh read shows it recorded", len(ntf.sent))
	}
}

// A project whose re-read fails is a failure of that project; the run goes
// on.
func TestRun_AFailedReReadFailsThatProjectOnly(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	reader := threeDueProjects(now)
	reader.getProjectFn = func(_ context.Context, id string) ([]byte, error) {
		if id == "p2" {
			return nil, errors.New("entity-service timeout")
		}
		return reader.listed[id], nil
	}
	ntf := delivering()

	result, err := Run(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, ntf, now, "", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Failures) != 1 || result.Failures[0].ProjectID != "p2" || len(ntf.sent) != 2 {
		t.Errorf("failures=%+v sent=%d, want p2 failed and the other two notified", result.Failures, len(ntf.sent))
	}
}

// A project with no Salesforce id, or a Salesforce date that does not parse,
// is skipped with a warning: no retry can fix its record.
func TestSalesforceDetails_UnusableRecordIsADataError(t *testing.T) {
	bad := partnerFixture()
	p := bad.projects["a0dP"]
	p.EndDate = sp("13/10/2026")
	bad.projects["a0dP"] = p

	noSfID := csmPartnerProject()
	noSfID.SfID = nil
	for _, tc := range []struct {
		name string
		sf   *fakeSalesforce
		proj project
	}{
		{"no Salesforce id", partnerFixture(), noSfID},
		{"unparseable end date", bad, csmPartnerProject()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := NewSalesforceDetails(tc.sf).forProject(context.Background(), tc.proj)
			var de *dataError
			if !errors.As(err, &de) {
				t.Errorf("err = %v, want a dataError", err)
			}
		})
	}
}

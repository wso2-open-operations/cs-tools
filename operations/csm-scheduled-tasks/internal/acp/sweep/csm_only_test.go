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
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakeAccounts struct {
	pages [][]string // account ids per page
	total int
	err   error
	calls int
	body  []byte
}

func (f *fakeAccounts) SearchAccounts(_ context.Context, body []byte) ([]byte, error) {
	f.calls++
	f.body = body
	if f.err != nil {
		return nil, f.err
	}
	i := f.calls - 1
	var ids []string
	if i < len(f.pages) {
		for _, id := range f.pages[i] {
			ids = append(ids, `{"id":"`+id+`"}`)
		}
	}
	return []byte(`{"accounts":[` + strings.Join(ids, ",") + `],"total":` + fmt.Sprint(f.total) + `,"hasMore":` + fmt.Sprint(i+1 < len(f.pages)) + `}`), nil
}

func csmOnlyListing(accountID string) *mockEntityReader {
	end := screenNow.AddDate(0, 0, 89).Format(time.RFC3339)
	acc := `,"account":{"id":"` + accountID + `","isPartner":false}`
	if accountID == "" {
		acc = ""
	}
	p := `{"id":"p1","subscriptionType":"subscription","suspensionProcessOverridden":false,"endDate":"` + end + `"` + acc + `}`
	return &mockEntityReader{searchProjectsFn: func(context.Context, []byte) ([]byte, error) {
		return []byte(`{"projects":[` + p + `],"total":1,"hasMore":false}`), nil
	}}
}

func TestCSMOnlyDetails_StatusComesFromTheCustomerList(t *testing.T) {
	for _, tc := range []struct {
		name                                string
		accountID                           string
		wantActed, wantStatus, wantNoStatus int
	}{
		{"account in the Customer list", "acc-customer", 1, 0, 0},
		{"account not in the list", "acc-other", 0, 1, 0},
		{"no account", "", 0, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := csmOnlyListing(tc.accountID)
			accounts := &fakeAccounts{pages: [][]string{{"acc-customer"}}, total: 1}
			ntf := delivering()
			result, err := Run(context.Background(), reader, CSMOnlyDetails(reader, accounts), &mockProjectUpdater{}, ntf, screenNow, "", nil, 0)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if result.ProjectsActed != tc.wantActed || result.ProjectsFilteredStatus != tc.wantStatus || result.ProjectsFilteredNoStatus != tc.wantNoStatus {
				t.Errorf("acted=%d status=%d noStatus=%d, want %d/%d/%d", result.ProjectsActed, result.ProjectsFilteredStatus, result.ProjectsFilteredNoStatus, tc.wantActed, tc.wantStatus, tc.wantNoStatus)
			}
			if (len(ntf.sent) > 0) != (tc.wantActed == 1) {
				t.Errorf("notices sent = %d with acted=%d", len(ntf.sent), tc.wantActed)
			}
			if !strings.Contains(string(accounts.body), `"lifeCycle":"Customer"`) {
				t.Errorf("accounts request = %s, want the lifeCycle filter", accounts.body)
			}
		})
	}
}

func TestCSMOnlyDetails_PagesTheCustomerList(t *testing.T) {
	reader := csmOnlyListing("acc-2")
	accounts := &fakeAccounts{pages: [][]string{{"acc-1"}, {"acc-2"}}, total: 2}
	result, err := Run(context.Background(), reader, CSMOnlyDetails(reader, accounts), &mockProjectUpdater{}, delivering(), screenNow, "", nil, 0)
	if err != nil || accounts.calls != 2 || result.ProjectsActed != 1 {
		t.Errorf("err=%v calls=%d acted=%d, want both pages read and the project evaluated", err, accounts.calls, result.ProjectsActed)
	}
}

// With no way to tell the Customers, nothing is evaluated, sent or written and
// the run fails visibly (fail closed).
func TestCSMOnlyDetails_UnreadableCustomerListStopsTheRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		acc  *fakeAccounts
	}{
		{"read error", &fakeAccounts{err: errors.New("entity-service unavailable")}},
		{"incomplete listing", &fakeAccounts{pages: [][]string{{"acc-customer"}}, total: 5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := csmOnlyListing("acc-customer")
			ntf, updater := delivering(), &mockProjectUpdater{}
			result, err := Run(context.Background(), reader, CSMOnlyDetails(reader, tc.acc), updater, ntf, screenNow, "", nil, 0)
			if err == nil || result.ProjectsEvaluated != 0 || len(ntf.sent) != 0 || len(updater.calls) != 0 || len(reader.getProjectCalls) != 0 {
				t.Errorf("err=%v result=%+v sent=%d writes=%d, want a failed run with nothing evaluated", err, result, len(ntf.sent), len(updater.calls))
			}
		})
	}
}

// A single-project run prepares the list too.
func TestCSMOnlyDetails_SingleProjectRunNeedsTheList(t *testing.T) {
	reader := csmOnlyListing("acc-customer")
	reader.getProjectFn = func(context.Context, string) ([]byte, error) {
		end := screenNow.AddDate(0, 0, 89).Format(time.RFC3339)
		return []byte(`{"id":"p1","subscriptionType":"subscription","suspensionProcessOverridden":false,"endDate":"` + end + `","account":{"id":"acc-customer","isPartner":false}}`), nil
	}
	accounts := &fakeAccounts{pages: [][]string{{"acc-customer"}}, total: 1}
	result, err := Run(context.Background(), reader, CSMOnlyDetails(reader, accounts), &mockProjectUpdater{}, delivering(), screenNow, "p1", nil, 0)
	if err != nil || accounts.calls != 1 || result.ProjectsActed != 1 {
		t.Errorf("err=%v calls=%d acted=%d", err, accounts.calls, result.ProjectsActed)
	}
}

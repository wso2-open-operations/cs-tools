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

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// fakeAccountSearch answers POST /accounts/search as the entity service does:
// every account whose name or number contains filters.searchQuery, ignoring
// case, one page of at most limit from pagination.offset. It records each
// query, and fails on a body without filters.searchQuery (the shape the real
// search reads).
type fakeAccountSearch struct {
	accounts []searchedAccount
	limit    int
	err      error
	queries  []string
}

// SearchAccounts implements accountSearcher.
func (f *fakeAccountSearch) SearchAccounts(_ context.Context, body []byte) ([]byte, error) {
	var req struct {
		Filters *struct {
			SearchQuery string `json:"searchQuery"`
		} `json:"filters"`
		Pagination struct {
			Offset int `json:"offset"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Filters == nil {
		return nil, fmt.Errorf("not an accounts search body: %s", body)
	}
	f.queries = append(f.queries, req.Filters.SearchQuery)
	if f.err != nil {
		return nil, f.err
	}
	q := strings.ToLower(req.Filters.SearchQuery)
	var out []searchedAccount
	for _, a := range f.accounts {
		if strings.Contains(strings.ToLower(a.Name), q) || strings.Contains(strings.ToLower(a.Number), q) {
			out = append(out, a)
		}
	}
	out = out[min(req.Pagination.Offset, len(out)):]
	hasMore := false
	if f.limit > 0 && len(out) > f.limit {
		out, hasMore = out[:f.limit], true
	}
	return json.Marshal(map[string]any{"accounts": out, "hasMore": hasMore})
}

// testAccounts is the fixture's accounts (made up): a name contained in
// another name, two accounts of one name, and a number contained in another.
var testAccounts = []searchedAccount{
	{ID: "a0", Number: "ACC0001", Name: "Example Corp"},
	{ID: "a1", Number: "ACC0010", Name: "Example Corp Europe"},
	{ID: "a2", Number: "ACC0002", Name: "Summit Enterprises"},
	{ID: "a3", Number: "ACC0003", Name: "Quartz Labs"},
	{ID: "a4", Number: "ACC0004", Name: "Quartz Labs"},
}

// accountDashboard is a generated dashboard of one count widget whose query
// has an account filter of op on the given names, beside an open-state filter.
func accountDashboard(t *testing.T, op string, names ...string) dashboard.Dashboard {
	t.Helper()
	b, _ := json.Marshal(names)
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` +
		`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[` +
		`{"field":"state","op":"in","values":["open"]},{"field":"account","op":"` + op + `","values":` + string(b) + `}]}}]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	return d
}

// TestWithDashboardAccounts_ResolvesTheGeneratedDashboard: an account's name
// or number as the user wrote it becomes an accountId filter with the same
// op on its id; an exact number wins over a longer one containing it, an
// exact name over a longer name, and a value found in one account only
// names that one; an account named twice is one id; each distinct value is
// searched once; the state filter is as generated.
func TestWithDashboardAccounts_ResolvesTheGeneratedDashboard(t *testing.T) {
	for _, op := range []string{"in", "notIn"} {
		search := &fakeAccountSearch{accounts: testAccounts}
		inner := &fakeGenerator{result: GeneratedDashboard{Dashboard: accountDashboard(t, op, "acc0001", "Example Corp", "summit")}}
		got, err := WithDashboardAccounts(inner, search).Generate(context.Background(), "x", nil)
		if err != nil {
			t.Fatalf("%s: Generate: %v", op, err)
		}
		q := got.Dashboard.Widgets[0].Query
		if projectFilter(q, "account") != nil {
			t.Errorf("%s: an account filter is left: %v", op, q)
		}
		if f := projectFilter(q, "accountId"); f == nil || f["op"] != op || fmt.Sprint(f["values"]) != "[a0 a2]" {
			t.Errorf("%s: accountId filter = %v, want %s [a0 a2]", op, f, op)
		}
		if projectFilter(q, "state") == nil {
			t.Errorf("%s: the state filter is gone: %v", op, q)
		}
		if fmt.Sprint(search.queries) != "[acc0001 Example Corp summit]" {
			t.Errorf("%s: searched %q, want each distinct value once", op, search.queries)
		}
	}
}

// TestWithDashboardAccounts_RefusesAnAccountItCannotPlace: a value no account
// matches, one several match, or a name two accounts share, is an
// errDashboardAccount whose message lists the candidates by name and number;
// a failed search is an errDashboardAccountLookup.
func TestWithDashboardAccounts_RefusesAnAccountItCannotPlace(t *testing.T) {
	for _, tc := range []struct {
		name, account string
		search        *fakeAccountSearch
		want          string
	}{
		{"no account", "Orion", &fakeAccountSearch{accounts: testAccounts}, `No account is named like "Orion".`},
		{"several accounts", "Corp", &fakeAccountSearch{accounts: testAccounts},
			`"Corp" matches several accounts (Example Corp (ACC0001), Example Corp Europe (ACC0010)); name the one you mean.`},
		{"a name two accounts share", "quartz labs", &fakeAccountSearch{accounts: testAccounts},
			`"quartz labs" matches several accounts (Quartz Labs (ACC0003), Quartz Labs (ACC0004)); name the one you mean.`},
		{"more than one page, read to the end", "ACC", &fakeAccountSearch{accounts: testAccounts, limit: 2},
			`"ACC" matches several accounts (Example Corp (ACC0001), Example Corp Europe (ACC0010), Summit Enterprises (ACC0002), Quartz Labs (ACC0003), Quartz Labs (ACC0004)); name the one you mean.`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := WithDashboardAccounts(&fakeGenerator{result: GeneratedDashboard{Dashboard: accountDashboard(t, "notIn", tc.account)}}, tc.search).Generate(context.Background(), "x", nil)
			if !errors.Is(err, errDashboardAccount) {
				t.Fatalf("err = %v, want errDashboardAccount", err)
			}
			if msg := dashboardTeamMessage(err); msg != tc.want {
				t.Errorf("message = %q, want %q", msg, tc.want)
			}
		})
	}

	gen := WithDashboardAccounts(&fakeGenerator{result: GeneratedDashboard{Dashboard: accountDashboard(t, "in", "Summit")}}, &fakeAccountSearch{err: errors.New("status 500")})
	if _, err := gen.Generate(context.Background(), "x", nil); !errors.Is(err, errDashboardAccountLookup) {
		t.Errorf("a failed search: err = %v, want errDashboardAccountLookup", err)
	}
}

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
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// fakeCaseAggregate answers POST /cases/aggregate with reply (or err), and
// records each request body.
type fakeCaseAggregate struct {
	reply  string
	err    error
	bodies []map[string]any
}

// AggregateCases implements caseAggregator.
func (f *fakeCaseAggregate) AggregateCases(_ context.Context, body []byte) ([]byte, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	f.bodies = append(f.bodies, req)
	if f.err != nil {
		return nil, f.err
	}
	return []byte(f.reply), nil
}

// TestListCaseRegions: one aggregate of every case by region, with no
// filters and room for every region; each group becomes a region, its label
// the key when it has none.
func TestListCaseRegions(t *testing.T) {
	agg := &fakeCaseAggregate{reply: `{"groups":[{"key":"apac","label":"APAC","count":4},{"key":"emea","label":"","count":1}],"othersCount":0,"totalRecords":5}`}
	got, err := listCaseRegions(context.Background(), agg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []caseRegion{{Key: "apac", Label: "APAC"}, {Key: "emea", Label: "emea"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("regions = %+v, want %+v", got, want)
	}
	wantBody := []map[string]any{{"groupBy": "region", "maxGroups": float64(regionListGroups)}}
	if !reflect.DeepEqual(agg.bodies, wantBody) {
		t.Errorf("requests = %v, want %v", agg.bodies, wantBody)
	}
}

// TestListCaseRegions_NoRegions: no case with a region is an empty list,
// not a failure.
func TestListCaseRegions_NoRegions(t *testing.T) {
	got, err := listCaseRegions(context.Background(), &fakeCaseAggregate{reply: `{"groups":[],"othersCount":0,"totalRecords":0}`})
	if err != nil || len(got) != 0 {
		t.Errorf("got %+v, %v; want no regions and no error", got, err)
	}
}

// TestListCaseRegions_Failures: a failed aggregate, a reply that is not the
// aggregate's, and more regions than one reply holds are lookup failures,
// never a shorter list.
func TestListCaseRegions_Failures(t *testing.T) {
	for name, agg := range map[string]*fakeCaseAggregate{
		"the aggregate failed": {err: errors.New("entity service: 503")},
		"not the aggregate":    {reply: `<html>`},
		"regions left over":    {reply: `{"groups":[{"key":"apac","label":"APAC","count":4}],"othersCount":2,"totalRecords":6}`},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := listCaseRegions(context.Background(), agg)
			if !errors.Is(err, errDashboardRegionLookup) || got != nil {
				t.Errorf("got %+v, %v; want errDashboardRegionLookup", got, err)
			}
		})
	}
}

// testRegionsReply is the aggregate's reply for the fixture's regions (made
// up): APAC on four cases, EMEA on one.
const testRegionsReply = `{"groups":[{"key":"apac","label":"APAC","count":4},{"key":"emea","label":"EMEA","count":1}],"othersCount":0,"totalRecords":5}`

// regionDashboard is a generated dashboard of a count widget whose query has
// a region filter of op on regions, beside an open-state filter, and a pie
// whose one slice has a region filter on sliceRegions (no slice when none).
func regionDashboard(t *testing.T, op string, regions []string, sliceRegions ...string) dashboard.Dashboard {
	t.Helper()
	b, _ := json.Marshal(regions)
	widgets := `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[` +
		`{"field":"state","op":"in","values":["open"]},{"field":"region","op":"` + op + `","values":` + string(b) + `}]}}`
	if len(sliceRegions) > 0 {
		sb, _ := json.Marshal(sliceRegions)
		widgets += `,{"id":"p","displayName":"P","resourceType":"security_report_analysis","shape":"pie","gridWidth":6,"query":{},` +
			`"slices":[{"label":"S","query":{"filters":[{"field":"region","op":"in","values":` + string(sb) + `}]}}]}`
	}
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` + widgets + `]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	return d
}

// TestWithDashboardRegions_ResolvesTheGeneratedDashboard: each region as the
// model wrote it, in any case and with spaces, becomes the region as an
// account writes it, each once, in a widget and in a slice, with the op kept;
// the regions are read once for the whole dashboard; the state filter is as
// generated.
func TestWithDashboardRegions_ResolvesTheGeneratedDashboard(t *testing.T) {
	for _, op := range []string{"in", "notIn"} {
		agg := &fakeCaseAggregate{reply: testRegionsReply}
		inner := &fakeGenerator{result: GeneratedDashboard{Dashboard: regionDashboard(t, op, []string{"apac", " APAC ", "Emea"}, "emea ")}}
		got, err := WithDashboardRegions(inner, agg).Generate(context.Background(), "x", nil)
		if err != nil {
			t.Fatalf("%s: Generate: %v", op, err)
		}
		q := got.Dashboard.Widgets[0].Query
		if f := projectFilter(q, "region"); f == nil || f["op"] != op || fmt.Sprint(f["values"]) != "[APAC EMEA]" {
			t.Errorf("%s: region filter = %v, want %s [APAC EMEA]", op, f, op)
		}
		if f := projectFilter(got.Dashboard.Widgets[1].Slices[0].Query, "region"); f == nil || fmt.Sprint(f["values"]) != "[EMEA]" {
			t.Errorf("%s: slice region filter = %v, want [EMEA]", op, f)
		}
		if projectFilter(q, "state") == nil {
			t.Errorf("%s: the state filter is gone: %v", op, q)
		}
		if len(agg.bodies) != 1 {
			t.Errorf("%s: %d region lookups, want 1", op, len(agg.bodies))
		}
	}
}

// TestWithDashboardRegions_LeavesADashboardWithoutRegionsAlone: no region
// filter, no lookup, even when a widget groups by region.
func TestWithDashboardRegions_LeavesADashboardWithoutRegionsAlone(t *testing.T) {
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` +
		`{"id":"w","displayName":"W","resourceType":"case","shape":"bar","gridWidth":6,"query":{"filters":[{"field":"state","op":"in","values":["open"]}]},"groupBy":{"field":"region","maxGroups":10}}]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	agg := &fakeCaseAggregate{err: errors.New("must not be called")}
	if _, err := WithDashboardRegions(&fakeGenerator{result: GeneratedDashboard{Dashboard: d}}, agg).Generate(context.Background(), "x", nil); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(agg.bodies) != 0 {
		t.Errorf("%d region lookups, want none", len(agg.bodies))
	}
}

// TestWithDashboardRegions_RefusesARegionNoCaseHas: a region not among the
// cases' regions is an errDashboardRegion naming it and the regions there
// are, for "in" and "notIn" (which would leave nothing out), in a widget or a
// slice; with no region anywhere, the reason says so; a failed lookup is an
// errDashboardRegionLookup.
func TestWithDashboardRegions_RefusesARegionNoCaseHas(t *testing.T) {
	for _, tc := range []struct {
		name, reply string
		lookupErr   error
		d           func(t *testing.T) dashboard.Dashboard
		wantErr     error
		want        string
	}{
		{"in", testRegionsReply, nil, func(t *testing.T) dashboard.Dashboard { return regionDashboard(t, "in", []string{"APAC", "LATAM"}) },
			errDashboardRegion, `widgets[0].filters[1]: no case's account has the region "LATAM"; the regions are APAC, EMEA`},
		{"notIn", testRegionsReply, nil, func(t *testing.T) dashboard.Dashboard { return regionDashboard(t, "notIn", []string{" Asia Pacific "}) },
			errDashboardRegion, `widgets[0].filters[1]: no case's account has the region "Asia Pacific"; the regions are APAC, EMEA`},
		{"a slice", testRegionsReply, nil, func(t *testing.T) dashboard.Dashboard { return regionDashboard(t, "in", []string{"APAC"}, "NA") },
			errDashboardRegion, `widgets[1].slices[0].filters[0]: no case's account has the region "NA"`},
		{"no regions at all", `{"groups":[],"othersCount":0,"totalRecords":0}`, nil, func(t *testing.T) dashboard.Dashboard { return regionDashboard(t, "in", []string{"APAC"}) },
			errDashboardRegion, `no case's account has a region yet, so "APAC" cannot be filtered on`},
		{"a failed lookup", "", errors.New("status 500"), func(t *testing.T) dashboard.Dashboard { return regionDashboard(t, "in", []string{"APAC"}) },
			errDashboardRegionLookup, "status 500"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agg := &fakeCaseAggregate{reply: tc.reply, err: tc.lookupErr}
			_, err := WithDashboardRegions(&fakeGenerator{result: GeneratedDashboard{Dashboard: tc.d(t)}}, agg).Generate(context.Background(), "x", nil)
			if !errors.Is(err, tc.wantErr) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %v containing %q", err, tc.wantErr, tc.want)
			}
		})
	}
}

// TestWithDashboardRegions_PassesTheGeneratorsErrorOn: a generator error is
// returned as it is, with no lookup.
func TestWithDashboardRegions_PassesTheGeneratorsErrorOn(t *testing.T) {
	agg := &fakeCaseAggregate{reply: testRegionsReply}
	_, err := WithDashboardRegions(&fakeGenerator{err: errGenerateRefused}, agg).Generate(context.Background(), "x", nil)
	if !errors.Is(err, errGenerateRefused) || len(agg.bodies) != 0 {
		t.Errorf("err = %v after %d lookups, want errGenerateRefused and none", err, len(agg.bodies))
	}
}

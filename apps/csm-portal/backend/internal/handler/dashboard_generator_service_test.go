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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmai"
)

// validGenerated is a dashboard as the model writes it (file format, no id).
const validGenerated = `{"displayName":"Top Customers by Open Cases","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,
"widgets":[{"id":"top","displayName":"Top 10","resourceType":"case","shape":"bar","gridWidth":12,
"query":{"filters":[{"field":"state","op":"notIn","values":["closed"]}]},"groupBy":{"field":"account","maxGroups":10}}]}`

type fakeAIClient struct {
	res    csmai.DashboardResult
	err    error
	gotReq csmai.DashboardRequest
}

func (f *fakeAIClient) GenerateDashboard(_ context.Context, req csmai.DashboardRequest) (csmai.DashboardResult, error) {
	f.gotReq = req
	return f.res, f.err
}

func TestServiceGenerator_KeepsTheLastValidCandidate(t *testing.T) {
	ai := &fakeAIClient{res: csmai.DashboardResult{Candidates: []json.RawMessage{
		json.RawMessage(validGenerated), json.RawMessage(`{"displayName":"broken","widgets":[{"id":"x","linkResourceType":"account"}]}`),
	}}}
	got, err := NewServiceDashboardGenerator(ai, nil).Generate(context.Background(), "p", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got.Dashboard.DisplayName != "Top Customers by Open Cases" || ai.gotReq.Prompt != "p" || ai.gotReq.Draft != nil {
		t.Errorf("got %+v, request %+v", got.Dashboard.DisplayName, ai.gotReq)
	}
}

// The "model replied" line carries the call's whole input: the uncached part
// and the prompt cache's write and read counts.
func TestServiceGenerator_LogsTheCacheTokenCounts(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ai := &fakeAIClient{res: csmai.DashboardResult{
		Candidates: []json.RawMessage{json.RawMessage(validGenerated)},
		Usage:      csmai.Usage{InputTokens: 120, CacheCreationInputTokens: 30, CacheReadInputTokens: 15000, OutputTokens: 400},
	}}
	if _, err := NewServiceDashboardGenerator(ai, nil).Generate(context.Background(), "p", nil); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	line := buf.String()
	for _, want := range []string{"inputTokens=120", "cacheCreationInputTokens=30", "cacheReadInputTokens=15000", "outputTokens=400"} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q has no %s", line, want)
		}
	}
}

func TestServiceGenerator_DeclinedAndNoValidCandidate(t *testing.T) {
	var declined *generateDeclinedError
	_, err := NewServiceDashboardGenerator(&fakeAIClient{res: csmai.DashboardResult{Declined: "No revenue field."}}, nil).
		Generate(context.Background(), "p", nil)
	if !errors.As(err, &declined) || declined.Reason != "No revenue field." {
		t.Errorf("err = %v, want the decline", err)
	}
	_, err = NewServiceDashboardGenerator(&fakeAIClient{res: csmai.DashboardResult{Candidates: []json.RawMessage{json.RawMessage(`{}`)}}}, nil).
		Generate(context.Background(), "p", nil)
	if !errors.Is(err, errGenerateUnusable) {
		t.Errorf("err = %v, want errGenerateUnusable", err)
	}
}

// TestServiceGenerateError_MapsEveryCode checks that every AI service error
// code maps to its generate error (only a timeout counts as one), and that a
// transport failure is a failed call.
func TestServiceGenerateError_MapsEveryCode(t *testing.T) {
	for code, want := range map[string]error{
		csmai.CodeNotConfigured:   ErrGenerateUnavailable,
		csmai.CodeRefused:         errGenerateRefused,
		csmai.CodeTruncated:       errGenerateTruncated,
		csmai.CodeUnusable:        errGenerateUnusable,
		csmai.CodeTimeout:         errGenerateCall,
		csmai.CodeModelCallFailed: errGenerateCall,
		csmai.CodeUsageLimit:      errGenerateUsageLimit,
		csmai.CodeInvalidRequest:  errGenerateCall,
	} {
		err := serviceGenerateError(&csmai.Error{StatusCode: 500, Code: code})
		if !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", code, err, want)
		}
		if (code == csmai.CodeTimeout) != isTimeout(err) {
			t.Errorf("%s: isTimeout = %v", code, isTimeout(err))
		}
	}
	if err := serviceGenerateError(csmai.ErrCall); !errors.Is(err, errGenerateCall) {
		t.Errorf("transport: err = %v", err)
	}
}

func TestServiceGenerator_SendsOnlyTheTeamsADashboardCanFilterBy(t *testing.T) {
	ai := &fakeAIClient{res: csmai.DashboardResult{Candidates: []json.RawMessage{json.RawMessage(validGenerated)}}}
	if _, err := NewServiceDashboardGenerator(ai, testTeamResults(t)).Generate(context.Background(), "p", nil); err != nil {
		t.Fatal(err)
	}
	if len(ai.gotReq.Teams) == 0 {
		t.Fatal("no teams sent")
	}
	want := map[string]bool{}
	for _, team := range newDashboardTeams(testTeamResults(t)).list {
		if team.GroupID != "" {
			want[team.Key] = true
		}
	}
	for i, team := range ai.gotReq.Teams {
		if !want[team.Key] || team.Name == "" || (team.Field != "creTeam" && team.Field != "sreTeam") || team.Family == "" {
			t.Errorf("team %d = %+v, want a registry team with a group, its name, family and field", i, team)
		}
	}
	if len(ai.gotReq.Teams) != len(want) {
		t.Errorf("sent %d teams, want %d (every team with a group)", len(ai.gotReq.Teams), len(want))
	}
}

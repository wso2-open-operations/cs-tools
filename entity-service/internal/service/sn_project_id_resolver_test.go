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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

type snSearchPage struct {
	projects []snProject
	total    int
}

// fakeSNPoster serves projects/search pages in order and records each call.
type fakeSNPoster struct {
	pages    []snSearchPage
	err      error
	paths    []string
	tokens   []string
	payloads []snSearchProjectsPayload
}

func (f *fakeSNPoster) Post(_ context.Context, path, token string, payload any) (json.RawMessage, error) {
	f.paths = append(f.paths, path)
	f.tokens = append(f.tokens, token)
	f.payloads = append(f.payloads, payload.(snSearchProjectsPayload))
	if f.err != nil {
		return nil, f.err
	}
	i := len(f.paths) - 1
	if i >= len(f.pages) {
		return json.RawMessage(`{"projects":[],"totalRecords":0}`), nil
	}
	return json.Marshal(snProjectsResponse{Projects: f.pages[i].projects, TotalRecords: f.pages[i].total})
}

const (
	snSysidA = "0123456789abcdef0123456789abcdef"
	snSysidB = "fedcba9876543210fedcba9876543210"
	snUUIDA  = "01234567-89ab-cdef-0123-456789abcdef"
)

func snProj(sysid, key string) snProject { return snProject{ID: sysid, Key: key} }

func TestSNProjectIDResolver_ExactKeyAmongSubstringMatches(t *testing.T) {
	f := &fakeSNPoster{pages: []snSearchPage{{projects: []snProject{
		snProj(snSysidB, "ACME2"), snProj(snSysidA, "ACME"), snProj("ffffffffffffffffffffffffffffffff", "BIGACME"),
	}, total: 3}}}
	id, found, err := (&snProjectIDResolver{client: f}).ResolveProjectID(context.Background(), " ACME ")
	if err != nil || !found || id != snUUIDA {
		t.Fatalf("id=%q found=%v err=%v, want %q", id, found, err, snUUIDA)
	}
	// No end-user token is ever forwarded; the key goes in as the free-text query.
	if f.paths[0] != "/projects/search" || f.tokens[0] != "" || f.payloads[0].Filters.SearchQuery != "ACME" {
		t.Errorf("call = %s token=%q payload=%+v", f.paths[0], f.tokens[0], f.payloads[0])
	}
}

func TestSNProjectIDResolver_NoMatchIsNotFound(t *testing.T) {
	cases := map[string][]snProject{
		"empty result":        nil,
		"only longer keys":    {snProj(snSysidA, "ACME2"), snProj(snSysidB, "XACME")},
		"only different case": {snProj(snSysidA, "acme")},
	}
	for name, projects := range cases {
		f := &fakeSNPoster{pages: []snSearchPage{{projects: projects, total: len(projects)}}}
		id, found, err := (&snProjectIDResolver{client: f}).ResolveProjectID(context.Background(), "ACME")
		if err != nil || found || id != "" {
			t.Errorf("%s: id=%q found=%v err=%v, want not found", name, id, found, err)
		}
	}
}

func TestSNProjectIDResolver_SeveralDistinctMatchesFail(t *testing.T) {
	f := &fakeSNPoster{pages: []snSearchPage{{projects: []snProject{snProj(snSysidA, "ACME"), snProj(snSysidB, "ACME")}, total: 2}}}
	_, found, err := (&snProjectIDResolver{client: f}).ResolveProjectID(context.Background(), "ACME")
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) || found {
		t.Fatalf("found=%v err=%v, want ConflictError", found, err)
	}
	if strings.HasPrefix(strings.ToLower(ce.Msg), "project not found") {
		t.Errorf("ambiguity must not look like a retryable miss: %q", ce.Msg)
	}
}

// A project repeated across pages (the window shifts while paging) is one match.
func TestSNProjectIDResolver_SameIDTwiceIsOneMatch(t *testing.T) {
	f := &fakeSNPoster{pages: []snSearchPage{{projects: []snProject{snProj(snSysidA, "ACME"), snProj(strings.ToUpper(snSysidA), "ACME")}, total: 2}}}
	id, found, err := (&snProjectIDResolver{client: f}).ResolveProjectID(context.Background(), "ACME")
	if err != nil || !found || id != snUUIDA {
		t.Fatalf("id=%q found=%v err=%v", id, found, err)
	}
}

func TestSNProjectIDResolver_PagesUntilTotal(t *testing.T) {
	page1 := make([]snProject, snProjectResolvePageSize)
	for i := range page1 {
		page1[i] = snProj(fmt.Sprintf("%032x", i+1), fmt.Sprintf("ACME-%d", i))
	}
	f := &fakeSNPoster{pages: []snSearchPage{
		{projects: page1, total: snProjectResolvePageSize + 1},
		{projects: []snProject{snProj(snSysidA, "ACME")}, total: snProjectResolvePageSize + 1},
	}}
	id, found, err := (&snProjectIDResolver{client: f}).ResolveProjectID(context.Background(), "ACME")
	if err != nil || !found || id != snUUIDA {
		t.Fatalf("id=%q found=%v err=%v", id, found, err)
	}
	if len(f.paths) != 2 || f.payloads[1].Pagination.Offset != snProjectResolvePageSize {
		t.Errorf("calls = %d, second offset = %d", len(f.paths), f.payloads[1].Pagination.Offset)
	}
}

func TestSNProjectIDResolver_TooManyCandidatesFails(t *testing.T) {
	full := make([]snProject, snProjectResolvePageSize)
	for i := range full {
		full[i] = snProj(fmt.Sprintf("%032x", i+1), fmt.Sprintf("ACME-%d", i))
	}
	pages := make([]snSearchPage, snProjectResolveMaxPages+1)
	for i := range pages {
		pages[i] = snSearchPage{projects: full, total: 100000}
	}
	f := &fakeSNPoster{pages: pages}
	_, found, err := (&snProjectIDResolver{client: f}).ResolveProjectID(context.Background(), "ACME")
	if err == nil || found || len(f.paths) != snProjectResolveMaxPages {
		t.Fatalf("found=%v err=%v calls=%d, want an error after %d pages", found, err, len(f.paths), snProjectResolveMaxPages)
	}
}

func TestSNProjectIDResolver_MalformedIDAndClientErrors(t *testing.T) {
	f := &fakeSNPoster{pages: []snSearchPage{{projects: []snProject{snProj("not-a-sysid", "ACME")}, total: 1}}}
	if _, found, err := (&snProjectIDResolver{client: f}).ResolveProjectID(context.Background(), "ACME"); err == nil || found {
		t.Errorf("malformed id: found=%v err=%v, want an error", found, err)
	}
	down := &apierror.ServiceUnavailableError{Msg: "down"}
	if _, found, err := (&snProjectIDResolver{client: &fakeSNPoster{err: down}}).ResolveProjectID(context.Background(), "ACME"); !errors.Is(err, down) || found {
		t.Errorf("client error: found=%v err=%v", found, err)
	}
	var ve *apierror.ValidationError
	if _, _, err := (&snProjectIDResolver{client: &fakeSNPoster{}}).ResolveProjectID(context.Background(), "  "); !errors.As(err, &ve) {
		t.Errorf("empty key: err=%v, want ValidationError", err)
	}
}

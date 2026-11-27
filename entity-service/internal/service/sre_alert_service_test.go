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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeSreAlertRepo struct {
	sort          domain.SreAlertSort
	offset, limit int
	detail        *domain.SreAlertDetail
	facetsSince   time.Time
}

func (r *fakeSreAlertRepo) Search(_ context.Context, _ domain.SreAlertSearchFilters, sort domain.SreAlertSort, offset, limit int) ([]domain.SreAlertListItem, int, error) {
	r.sort, r.offset, r.limit = sort, offset, limit
	return nil, 0, nil
}

func (r *fakeSreAlertRepo) Get(context.Context, string) (*domain.SreAlertDetail, error) {
	return r.detail, nil
}

func (r *fakeSreAlertRepo) Facets(_ context.Context, since time.Time, _ int) (domain.SreAlertFacets, error) {
	r.facetsSince = since
	return domain.SreAlertFacets{}, nil
}

func TestSreAlertSearch_DefaultsToNewestFirstAndCapsThePage(t *testing.T) {
	repo := &fakeSreAlertRepo{}
	svc := NewSreAlertService(repo)

	resp, err := svc.Search(context.Background(), domain.SearchSreAlertsRequest{})
	if err != nil || repo.sort != (domain.SreAlertSort{Field: "receivedOn", Order: "desc"}) || repo.limit != 20 || resp.Alerts == nil {
		t.Fatalf("resp %+v, sort %+v limit %d, err %v; want newest first, 20 a page and an empty list, not null", resp, repo.sort, repo.limit, err)
	}
	_, err = svc.Search(context.Background(), domain.SearchSreAlertsRequest{
		SortBy: &domain.SreAlertSort{Field: "severity", Order: "ASC"}, Pagination: &domain.Pagination{Offset: 40, Limit: 500},
	})
	if err != nil || repo.sort != (domain.SreAlertSort{Field: "severity", Order: "asc"}) || repo.offset != 40 || repo.limit != 100 {
		t.Fatalf("sort %+v offset %d limit %d, err %v; want severity asc from 40, capped at 100", repo.sort, repo.offset, repo.limit, err)
	}
}

func TestSreAlertSearch_RejectsBadFilters(t *testing.T) {
	from := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	for name, req := range map[string]domain.SearchSreAlertsRequest{
		"incident":    {Filters: domain.SreAlertSearchFilters{Incident: "maybe"}},
		"tier":        {Filters: domain.SreAlertSearchFilters{Tiers: []domain.AlertTier{"high"}}},
		"date range":  {Filters: domain.SreAlertSearchFilters{ReceivedFrom: &from, ReceivedTo: &from}},
		"sort field":  {SortBy: &domain.SreAlertSort{Field: "raw_body"}},
		"sort order":  {SortBy: &domain.SreAlertSort{Order: "sideways"}},
		"pagination":  {Pagination: &domain.Pagination{Offset: -1}},
		"incident id": {Filters: domain.SreAlertSearchFilters{IncidentID: "INC0001"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewSreAlertService(&fakeSreAlertRepo{}).Search(context.Background(), req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want a validation error", err)
			}
		})
	}
}

func TestSreAlertGet_NotFound(t *testing.T) {
	_, err := NewSreAlertService(&fakeSreAlertRepo{}).Get(context.Background(), "ALT000000001")
	var nfe *apierror.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("err = %v, want not found", err)
	}
}

func TestSreAlertFacets_LookBackThirtyDays(t *testing.T) {
	repo := &fakeSreAlertRepo{}
	svc := &sreAlertService{repo: repo, now: func() time.Time { return time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC) }}
	if _, err := svc.Facets(context.Background()); err != nil || !repo.facetsSince.Equal(time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("since = %s, err %v; want 30 days back", repo.facetsSince, err)
	}
}

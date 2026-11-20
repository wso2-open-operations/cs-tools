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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type capturePIRepo struct {
	fakePIRepo
	got domain.SearchProductIssuesFilters
}

func (c *capturePIRepo) Search(_ context.Context, f domain.SearchProductIssuesFilters, _ domain.Pagination) ([]domain.ProductIssueSummary, int, error) {
	c.got = f
	return []domain.ProductIssueSummary{{ID: "x"}}, 1, nil
}

func TestProductIssueSearch_MineAndSort(t *testing.T) {
	repo := &capturePIRepo{}
	svc := NewProductIssueService(&fakePWEng{}, repo, internalAccess{})
	var ve *apierror.ValidationError
	search := func(ctx context.Context, f domain.SearchProductIssuesFilters) (domain.SearchProductIssuesResponse, error) {
		return svc.Search(ctx, domain.SearchProductIssuesRequest{Filters: f})
	}
	anon := auth.WithIdentity(context.Background(), auth.Identity{Validated: true})
	if _, err := search(anon, domain.SearchProductIssuesFilters{Mine: true}); !errors.As(err, &ve) {
		t.Errorf("mine without user: %v", err)
	}
	if _, err := search(pwCtx(), domain.SearchProductIssuesFilters{Sort: "bogus"}); !errors.As(err, &ve) {
		t.Errorf("bad sort: %v", err)
	}
	// MineEmail is never taken from the request body.
	if _, err := search(pwCtx(), domain.SearchProductIssuesFilters{MineEmail: "evil@x.com"}); err != nil || repo.got.MineEmail != "" || repo.got.Sort != "created" {
		t.Errorf("plain: %+v %v", repo.got, err)
	}
	resp, err := search(pwCtx(), domain.SearchProductIssuesFilters{Mine: true, BoardStatuses: []string{"WAITING_ON_CS"}})
	if err != nil || repo.got.MineEmail != pwEmail || repo.got.Sort != "pausedSince" {
		t.Errorf("mine: %+v %v", repo.got, err)
	}
	if len(resp.Items) != 1 || resp.Items[0].SLA.State != "NONE" {
		t.Errorf("standing not applied: %+v", resp.Items)
	}
	if _, err := search(pwCtx(), domain.SearchProductIssuesFilters{Mine: true, Sort: " slaPercentage "}); err != nil || repo.got.Sort != "slaPercentage" {
		t.Errorf("explicit sort: %+v %v", repo.got, err)
	}
}

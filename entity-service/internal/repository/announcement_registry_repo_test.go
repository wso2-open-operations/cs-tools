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

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The repository labels every row it returns an announcement, so it refuses
// to run for a request that is not filtered to announcements only. The guard
// returns before any query, so no database is needed here.
func TestAnnouncementRegistryRepoRefusesANonAnnouncementTypeFilter(t *testing.T) {
	t.Parallel()
	repo := &announcementRegistryRepo{} // nil db: reaching it would panic
	for name, types := range map[string][]string{
		"no type filter":          nil,
		"a different type":        {"case"},
		"announcement and others": {"announcement", "case"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := domain.SearchCasesRequest{Parsed: domain.ParsedCaseFilters{Types: types}}
			if _, err := repo.SearchAnnouncementCases(context.Background(), req, SearchScope{Unrestricted: true}, 10); err == nil {
				t.Fatalf("Types=%v: want an error, got none", types)
			}
		})
	}
}

// The grouped read is for internal callers only. The repository refuses any
// scope that is not Unrestricted on its own, before any query (nil db: reaching
// it would panic), so a caller that skips the service cannot read with a
// customer scope. The zero scope fails closed too.
func TestAnnouncementRegistryRowsRepoRefusesANonInternalScope(t *testing.T) {
	t.Parallel()
	repo := &announcementRegistryRepo{}
	req := domain.SearchCasesRequest{Parsed: domain.ParsedCaseFilters{Types: []string{"announcement"}}}
	for name, scope := range map[string]SearchScope{
		"zero scope":        {},
		"customer scope":    {ProjectIDs: []string{"00000000-0000-0000-0000-000000000000"}, ViewerEmail: "jane.doe@example.com"},
		"internal-but-flag": {HasInternalAccess: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := repo.SearchAnnouncementRegistryRows(context.Background(), req, scope)
			var forbidden *apierror.ForbiddenError
			if !errors.As(err, &forbidden) {
				t.Fatalf("want a ForbiddenError, got %v", err)
			}
		})
	}
}

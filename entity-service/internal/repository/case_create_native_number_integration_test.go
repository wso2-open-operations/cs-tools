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

// A case created natively (plain PostgreSQL) is numbered like a synced one:
// CS + 7 digits from the shared cs_number_seq (migration 0180's
// next_work_item_number) and a WSO2 id "<project key>-<n>" from the project's
// wso2_id_counter (next_wso2_id), for every one of the five case-like types.
// Through the real CaseRepository.CreateCase, as an internal caller and as a
// registered project member. Reuses the case-create RLS fixture; skipped
// without CASE_CREATE_RLS_TEST_DSN (a login that does not bypass row-level
// security).
//
//	CASE_CREATE_RLS_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseCreateNativeNumbers

package repository_test

import (
	"context"
	"regexp"
	"strconv"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

var (
	nativeCaseNumber = regexp.MustCompile(`^CS([0-9]{7})$`)
	nativeWSO2ID     = regexp.MustCompile(`^CCMPROJ-([0-9]+)$`) // the fixture project's key
)

func TestCaseCreateNativeNumbers(t *testing.T) {
	pool := seedCaseCreateMemberFixture(t)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	internal := repository.WithSystemIdentity(context.Background())
	customer := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: ccmCustomer})

	request := func(caseType string) domain.CreateCaseRequest {
		r := ccmRequest(caseType, ccmProjectID)
		if caseType == "engagement" {
			r.EngagementType = domain.EngagementTypeMigration
			r.EngagementPaymentType = domain.EngagementPaymentTypePaid
		}
		return r
	}

	// Every case-like type as an internal caller, then a member's create:
	// they share one CS sequence and one per-project counter.
	creates := []struct {
		caseType string
		ctx      context.Context
	}{
		{"case", internal},
		{"service_request", internal},
		{"engagement", internal},
		{"security_report_analysis", internal},
		{"announcement", internal},
		{"case", customer},
		{"service_request", customer},
	}

	var prevNumber, prevCounter int
	seenIDs := map[string]bool{}
	for i, c := range creates {
		created, err := repo.CreateCase(c.ctx, request(c.caseType))
		if err != nil {
			t.Fatalf("create %d (%s): %v", i, c.caseType, err)
		}

		m := nativeCaseNumber.FindStringSubmatch(created.Number)
		if m == nil {
			t.Fatalf("create %d (%s): number %q is not CS + 7 digits", i, c.caseType, created.Number)
		}
		number, _ := strconv.Atoi(m[1])

		w := nativeWSO2ID.FindStringSubmatch(created.InternalID)
		if w == nil {
			t.Fatalf("create %d (%s): WSO2 id %q is not CCMPROJ-<n>", i, c.caseType, created.InternalID)
		}
		counter, _ := strconv.Atoi(w[1])

		if seenIDs[created.InternalID] {
			t.Errorf("create %d (%s): WSO2 id %s issued twice", i, c.caseType, created.InternalID)
		}
		seenIDs[created.InternalID] = true
		if i > 0 {
			if number <= prevNumber {
				t.Errorf("create %d (%s): number CS%07d does not follow CS%07d", i, c.caseType, number, prevNumber)
			}
			if counter <= prevCounter {
				t.Errorf("create %d (%s): WSO2 id counter %d does not follow %d", i, c.caseType, counter, prevCounter)
			}
		}
		prevNumber, prevCounter = number, counter
	}
}

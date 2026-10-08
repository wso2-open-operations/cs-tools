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

package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The detail read's view of a customer's proposed time (fillCustomerProposal) is best effort: a
// failure of the facts query is logged and the field stays unset, the read does not fail. That is
// why nothing the service does with the PATCH receipt may DEPEND on customerProposal being there
// (changeRequestService.PatchChangeRequest decides what to mirror to the previous system from the caller,
// and its tests feed it exactly this unset field). This pins the premise: the failure is real, it
// is swallowed, and the change request comes back without a proposal although the row has one.
func TestFillCustomerProposal_AFailedReadLeavesTheFieldUnset(t *testing.T) {
	// A pool nobody can reach and a context that is already over: the facts query cannot run, and
	// no database is needed to prove what happens then.
	pool, err := pgxpool.New(context.Background(), "postgres://nobody:none@127.0.0.1:1/none?sslmode=disable")
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	r := &changeRequestRepo{db: NewScoped(pool)}
	ctx, cancel := context.WithCancel(WithCallerIdentity(context.Background(), SearchScope{ViewerEmail: "caller@example.com"}))
	cancel()

	proposed := "2030-03-08T09:00:00Z"
	cr := domain.ChangeRequest{CustomerUpdatedOn: &proposed}
	cr.ID = "00000000-0000-4000-8000-000000000001"
	r.fillCustomerProposal(ctx, &cr)
	if cr.CustomerProposal != nil {
		t.Fatalf("customerProposal = %+v after a failed read, want it unset", cr.CustomerProposal)
	}
	if cr.CustomerUpdatedOn == nil || *cr.CustomerUpdatedOn != proposed {
		t.Fatalf("the failed read changed what the row said: customerUpdatedOn = %v", cr.CustomerUpdatedOn)
	}

	// With no proposal on the row the read is not even attempted.
	bare := domain.ChangeRequest{}
	bare.ID = "00000000-0000-4000-8000-000000000001"
	r.fillCustomerProposal(ctx, &bare)
	if bare.CustomerProposal != nil {
		t.Fatalf("customerProposal = %+v on a change request with no proposed time", bare.CustomerProposal)
	}
}

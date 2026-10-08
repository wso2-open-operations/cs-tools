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
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// outageSNFirstService is the OutageService for
// DATA_SOURCE=postgres-servicenow-dual-write. Every method is the plain
// Postgres service's (pgOutageService, embedded) except CreateOutage, which is
// external-system-FIRST and SYNCHRONOUS, exactly like
// deploymentService.createDeploymentSNFirst and caseService.createCaseSNFirst.
//
// Outage updates and communications stay Postgres-only, as before: this type
// changes how a NEW outage gets its number and id, nothing else.
type outageSNFirstService struct {
	*pgOutageService
	// external is the external-system OutageService (snOutageService in
	// production). Only its CreateOutage is used: it already posts the create
	// and returns the number and id the external system assigned, so no second
	// HTTP client is needed here.
	external OutageService
}

// NewOutageServiceWithSNFirstCreate constructs the OutageService for
// DATA_SOURCE=postgres-servicenow-dual-write: the Postgres service for
// everything, with CreateOutage creating in the external system first and
// storing the number and id it assigns -- see CreateOutage's own doc comment.
func NewOutageServiceWithSNFirstCreate(repo repository.OutageRepository, external OutageService) OutageService {
	return &outageSNFirstService{pgOutageService: &pgOutageService{repo: repo}, external: external}
}

// CreateOutage implements OutageService for the dual-write data source:
// external-system-FIRST and SYNCHRONOUS, for the same reason as
// createDeploymentSNFirst -- the external system is authoritative for the
// outage's number and id (they are what its other records and the people
// working in it refer to), and Postgres has no way to produce a matching pair.
// Creating the Postgres row first and mirroring afterwards would leave a
// permanent orphan if the mirror failed. Calling the external system first,
// and writing to Postgres only once that succeeds, makes that orphan
// impossible. Outage numbers also come from the external system alone on this
// path: outage_number_seq is never consulted, so the two systems cannot hand
// out the same number.
//
// Flow: (1) the request is validated and resolved by the same code the plain
// Postgres service uses (prepareCreate), BEFORE anything is created upstream,
// so a request Postgres would reject never reaches the external system;
// (2) the external create runs; (3) its number and id are stored in Postgres,
// in the one transaction that also writes the seeded journal entries and
// affected items, and the Postgres read-back is returned, so the response is
// the same shape as every other mode.
//
// The external call is made exactly once, with no internal retry: if the
// external create actually succeeds but the HTTP response back to
// entity-service is lost (timeout, network blip), a retry sends a second
// CREATE, producing a duplicate outage there. That is worse than for most
// records: an outage on an in-scope service is not inert, the next cloud
// status sweep posts a real outage_begin for it to a shared dashboard.
// Entity-service cannot tell a lost response from a real failure, so retry
// policy belongs to the caller. If the external call fails, the error is
// returned and NOTHING is written to Postgres, by construction (repo.Create is
// never called). No orphan gets created.
//
// KNOWN GAP: if the external create succeeds and the Postgres insert then
// fails, the outage exists externally but not in Postgres. That is real drift
// needing operator attention, not a safely-rejected request. It is returned as
// an error and logged with the outage's number and id for reconciliation; no
// failure-record table is written, matching the other create-first paths. The
// caller must NOT blindly retry (it would create a second external outage).
func (s *outageSNFirstService) CreateOutage(ctx context.Context, req domain.CreateOutageRequest) (domain.CreateOutageResponse, error) {
	in, err := s.prepareCreate(ctx, req)
	if err != nil {
		return domain.CreateOutageResponse{}, err
	}

	created, err := s.external.CreateOutage(ctx, req)
	if err != nil {
		return domain.CreateOutageResponse{}, err
	}

	// Postgres cannot store an outage without a number and a uuid-shaped id.
	// The create has already happened upstream by now, so a reply without them
	// is a partial creation needing reconciliation, not a rejected client
	// request: a downstream error, logged, with nothing written to Postgres.
	id, number := created.Outage.ID, created.Outage.Number
	if id == "" || number == "" || !validate.IsUUID(id) {
		slog.ErrorContext(ctx, "create outage: external outage created but the create reply carried no usable id/number; nothing written to Postgres, needs reconciliation",
			"outageId", id, "number", number)
		return domain.CreateOutageResponse{}, &apierror.DownstreamError{Msg: "The outage was created but its number and id were not returned by the upstream service, so it could not be stored. It needs to be reconciled."}
	}

	in.ID, in.Number = id, number
	out, err := s.repo.Create(ctx, in)
	if err != nil {
		slog.ErrorContext(ctx, "create outage: external outage created but the Postgres insert failed",
			"outageId", id, "number", number, "error", err)
		return domain.CreateOutageResponse{}, err
	}
	return domain.CreateOutageResponse{Message: "Outage created successfully.", Outage: out}, nil
}

package service

import (
	"context"
	"log"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// IngestService lands registrations delivered by the queue poller.
//
// The poller itself stays in the BFF — it is the thing that talks to the queue
// service, and that is a PLG concern. What lives here is the transaction: a
// registration touches plg_person, plg_organization, plg_org_platform and the
// attribute overflow table, and those four writes have to be atomic. They
// cannot be, across HTTP, unless one endpoint wraps them — which is what this
// is (contract I1).
type IngestService interface {
	Register(ctx context.Context, req domain.IngestRegistrationsRequest) (domain.IngestBatchResult, error)
	RecordFailure(ctx context.Context, req domain.RecordIngestFailureRequest) (domain.RecordIngestFailureResult, error)
}

type ingestService struct {
	repo     repository.IngestRepository
	failures repository.FailureRepository
}

// NewIngestService wires an IngestService over its repositories.
func NewIngestService(repo repository.IngestRepository, failures repository.FailureRepository) IngestService {
	return &ingestService{repo: repo, failures: failures}
}

// Register lands a batch, each registration in its own transaction.
//
// One bad record does not reject the rest, which is why the batch is handled
// here rather than by the caller issuing one request per record. Idempotency is
// structural: plg_organization's UNIQUE
// organization_name is what makes a redelivered event a no-op.
func (s *ingestService) Register(ctx context.Context, req domain.IngestRegistrationsRequest) (domain.IngestBatchResult, error) {
	// Accepted and Failed are counted as the loop goes, not derived afterwards.
	// They are part of this endpoint's JSON contract and the BFF's own ingest
	// service populates them too (internal/plg/service/ingest_service.go), so a
	// batch answering {"accepted":0,"failed":0} with results in it would be
	// indistinguishable from a batch that did nothing — and silent to anything
	// alerting on failed > 0.
	out := domain.IngestBatchResult{Results: []domain.IngestResult{}}

	for i := range req.Registrations {
		reg := req.Registrations[i]
		res, err := s.repo.Register(ctx, reg.Registration, reg.Attributes)
		if err != nil {
			// Logged and recorded, never fatal to the batch. The event is already
			// gone from the queue — consuming deletes — so a registration that
			// cannot land has nowhere else to exist.
			log.Printf("plg ingest: %q failed: %v", reg.OrganizationName, err)
			out.Results = append(out.Results, domain.IngestResult{
				Status:           "FAILED",
				OrganizationName: reg.OrganizationName,
				Message:          err.Error(),
			})
			out.Failed++
			continue
		}
		out.Results = append(out.Results, *res)
		out.Accepted++
	}
	return out, nil
}

func (s *ingestService) RecordFailure(ctx context.Context, req domain.RecordIngestFailureRequest) (domain.RecordIngestFailureResult, error) {
	if req.Failure == "" {
		return domain.RecordIngestFailureResult{}, invalidField("failure must not be empty")
	}
	f := repository.IngestFailure{Payload: req.Payload, Failure: req.Failure}
	if req.EventID != nil {
		f.EventID = *req.EventID
	}
	if req.EventType != nil {
		f.EventType = *req.EventType
	}
	if req.ReceivedAt != nil {
		// The repository takes the timestamp as a string and lets Postgres cast
		// it, so a source that sends an unparseable one fails at the column
		// rather than silently landing as "now".
		stamp := req.ReceivedAt.Format(time.RFC3339)
		f.ReceivedAt = &stamp
	}
	err := s.failures.Record(ctx, f)
	if err != nil {
		return domain.RecordIngestFailureResult{}, err
	}
	return domain.RecordIngestFailureResult{}, nil
}

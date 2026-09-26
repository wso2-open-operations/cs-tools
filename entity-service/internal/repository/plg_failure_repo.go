package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FailureRepository records events consumed from the queue that could not be
// processed.
//
// The queue deletes on consume, so this is the only place a failed event
// survives. Writing here is the last step that can keep a registration
// recoverable.
type FailureRepository interface {
	Record(ctx context.Context, f IngestFailure) error
	OpenCount(ctx context.Context) (int, error)
}

// IngestFailure is one unprocessable event.
type IngestFailure struct {
	EventID    string
	EventType  string
	ReceivedAt *string
	Payload    json.RawMessage
	Failure    string
}

type failureRepository struct{ db *pgxpool.Pool }

// NewFailureRepository builds a FailureRepository over the given pool.
func NewFailureRepository(db *pgxpool.Pool) FailureRepository {
	return &failureRepository{db: db}
}

func (r *failureRepository) Record(ctx context.Context, f IngestFailure) error {
	const q = `
		INSERT INTO plg_ingest_failure (event_id, event_type, received_at, payload, failure)
		VALUES (NULLIF($1, ''), NULLIF($2, ''), $3::TIMESTAMPTZ, $4::JSONB, $5)`

	// A payload that is not valid JSON still has to be kept, so it is stored as
	// a JSON string rather than rejected by the JSONB cast.
	payload := f.Payload
	if !json.Valid(payload) {
		wrapped, err := json.Marshal(string(payload))
		if err != nil {
			wrapped = []byte(`"<unrepresentable payload>"`)
		}
		payload = wrapped
	}

	if _, err := r.db.Exec(ctx, q, f.EventID, f.EventType, f.ReceivedAt, string(payload), f.Failure); err != nil {
		return fmt.Errorf("record ingest failure: %w", err)
	}
	return nil
}

// OpenCount reports how many failures are still unresolved, for the startup log
// and any later health check.
func (r *failureRepository) OpenCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*)::INT FROM plg_ingest_failure WHERE resolved_on IS NULL`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count ingest failures: %w", err)
	}
	return n, nil
}

package repository

import (
	"context"
	"encoding/json"
	"testing"
)

func TestAnOpenFailureIsNotRecordedTwice(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	clear := func() { _, _ = pool.Exec(context.Background(), `DELETE FROM plg_ingest_failure`) }
	clear()
	t.Cleanup(clear)

	repo := NewFailureRepository(pool)
	f := IngestFailure{EventID: "moesif-1", Payload: json.RawMessage(`{}`), Failure: "unknown product"}
	for i := 0; i < 2; i++ {
		if err := repo.Record(ctx, f); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
	if n, _ := repo.OpenCount(ctx); n != 1 {
		t.Fatalf("a repeated open event should be recorded once, got %d", n)
	}

	if _, err := pool.Exec(ctx, `UPDATE plg_ingest_failure SET resolved_on = now()`); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if err := repo.Record(ctx, f); err != nil {
		t.Fatalf("record after resolve: %v", err)
	}
	for _, noID := range []IngestFailure{{Payload: json.RawMessage(`{}`), Failure: "a"}, {Payload: json.RawMessage(`{}`), Failure: "b"}} {
		if err := repo.Record(ctx, noID); err != nil {
			t.Fatalf("record without an id: %v", err)
		}
	}
	if n, _ := repo.OpenCount(ctx); n != 3 {
		t.Fatalf("a resolved event recurs, and events without an id are always kept: want 3, got %d", n)
	}
}

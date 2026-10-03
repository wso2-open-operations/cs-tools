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

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Against a database built from migrations (0184 included): the outage API
// stores all three channels, counts them per channel, and the public status
// page sees only the external one.
//
// Run with PG_OUTAGE_TEST_DSN=postgres://... go test -run OutageCommunicationChannel ./internal/repository/
func TestOutageCommunicationChannelLive(t *testing.T) {
	dsn := os.Getenv("PG_OUTAGE_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_OUTAGE_TEST_DSN not set; this test needs a real database")
	}
	ctx := repository.WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	const outageID = "58888888-0000-0000-0000-000000000001"
	cleanup := func() {
		// outage_communication rows go with the outage (ON DELETE CASCADE).
		if _, err := pool.Exec(ctx, `DELETE FROM outage WHERE id = $1`, outageID); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(ctx, `
		INSERT INTO outage (id, created_on, updated_on, created_by, updated_by, number, name)
		VALUES ($1, NOW(), NOW(), 'test', 'test', 'OUT-CHANNEL-TEST', 'channel test')`, outageID); err != nil {
		t.Fatalf("seed outage: %v", err)
	}

	repo := repository.NewOutageRepository(repository.NewScoped(pool))
	for _, c := range []struct {
		ch   domain.OutageCommunicationChannel
		body string
	}{
		{domain.OutageCommunicationChannelExternal, "public update"},
		{domain.OutageCommunicationChannelInternal, "internal note"},
		{domain.OutageCommunicationChannelAdditional, "additional note"},
	} {
		if _, err := repo.AddCommunication(ctx, outageID, c.ch, c.body, "test"); err != nil {
			t.Fatalf("add %s communication: %v", c.ch, err)
		}
	}

	detail, err := repo.GetByID(ctx, outageID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got := detail.CommunicationCounts; got.External != 1 || got.Internal != 1 || got.Additional != 1 {
		t.Errorf("per-channel counts = %+v, want 1/1/1", got)
	}

	// The public status page.
	comments, err := repository.NewCloudStatusDashboardRepository(pool).OutageComments(ctx, outageID)
	if err != nil {
		t.Fatalf("OutageComments: %v", err)
	}
	if len(comments) != 1 || comments[0].Comment != "public update" {
		t.Errorf("public comments = %+v, want only the external one", comments)
	}

	// A channel outside the three is refused by THAT constraint. Any error is
	// not enough: an insert failing for an unrelated reason (a new NOT NULL
	// column, say) would pass this with the constraint gone.
	_, err = pool.Exec(ctx, `INSERT INTO outage_communication (outage_id, channel, comment) VALUES ($1, 'private', 'x')`, outageID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "outage_communication_channel_chk" {
		t.Errorf("unknown channel: got err %v, want an outage_communication_channel_chk violation (23514)", err)
	}
}

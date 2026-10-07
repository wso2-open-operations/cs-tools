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
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Linking a problem to a change request (ServiceNow's problem.rfc) against a
// real database: link, the link read back by GetProblem, unlink with "", and
// an unknown change request refused. Run with ENTITY_TEST_DATABASE_URL; the
// rows it seeds are deleted afterwards.
const (
	pcProblemID = "47777777-0000-0000-0000-0000000000c1"
	pcChangeID  = "47777777-0000-0000-0000-0000000000c2"
)

func pcSetup(t *testing.T) (context.Context, *pgxpool.Pool, ProblemRepository) {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	ctx := WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	cleanup := func() {
		for _, id := range []string{pcProblemID, pcChangeID} {
			if _, err := pool.Exec(context.Background(), `DELETE FROM work_item WHERE id = $1`, id); err != nil {
				t.Errorf("CLEANUP FAILED, delete work_item %s by hand: %v", id, err)
			}
		}
	}
	cleanup()
	t.Cleanup(func() { cleanup(); pool.Close() })
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		  VALUES ($1, NOW(), NOW(), 't', 't', 'CHG-PC-0001', 'Roll back the gateway', 'CHANGE_REQUEST')`, []any{pcChangeID}},
		{`INSERT INTO change_request (id) VALUES ($1)`, []any{pcChangeID}},
		{`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		  VALUES ($1, NOW(), NOW(), 't', 't', 'PRB-PC-0001', 'Gateway 502s', 'PROBLEM')`, []any{pcProblemID}},
		{`INSERT INTO problem (id, state, is_active, opened_on) VALUES ($1, 'NEW', TRUE, NOW())`, []any{pcProblemID}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return ctx, pool, NewProblemRepository(NewScoped(pool))
}

func TestProblemChangeRequestIntegration(t *testing.T) {
	ctx, _, repo := pcSetup(t)
	link := func(id string) error {
		_, err := repo.UpdateProblemFields(ctx, domain.UpdateProblemRequest{ID: pcProblemID, ChangeRequestID: &id}, "t@example.com")
		return err
	}
	linked := func() *domain.CaseNumberRef {
		d, err := repo.GetProblem(ctx, pcProblemID)
		if err != nil {
			t.Fatalf("GetProblem: %v", err)
		}
		return d.LinkedChangeRequest
	}

	if err := link(pcChangeID); err != nil {
		t.Fatalf("link: %v", err)
	}
	if cr := linked(); cr == nil || cr.ID != pcChangeID || cr.Number != "CHG-PC-0001" {
		t.Errorf("after link, linkedChangeRequest = %+v, want CHG-PC-0001", cr)
	}

	// An unknown change request is refused, and the existing link stays.
	err := link("47777777-0000-0000-0000-0000000000ff")
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "changeRequestId") {
		t.Errorf("unknown change request: err = %v, want a changeRequestId ValidationError", err)
	}
	if cr := linked(); cr == nil || cr.ID != pcChangeID {
		t.Errorf("a refused link changed the existing one: %+v", cr)
	}

	if err := link(""); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if cr := linked(); cr != nil {
		t.Errorf("after unlink, linkedChangeRequest = %+v, want none", cr)
	}
}

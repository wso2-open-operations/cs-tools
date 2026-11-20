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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.
// The audit actor: the person an audit row names, kept when a request's identity
// is escalated by WithSystemIdentity. The last test needs a real Postgres, and is
// skipped without ENTITY_TEST_DATABASE_URL.

package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestWithSystemIdentity_KeepsThePersonForTheAuditLog(t *testing.T) {
	person := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: "sam@example.com"})

	scope, ok := repository.CallerIdentityFromContext(repository.WithSystemIdentity(person))
	if !ok {
		t.Fatal("no identity on the escalated context")
	}
	if !scope.Unrestricted || scope.ViewerEmail != "" {
		t.Errorf("escalated scope = %+v, want unrestricted with no viewer", scope)
	}
	if scope.AuditActor != "sam@example.com" {
		t.Errorf("AuditActor = %q, want the person who was escalated", scope.AuditActor)
	}
}

func TestWithSystemIdentity_EscalatingTwiceKeepsThePerson(t *testing.T) {
	person := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: "sam@example.com"})

	scope, _ := repository.CallerIdentityFromContext(repository.WithSystemIdentity(repository.WithSystemIdentity(person)))

	if scope.AuditActor != "sam@example.com" {
		t.Errorf("AuditActor = %q after two escalations, want the person", scope.AuditActor)
	}
}

func TestWithSystemIdentity_WithoutAPersonHasNoActor(t *testing.T) {
	scope, _ := repository.CallerIdentityFromContext(repository.WithSystemIdentity(context.Background()))

	if scope.AuditActor != "" {
		t.Errorf("AuditActor = %q, want none: a background job has no person", scope.AuditActor)
	}
}

func TestScoped_EscalatedIdentitySetsTheAuditActorSetting(t *testing.T) {
	url := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	db := repository.NewScoped(pool)

	person := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: "sam@example.com"})
	var viewer, actor string
	err = db.QueryRow(repository.WithSystemIdentity(person),
		`SELECT current_setting('app.viewer_email', true), current_setting('app.audit_actor', true)`).Scan(&viewer, &actor)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if viewer != "" || actor != "sam@example.com" {
		t.Errorf("viewer = %q, actor = %q; want no viewer and the person as the actor", viewer, actor)
	}

	// A scope that was never escalated: the actor is the viewer. Internal staff,
	// because a customer scope also reads project_contact, which a bare test
	// database does not have.
	staff := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true, ViewerEmail: "sam@example.com"})
	err = db.QueryRow(staff, `SELECT current_setting('app.audit_actor', true)`).Scan(&actor)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if actor != "sam@example.com" {
		t.Errorf("actor = %q for an ordinary scope, want the viewer", actor)
	}
}

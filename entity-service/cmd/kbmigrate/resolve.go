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

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// userResolver resolves a ServiceNow sys_user sys_id to this platform's
// entity-service users.id (TEXT, keyed by email). See resolveUser's doc
// comment for the exact join path. A fake implementation backs the pure
// unit tests in resolve_test.go; realUserResolver is the live, two-database
// implementation used by main.go.
type userResolver interface {
	// ResolveUser returns the entity-service users.id for the given SN
	// sys_user sys_id, or an error describing exactly which step of the
	// resolution path failed (never a bare "not found").
	ResolveUser(ctx context.Context, snUserSysID string) (string, error)
}

// realUserResolver implements userResolver via the documented two-hop path:
//  1. snUserSysID -> sysIDToUUID -> look up that UUID's row in
//     csm-sync-service's OWN, separate Postgres database ("user" table,
//     singular -- read-only, never written to here) to get its email.
//  2. That email -> look up entity-service's own "users" table (also TEXT,
//     but keyed by email, NOT derived from any sys_id) to get the final id.
//
// entity-service.users.id is intentionally NOT derived from the SN sys_id
// the way cases/products UUIDs are -- see resolveCase for that other path.
//
// Results are cached per sys_id for the lifetime of one run: the same small
// set of authors/reviewers/managers recurs across many articles.
type realUserResolver struct {
	csmSyncPool *pgxpool.Pool // read-only: csm-sync-service's database
	entityPool  *pgxpool.Pool // this service's own database

	cache map[string]userResolution
}

type userResolution struct {
	id  string
	err error
}

func newRealUserResolver(csmSyncPool, entityPool *pgxpool.Pool) *realUserResolver {
	return &realUserResolver{
		csmSyncPool: csmSyncPool,
		entityPool:  entityPool,
		cache:       make(map[string]userResolution),
	}
}

// ResolveUser implements userResolver.
func (r *realUserResolver) ResolveUser(ctx context.Context, snUserSysID string) (string, error) {
	if cached, ok := r.cache[snUserSysID]; ok {
		return cached.id, cached.err
	}
	id, err := r.resolveUncached(ctx, snUserSysID)
	r.cache[snUserSysID] = userResolution{id: id, err: err}
	return id, err
}

func (r *realUserResolver) resolveUncached(ctx context.Context, snUserSysID string) (string, error) {
	if snUserSysID == "" {
		return "", fmt.Errorf("empty sys_user sys_id")
	}

	derivedUUID := sysIDToUUID(snUserSysID)
	if !isCanonicalUUID(derivedUUID) {
		return "", fmt.Errorf("sys_user sys_id %q did not convert to a canonical UUID (got %q)", snUserSysID, derivedUUID)
	}

	var email string
	err := r.csmSyncPool.QueryRow(ctx, `SELECT email FROM "user" WHERE id = $1`, derivedUUID).Scan(&email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("sys_user %q (uuid %q) not found in csm-sync-service's user table", snUserSysID, derivedUUID)
		}
		return "", fmt.Errorf("query csm-sync-service user table for sys_user %q: %w", snUserSysID, err)
	}

	var entityUserID string
	err = r.entityPool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&entityUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("email %q (resolved from sys_user %q) not found in entity-service users table", email, snUserSysID)
		}
		return "", fmt.Errorf("query entity-service users table for email %q: %w", email, err)
	}
	return entityUserID, nil
}

// caseResolver resolves a ServiceNow sn_customerservice_case sys_id to this
// platform's cases.id (UUID). Unlike users, cases.id IS derived from the SN
// sys_id via the same deterministic sysIDToUUID transform used everywhere
// else in this codebase -- no cross-database lookup is needed, only an
// optional existence check.
type caseResolver interface {
	// ResolveCase returns the derived case UUID and true if it exists in
	// entity-service's own cases table, or ("", false, nil) if the derived
	// UUID does not correspond to a real row -- source_case_id is nullable,
	// so a caller should set it to NULL rather than fail the whole article
	// in that situation. A non-nil error indicates the existence check
	// itself could not be performed (a DB problem), not "does not exist".
	ResolveCase(ctx context.Context, snCaseSysID string) (caseUUID string, exists bool, err error)
}

type realCaseResolver struct {
	entityPool *pgxpool.Pool
}

func newRealCaseResolver(entityPool *pgxpool.Pool) *realCaseResolver {
	return &realCaseResolver{entityPool: entityPool}
}

// ResolveCase implements caseResolver.
func (r *realCaseResolver) ResolveCase(ctx context.Context, snCaseSysID string) (string, bool, error) {
	if snCaseSysID == "" {
		return "", false, nil
	}
	derivedUUID := sysIDToUUID(snCaseSysID)
	if !isCanonicalUUID(derivedUUID) {
		// Malformed sys_id: cannot possibly match a real case row. Treated
		// the same as "does not exist" (source_case_id is optional), not a
		// hard error -- an existence check, not a syntax check.
		return "", false, nil
	}

	var exists bool
	err := r.entityPool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cases WHERE id = $1)`, derivedUUID).Scan(&exists)
	if err != nil {
		return "", false, fmt.Errorf("check case existence for %q: %w", derivedUUID, err)
	}
	return derivedUUID, exists, nil
}

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

// Package repository: this file replaces the original single kb_managers
// table with two separate ones the real schema uses -- individual users
// (knowledge_base_manager_user) and groups (knowledge_base_manager_group).
// created_by is NOT NULL on both real tables (unlike the original
// kb_managers, which had no such column) -- both Create methods take it as
// an explicit parameter rather than folding it into the request struct,
// so the caller (handler layer, reading the authenticated user's identity)
// supplies it rather than a client being able to set it arbitrarily.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// KBManagerUserRepository defines the persistence operations for the
// knowledge_base_manager_user table -- individual users granted manager
// access to a knowledge base.
type KBManagerUserRepository interface {
	// SearchKBManagerUsers returns rows matching the given optional filters.
	SearchKBManagerUsers(ctx context.Context, req domain.SearchKBManagerUsersRequest) ([]domain.KBManagerUser, error)
	// CreateKBManagerUser grants a user manager access to a knowledge base.
	// A ValidationError is returned if this pair already exists (UNIQUE
	// constraint) or references a knowledge base that does not exist.
	CreateKBManagerUser(ctx context.Context, req domain.CreateKBManagerUserRequest, createdBy string) (domain.KBManagerUser, error)
	// DeleteKBManagerUser revokes a user's manager access to a knowledge base.
	DeleteKBManagerUser(ctx context.Context, knowledgeBaseID, userID string) error
}

type kbManagerUserRepo struct {
	db *pgxpool.Pool
}

// NewKBManagerUserRepository constructs a KBManagerUserRepository backed by the given connection pool.
func NewKBManagerUserRepository(db *pgxpool.Pool) KBManagerUserRepository {
	return &kbManagerUserRepo{db: db}
}

// SearchKBManagerUsers implements KBManagerUserRepository.
func (r *kbManagerUserRepo) SearchKBManagerUsers(ctx context.Context, req domain.SearchKBManagerUsersRequest) ([]domain.KBManagerUser, error) {
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if req.KnowledgeBaseID != "" {
		where += fmt.Sprintf(" AND knowledge_base_id = $%d", argIdx)
		args = append(args, req.KnowledgeBaseID)
		argIdx++
	}
	if req.UserID != "" {
		where += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, req.UserID)
		argIdx++
	}

	query := fmt.Sprintf(
		`SELECT id, knowledge_base_id, user_id, created_on FROM knowledge_base_manager_user %s ORDER BY created_on`,
		where,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search kb manager users: %w", err)
	}
	defer rows.Close()

	managers := make([]domain.KBManagerUser, 0)
	for rows.Next() {
		var m domain.KBManagerUser
		if err := rows.Scan(&m.ID, &m.KnowledgeBaseID, &m.UserID, &m.CreatedOn); err != nil {
			return nil, fmt.Errorf("scan kb manager user: %w", err)
		}
		managers = append(managers, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kb manager users: %w", err)
	}
	return managers, nil
}

// CreateKBManagerUser implements KBManagerUserRepository.
func (r *kbManagerUserRepo) CreateKBManagerUser(ctx context.Context, req domain.CreateKBManagerUserRequest, createdBy string) (domain.KBManagerUser, error) {
	const query = `
		INSERT INTO knowledge_base_manager_user (id, knowledge_base_id, user_id, created_on, created_by)
		VALUES (gen_random_uuid(), $1, $2, NOW(), $3)
		RETURNING id, knowledge_base_id, user_id, created_on`

	var m domain.KBManagerUser
	err := r.db.QueryRow(ctx, query, req.KnowledgeBaseID, req.UserID, createdBy).Scan(
		&m.ID, &m.KnowledgeBaseID, &m.UserID, &m.CreatedOn,
	)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503":
				return domain.KBManagerUser{}, &apierror.ValidationError{Msg: "knowledge base does not exist: " + pgErr.Detail}
			case "23505":
				return domain.KBManagerUser{}, &apierror.ValidationError{Msg: "this user is already a manager for this knowledge base"}
			}
		}
		return domain.KBManagerUser{}, fmt.Errorf("create kb manager user: %w", err)
	}
	return m, nil
}

// DeleteKBManagerUser implements KBManagerUserRepository.
func (r *kbManagerUserRepo) DeleteKBManagerUser(ctx context.Context, knowledgeBaseID, userID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM knowledge_base_manager_user WHERE knowledge_base_id = $1 AND user_id = $2`,
		knowledgeBaseID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete kb manager user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.ValidationError{Msg: "this user is not currently a manager for this knowledge base"}
	}
	return nil
}

// KBManagerGroupRepository defines the persistence operations for the
// knowledge_base_manager_group table -- every member of a group granted
// manager access to a knowledge base.
type KBManagerGroupRepository interface {
	// SearchKBManagerGroups returns rows matching the given optional filters.
	SearchKBManagerGroups(ctx context.Context, req domain.SearchKBManagerGroupsRequest) ([]domain.KBManagerGroup, error)
	// CreateKBManagerGroup grants every member of a group manager access to
	// a knowledge base. A ValidationError is returned if this pair already
	// exists (UNIQUE constraint) or references a knowledge base that does
	// not exist. group_id has no FK to check against -- see the domain
	// type's own doc comment for why.
	CreateKBManagerGroup(ctx context.Context, req domain.CreateKBManagerGroupRequest, createdBy string) (domain.KBManagerGroup, error)
	// DeleteKBManagerGroup revokes a group's manager access to a knowledge base.
	DeleteKBManagerGroup(ctx context.Context, knowledgeBaseID, groupID string) error
}

type kbManagerGroupRepo struct {
	db *pgxpool.Pool
}

// NewKBManagerGroupRepository constructs a KBManagerGroupRepository backed by the given connection pool.
func NewKBManagerGroupRepository(db *pgxpool.Pool) KBManagerGroupRepository {
	return &kbManagerGroupRepo{db: db}
}

// SearchKBManagerGroups implements KBManagerGroupRepository.
func (r *kbManagerGroupRepo) SearchKBManagerGroups(ctx context.Context, req domain.SearchKBManagerGroupsRequest) ([]domain.KBManagerGroup, error) {
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if req.KnowledgeBaseID != "" {
		where += fmt.Sprintf(" AND knowledge_base_id = $%d", argIdx)
		args = append(args, req.KnowledgeBaseID)
		argIdx++
	}
	if req.GroupID != "" {
		where += fmt.Sprintf(" AND group_id = $%d", argIdx)
		args = append(args, req.GroupID)
		argIdx++
	}

	query := fmt.Sprintf(
		`SELECT id, knowledge_base_id, group_id, group_name, created_on FROM knowledge_base_manager_group %s ORDER BY created_on`,
		where,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search kb manager groups: %w", err)
	}
	defer rows.Close()

	managers := make([]domain.KBManagerGroup, 0)
	for rows.Next() {
		var m domain.KBManagerGroup
		if err := rows.Scan(&m.ID, &m.KnowledgeBaseID, &m.GroupID, &m.GroupName, &m.CreatedOn); err != nil {
			return nil, fmt.Errorf("scan kb manager group: %w", err)
		}
		managers = append(managers, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kb manager groups: %w", err)
	}
	return managers, nil
}

// CreateKBManagerGroup implements KBManagerGroupRepository.
func (r *kbManagerGroupRepo) CreateKBManagerGroup(ctx context.Context, req domain.CreateKBManagerGroupRequest, createdBy string) (domain.KBManagerGroup, error) {
	const query = `
		INSERT INTO knowledge_base_manager_group (id, knowledge_base_id, group_id, group_name, created_on, created_by)
		VALUES (gen_random_uuid(), $1, $2, $3, NOW(), $4)
		RETURNING id, knowledge_base_id, group_id, group_name, created_on`

	var m domain.KBManagerGroup
	err := r.db.QueryRow(ctx, query, req.KnowledgeBaseID, req.GroupID, req.GroupName, createdBy).Scan(
		&m.ID, &m.KnowledgeBaseID, &m.GroupID, &m.GroupName, &m.CreatedOn,
	)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503":
				return domain.KBManagerGroup{}, &apierror.ValidationError{Msg: "knowledge base does not exist: " + pgErr.Detail}
			case "23505":
				return domain.KBManagerGroup{}, &apierror.ValidationError{Msg: "this group is already a manager for this knowledge base"}
			}
		}
		return domain.KBManagerGroup{}, fmt.Errorf("create kb manager group: %w", err)
	}
	return m, nil
}

// DeleteKBManagerGroup implements KBManagerGroupRepository.
func (r *kbManagerGroupRepo) DeleteKBManagerGroup(ctx context.Context, knowledgeBaseID, groupID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM knowledge_base_manager_group WHERE knowledge_base_id = $1 AND group_id = $2`,
		knowledgeBaseID, groupID,
	)
	if err != nil {
		return fmt.Errorf("delete kb manager group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.ValidationError{Msg: "this group is not currently a manager for this knowledge base"}
	}
	return nil
}

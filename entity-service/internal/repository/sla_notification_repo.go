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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SLANotificationRuleRepository is sla_notification_rule (migration 0249).
// csm-notification-service reads these to decide which action(s) to run
// against an sla.threshold_crossed event (R46-R51); this repository only
// stores the admin's own configuration.
type SLANotificationRuleRepository interface {
	List(ctx context.Context) ([]domain.SLANotificationRule, error)
	Get(ctx context.Context, id string) (domain.SLANotificationRule, error)
	Create(ctx context.Context, req domain.UpsertSLANotificationRuleRequest, actorEmail string) (domain.SLANotificationRule, error)
	Update(ctx context.Context, id string, req domain.UpsertSLANotificationRuleRequest, actorEmail string) (domain.SLANotificationRule, error)
}

type slaNotificationRuleRepository struct{ db *pgxpool.Pool }

// NewSLANotificationRuleRepository constructs an SLANotificationRuleRepository.
func NewSLANotificationRuleRepository(db *pgxpool.Pool) SLANotificationRuleRepository {
	return &slaNotificationRuleRepository{db: db}
}

const slaNotificationRuleSelect = `
	SELECT id::text, definition_id::text, threshold_percentage, channel, recipients, webhook_url,
	       is_active, created_on, updated_on
	  FROM sla_notification_rule`

func scanSLANotificationRule(row pgx.Row) (domain.SLANotificationRule, error) {
	var n domain.SLANotificationRule
	var definitionID *string
	var recipientsRaw []byte
	if err := row.Scan(&n.ID, &definitionID, &n.ThresholdPercentage, &n.Channel, &recipientsRaw,
		&n.WebhookURL, &n.IsActive, &n.CreatedOn, &n.UpdatedOn); err != nil {
		return domain.SLANotificationRule{}, err
	}
	n.DefinitionID = definitionID
	if err := json.Unmarshal(recipientsRaw, &n.Recipients); err != nil {
		return domain.SLANotificationRule{}, fmt.Errorf("decode sla_notification_rule.recipients: %w", err)
	}
	return n, nil
}

func (r *slaNotificationRuleRepository) List(ctx context.Context) ([]domain.SLANotificationRule, error) {
	rows, err := r.db.Query(ctx, slaNotificationRuleSelect+` ORDER BY created_on DESC`)
	if err != nil {
		return nil, fmt.Errorf("list sla notification rules: %w", err)
	}
	defer rows.Close()
	out := []domain.SLANotificationRule{}
	for rows.Next() {
		n, err := scanSLANotificationRule(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sla notification rule: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *slaNotificationRuleRepository) Get(ctx context.Context, id string) (domain.SLANotificationRule, error) {
	n, err := scanSLANotificationRule(r.db.QueryRow(ctx, slaNotificationRuleSelect+` WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SLANotificationRule{}, &apierror.NotFoundError{Msg: "sla notification rule not found"}
	}
	if err != nil {
		return domain.SLANotificationRule{}, fmt.Errorf("get sla notification rule: %w", err)
	}
	return n, nil
}

func (r *slaNotificationRuleRepository) Create(ctx context.Context, req domain.UpsertSLANotificationRuleRequest, actorEmail string) (domain.SLANotificationRule, error) {
	recipientsJSON, err := json.Marshal(req.Recipients)
	if err != nil {
		return domain.SLANotificationRule{}, fmt.Errorf("encode sla notification rule recipients: %w", err)
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	n, err := scanSLANotificationRule(r.db.QueryRow(ctx, `
		INSERT INTO sla_notification_rule (definition_id, threshold_percentage, channel, recipients, webhook_url, is_active, created_by, updated_by)
		VALUES ($1::uuid, $2, $3, $4::jsonb, $5, $6, $7, $7)
		RETURNING id::text, definition_id::text, threshold_percentage, channel, recipients, webhook_url, is_active, created_on, updated_on`,
		req.DefinitionID, req.ThresholdPercentage, req.Channel, recipientsJSON, req.WebhookURL, isActive, actorEmail))
	if err != nil {
		return domain.SLANotificationRule{}, fmt.Errorf("create sla notification rule: %w", err)
	}
	return n, nil
}

// Update leaves is_active untouched when req.IsActive is nil (COALESCE),
// rather than defaulting it to true -- otherwise a PATCH that only edits,
// say, recipients or the threshold (with no isActive field at all) would
// silently re-enable a rule an admin had disabled. The webapp always sends
// isActive, but this API is the actual boundary that has to enforce that,
// not any one caller of it.
func (r *slaNotificationRuleRepository) Update(ctx context.Context, id string, req domain.UpsertSLANotificationRuleRequest, actorEmail string) (domain.SLANotificationRule, error) {
	recipientsJSON, err := json.Marshal(req.Recipients)
	if err != nil {
		return domain.SLANotificationRule{}, fmt.Errorf("encode sla notification rule recipients: %w", err)
	}
	n, err := scanSLANotificationRule(r.db.QueryRow(ctx, `
		UPDATE sla_notification_rule SET
			definition_id = $2::uuid, threshold_percentage = $3, channel = $4, recipients = $5::jsonb,
			webhook_url = $6, is_active = COALESCE($7::boolean, is_active), updated_on = now(), updated_by = $8
		WHERE id = $1::uuid
		RETURNING id::text, definition_id::text, threshold_percentage, channel, recipients, webhook_url, is_active, created_on, updated_on`,
		id, req.DefinitionID, req.ThresholdPercentage, req.Channel, recipientsJSON, req.WebhookURL, req.IsActive, actorEmail))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SLANotificationRule{}, &apierror.NotFoundError{Msg: "sla notification rule not found"}
	}
	if err != nil {
		return domain.SLANotificationRule{}, fmt.Errorf("update sla notification rule: %w", err)
	}
	return n, nil
}

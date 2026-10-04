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

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/risk"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// riskClient abstracts the risk-tracking MySQL operations used by
// CustomerHealthHandler.
type riskClient interface {
	OpenProjectRisk(ctx context.Context, projectSysID, accountSysID, comment, email string) (*risk.ProjectRisk, error)
	CloseProjectRisk(ctx context.Context, riskID int, comment, email string) (*risk.ProjectRisk, error)
	MarkProjectHealthy(ctx context.Context, projectSysID, accountSysID, email string, comment *string) (*risk.HealthStatusRecord, error)
	RevertProjectHealth(ctx context.Context, projectSysID, accountSysID string) (*risk.HealthStatusRecord, error)
	GetAccountHealthStatus(ctx context.Context, accountSysID string) ([]risk.ProjectHealthStatus, error)
	GetAccountHealthSummary(ctx context.Context, accountSysID string) (*risk.HealthSummary, error)
	GetProjectRiskHistory(ctx context.Context, projectSysID string) ([]risk.ProjectRisk, error)
	GetBatchHealthSummaries(ctx context.Context, accountSysIDs []string) (map[string]string, error)
	GetAccountsByHealthStatus(ctx context.Context, healthStatus string) ([]string, error)
	InitProjectHealthRows(ctx context.Context, projectSysIDs []string, accountSysID string) error
	CreateActionItem(ctx context.Context, riskID int, payload risk.CreateActionItemRequest, email string) (*risk.RiskActionItem, error)
	UpdateActionItemStatus(ctx context.Context, actionItemID int, newStatus string, resolutionComment *string, email string) (*risk.RiskActionItem, error)
	UpdateActionItem(ctx context.Context, actionItemID int, payload risk.UpdateActionItemRequest) (*risk.RiskActionItem, error)
	GetActionItemsByRisk(ctx context.Context, riskID int, statusFilter *string) ([]risk.RiskActionItem, error)
	GetActionItemsByAccount(ctx context.Context, accountSysID string, projectSysID, statusFilter *string) ([]risk.RiskActionItem, error)
	CreateActionItemComment(ctx context.Context, actionItemID int, comment, email string) (*risk.ActionItemComment, error)
	GetActionItemComments(ctx context.Context, actionItemID int) ([]risk.ActionItemComment, error)
}

// customerHealthSNClient abstracts the two legacy-data-source
// customer-health lookups used by CustomerHealthHandler (the rest of this
// domain is pure MySQL, via riskClient).
type customerHealthSNClient interface {
	GetCustomerHealthSummary(ctx context.Context, email, phrase, risks *string, region []string, product, abtTeam *string, offset, limit int) (*servicenow.AccountSummaryResponse, error)
	GetCustomerHealthDetail(ctx context.Context, accountID string) (*servicenow.AccountDetail, error)
}

// CustomerHealthHandler handles HTTP requests for SupportPortalLite's
// customer-health/risk-tracking endpoints (/customer-health/*),
// delegating to MySQL (risk-tracking state) and the backing system (account/project
// summary data) as each endpoint requires.
type CustomerHealthHandler struct {
	risk        riskClient
	sn          customerHealthSNClient
	accessGuard *AccessGuard
}

// NewCustomerHealthHandler creates a CustomerHealthHandler. accessGuard
// enforces PermViewerAccess, SupportPortalLite's blanket audience gate — no
// endpoint in this domain has an additional fine-grained permission check
// beyond it (confirmed by reading every customer-health resource function in
// service.bal: none call a group list beyond the global request
// interceptor).
func NewCustomerHealthHandler(riskClient riskClient, sn customerHealthSNClient, accessGuard *AccessGuard) *CustomerHealthHandler {
	return &CustomerHealthHandler{risk: riskClient, sn: sn, accessGuard: accessGuard}
}

// customerHealthSummaryRequest is the payload for POST
// /customer-health/summary — mirrors Ballerina
// types:CustomerHealthSummaryRequest.
type customerHealthSummaryRequest struct {
	Email        *string  `json:"email"`
	Phrase       *string  `json:"phrase"`
	Risks        *string  `json:"risks"`
	Region       []string `json:"region"`
	Product      *string  `json:"product"`
	AbtTeam      *string  `json:"abtTeam"`
	HealthStatus *string  `json:"healthStatus"`
	Offset       int      `json:"offset"`
	Limit        int      `json:"limit"`
}

// accountSummaryResponse is the enriched, portal-shaped response for POST
// /customer-health/summary — mirrors Ballerina types:AccountSummaryResponse
// after the resource function's health-status enrichment pass.
type accountSummaryResponse struct {
	Data       []accountSummary `json:"data"`
	TotalCount int              `json:"totalCount"`
}

type accountSummary struct {
	AccountSysID           string                  `json:"accountSysId"`
	AccountName            *string                 `json:"accountName"`
	HasNoGoLive            servicenow.GoLiveStatus `json:"hasNoGoLive"`
	HasRecentCases         bool                    `json:"hasRecentCases"`
	HasEolProduct          bool                    `json:"hasEolProduct"`
	HasAbandonedMigrations bool                    `json:"hasAbandonedMigrations"`
	HasMigrationDelays     bool                    `json:"hasMigrationDelays"`
	HasRecentEscalations   bool                    `json:"hasRecentEscalations"`
	NoSupportCases6mo      bool                    `json:"noSupportCases6mo"`
	HealthStatus           *string                 `json:"healthStatus"`
}

// GetSummary handles POST /customer-health/summary.
//
// When payload.HealthStatus names a status, this first resolves the
// matching account sys_ids from MySQL, then paginates the backing system in batches
// of 200 to fetch every account matching the other filters, intersects the
// two sets in memory, and paginates the intersection — exactly mirroring
// the Ballerina resource function's approach (the backing system's summary API has
// no sys_id-list filter, so the intersection can't be pushed down to it).
// Without a health-status filter, it fetches one page from the backing system
// directly and enriches it with health-status values from MySQL.
func (h *CustomerHealthHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}
	var payload customerHealthSummaryRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if payload.Limit <= 0 {
		payload.Limit = 10
	}

	healthStatus := ""
	if payload.HealthStatus != nil {
		healthStatus = *payload.HealthStatus
	}

	ctx := r.Context()

	if healthStatus != "" {
		resp, err := h.summaryFilteredByHealthStatus(ctx, payload, healthStatus)
		if err != nil {
			slog.ErrorContext(ctx, "customer health summary (filtered) failed", "userID", user.UserID, "err", summarizeErr(err))
			mapUpstreamErrorGeneric(w, err, "Failed to retrieve customer health summary.")
			return
		}
		writeJSONValue(w, http.StatusOK, resp)
		return
	}

	snResp, err := h.sn.GetCustomerHealthSummary(ctx, payload.Email, payload.Phrase, payload.Risks, payload.Region,
		payload.Product, payload.AbtTeam, payload.Offset, payload.Limit)
	if err != nil {
		slog.ErrorContext(ctx, "servicenow GetCustomerHealthSummary failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve customer health summary.")
		return
	}

	accountSysIDs := make([]string, len(snResp.Data))
	for i, acc := range snResp.Data {
		accountSysIDs[i] = acc.AccountSysID
	}
	reviewStatuses, err := h.risk.GetBatchHealthSummaries(ctx, accountSysIDs)
	if err != nil {
		slog.ErrorContext(ctx, "risk GetBatchHealthSummaries failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve customer health summary.")
		return
	}

	writeJSONValue(w, http.StatusOK, accountSummaryResponse{
		Data:       enrichAccountSummaries(snResp.Data, reviewStatuses),
		TotalCount: snResp.TotalCount,
	})
}

// summaryFilteredByHealthStatus implements the MySQL-filtered branch of
// GetSummary — see its doc comment.
func (h *CustomerHealthHandler) summaryFilteredByHealthStatus(ctx context.Context, payload customerHealthSummaryRequest, healthStatus string) (accountSummaryResponse, error) {
	matchingIDs, err := h.risk.GetAccountsByHealthStatus(ctx, healthStatus)
	if err != nil {
		return accountSummaryResponse{}, err
	}
	if len(matchingIDs) == 0 {
		return accountSummaryResponse{Data: []accountSummary{}, TotalCount: 0}, nil
	}
	idSet := make(map[string]bool, len(matchingIDs))
	for _, id := range matchingIDs {
		idSet[id] = true
	}

	const batchSize = 200
	// maxBatches bounds the loop even if the backing system's response is
	// inconsistent (e.g. it ignores offset, or caps a page below batchSize
	// while still reporting more remain) -- 500 batches is 100,000 accounts,
	// far beyond any real account count, so hitting it means the upstream
	// itself is behaving unexpectedly rather than this genuinely having
	// that much data.
	const maxBatches = 500
	snOffset := 0
	var allAccounts []servicenow.AccountSummary
	for i := 0; i < maxBatches; i++ {
		batch, err := h.sn.GetCustomerHealthSummary(ctx, payload.Email, payload.Phrase, payload.Risks, payload.Region,
			payload.Product, payload.AbtTeam, snOffset, batchSize)
		if err != nil {
			return accountSummaryResponse{}, err
		}
		allAccounts = append(allAccounts, batch.Data...)
		if len(batch.Data) == 0 || len(batch.Data) < batchSize || len(allAccounts) >= batch.TotalCount {
			break
		}
		snOffset += batchSize
	}

	filtered := make([]servicenow.AccountSummary, 0, len(allAccounts))
	for _, acc := range allAccounts {
		if idSet[acc.AccountSysID] {
			filtered = append(filtered, acc)
		}
	}

	total := len(filtered)
	// A negative Offset would otherwise reach filtered[startIdx:endIdx] as a
	// negative slice index and panic; clamp to 0 the same way a negative
	// Limit is never allowed to make endIdx < startIdx below.
	offset := max(payload.Offset, 0)
	startIdx := min(offset, total)
	endIdx := min(offset+payload.Limit, total)
	if endIdx < startIdx {
		endIdx = startIdx
	}
	paged := filtered[startIdx:endIdx]

	pagedIDs := make([]string, len(paged))
	for i, acc := range paged {
		pagedIDs[i] = acc.AccountSysID
	}
	reviewStatuses, err := h.risk.GetBatchHealthSummaries(ctx, pagedIDs)
	if err != nil {
		return accountSummaryResponse{}, err
	}

	return accountSummaryResponse{Data: enrichAccountSummaries(paged, reviewStatuses), TotalCount: total}, nil
}

func enrichAccountSummaries(accounts []servicenow.AccountSummary, reviewStatuses map[string]string) []accountSummary {
	enriched := make([]accountSummary, len(accounts))
	for i, acc := range accounts {
		var healthStatus *string
		if s, ok := reviewStatuses[acc.AccountSysID]; ok {
			healthStatus = &s
		}
		enriched[i] = accountSummary{
			AccountSysID:           acc.AccountSysID,
			AccountName:            acc.AccountName,
			HasNoGoLive:            acc.HasNoGoLive,
			HasRecentCases:         acc.HasRecentCases,
			HasEolProduct:          acc.HasEolProduct,
			HasAbandonedMigrations: acc.HasAbandonedMigrations,
			HasMigrationDelays:     acc.HasMigrationDelays,
			HasRecentEscalations:   acc.HasRecentEscalations,
			NoSupportCases6mo:      acc.NoSupportCases6mo,
			HealthStatus:           healthStatus,
		}
	}
	return enriched
}

// initHealthTrackingRequest is the payload for POST
// /customer-health/accounts/{accountSysId}/init-health-tracking.
type initHealthTrackingRequest struct {
	ProjectSysIDs []string `json:"projectSysIds"`
}

// InitHealthTracking handles POST
// /customer-health/accounts/{accountSysId}/init-health-tracking.
func (h *CustomerHealthHandler) InitHealthTracking(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerWriteAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	accountSysID := r.PathValue("accountSysId")
	if accountSysID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	var payload initHealthTrackingRequest
	if !decodeJSONBody(w, r, &payload) {
		return
	}

	if err := h.risk.InitProjectHealthRows(r.Context(), payload.ProjectSysIDs, accountSysID); err != nil {
		slog.ErrorContext(r.Context(), "risk InitProjectHealthRows failed", "userID", user.UserID, "accountSysId", accountSysID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to initialise health tracking.")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// GetAccountDetail handles GET /customer-health/accounts/{accountId}.
func (h *CustomerHealthHandler) GetAccountDetail(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	accountID := r.PathValue("accountId")
	if accountID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	detail, err := h.sn.GetCustomerHealthDetail(r.Context(), accountID)
	if err != nil {
		if errors.Is(err, servicenow.ErrAccountNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "servicenow GetCustomerHealthDetail failed", "userID", user.UserID, "accountId", accountID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve account health detail.")
		return
	}
	writeJSONValue(w, http.StatusOK, detail)
}

// OpenRisk handles POST /customer-health/projects/{projectSysId}/risk.
func (h *CustomerHealthHandler) OpenRisk(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerWriteAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	projectSysID := r.PathValue("projectSysId")
	if projectSysID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	var payload risk.OpenRiskRequest
	if !decodeJSONBody(w, r, &payload) {
		return
	}

	result, err := h.risk.OpenProjectRisk(r.Context(), projectSysID, payload.AccountSysID, payload.Comment, user.Email)
	if err != nil {
		slog.ErrorContext(r.Context(), "risk OpenProjectRisk failed", "userID", user.UserID, "projectSysId", projectSysID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to open project risk.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// CloseRisk handles PUT /customer-health/risks/{riskId}/close.
func (h *CustomerHealthHandler) CloseRisk(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerWriteAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	riskID, ok := parseIntPathValue(w, r, "riskId")
	if !ok {
		return
	}

	var payload risk.CloseRiskRequest
	if !decodeJSONBody(w, r, &payload) {
		return
	}

	result, err := h.risk.CloseProjectRisk(r.Context(), riskID, payload.Comment, user.Email)
	if err != nil {
		var valErr *risk.ValidationError
		if errors.As(err, &valErr) {
			writeError(w, http.StatusBadRequest, valErr.Message)
			return
		}
		slog.ErrorContext(r.Context(), "risk CloseProjectRisk failed", "userID", user.UserID, "riskId", riskID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to close project risk.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// MarkHealthy handles POST
// /customer-health/projects/{projectSysId}/mark-healthy.
func (h *CustomerHealthHandler) MarkHealthy(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerWriteAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	projectSysID := r.PathValue("projectSysId")
	if projectSysID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	var payload risk.MarkHealthyRequest
	if !decodeJSONBody(w, r, &payload) {
		return
	}

	result, err := h.risk.MarkProjectHealthy(r.Context(), projectSysID, payload.AccountSysID, user.Email, payload.Comment)
	if err != nil {
		slog.ErrorContext(r.Context(), "risk MarkProjectHealthy failed", "userID", user.UserID, "projectSysId", projectSysID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to mark project healthy.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// RevertReview handles POST
// /customer-health/projects/{projectSysId}/revert-review.
func (h *CustomerHealthHandler) RevertReview(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerWriteAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	projectSysID := r.PathValue("projectSysId")
	if projectSysID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	var payload risk.RevertHealthRequest
	if !decodeJSONBody(w, r, &payload) {
		return
	}

	result, err := h.risk.RevertProjectHealth(r.Context(), projectSysID, payload.AccountSysID)
	if err != nil {
		slog.ErrorContext(r.Context(), "risk RevertProjectHealth failed", "userID", user.UserID, "projectSysId", projectSysID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to revert project health review.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetAccountHealthStatus handles GET
// /customer-health/accounts/{accountSysId}/health-status.
func (h *CustomerHealthHandler) GetAccountHealthStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	accountSysID := r.PathValue("accountSysId")
	if accountSysID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.risk.GetAccountHealthStatus(r.Context(), accountSysID)
	if err != nil {
		slog.ErrorContext(r.Context(), "risk GetAccountHealthStatus failed", "userID", user.UserID, "accountSysId", accountSysID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve account health status.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetAccountHealthSummary handles GET
// /customer-health/accounts/{accountSysId}/health-summary.
func (h *CustomerHealthHandler) GetAccountHealthSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	accountSysID := r.PathValue("accountSysId")
	if accountSysID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.risk.GetAccountHealthSummary(r.Context(), accountSysID)
	if err != nil {
		slog.ErrorContext(r.Context(), "risk GetAccountHealthSummary failed", "userID", user.UserID, "accountSysId", accountSysID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve account health summary.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// GetProjectRiskHistory handles GET
// /customer-health/projects/{projectSysId}/risk-history.
func (h *CustomerHealthHandler) GetProjectRiskHistory(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}
	projectSysID := r.PathValue("projectSysId")
	if projectSysID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.risk.GetProjectRiskHistory(r.Context(), projectSysID)
	if err != nil {
		slog.ErrorContext(r.Context(), "risk GetProjectRiskHistory failed", "userID", user.UserID, "projectSysId", projectSysID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve project risk history.")
		return
	}
	writeJSONValue(w, http.StatusOK, result)
}

// parseIntPathValue reads and parses r.PathValue(name) as an int, writing a
// 400 response and returning ok=false on a missing or non-numeric value.
func parseIntPathValue(w http.ResponseWriter, r *http.Request, name string) (value int, ok bool) {
	raw := r.PathValue(name)
	n, err := strconv.Atoi(raw)
	if raw == "" || err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return 0, false
	}
	return n, true
}

// decodeJSONBody reads r.Body (capped at maxRequestBodyBytes) and decodes it
// as JSON into dst, writing an appropriate error response and returning
// false on any failure.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return false
	}
	if err := json.Unmarshal(body, dst); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return false
	}
	return true
}

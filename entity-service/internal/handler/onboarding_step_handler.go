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

package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// OnboardingStepHandler handles HTTP requests for the onboarding_step
// resource — the per-membership status ledger of the customer onboarding
// flow (see domain.OnboardingStep).
type OnboardingStepHandler struct {
	svc service.OnboardingStepService
}

// NewOnboardingStepHandler constructs an OnboardingStepHandler.
func NewOnboardingStepHandler(svc service.OnboardingStepService) *OnboardingStepHandler {
	return &OnboardingStepHandler{svc: svc}
}

// UpsertOnboardingStep handles PUT /onboarding-steps/{membershipSfId}/{step}.
func (h *OnboardingStepHandler) UpsertOnboardingStep(w http.ResponseWriter, r *http.Request) {
	var req domain.UpsertOnboardingStepRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.MembershipSfID = r.PathValue("membershipSfId")
	req.Step = domain.OnboardingStepName(r.PathValue("step"))
	step, err := h.svc.Upsert(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(step)
}

// GetOnboardingSteps handles GET /onboarding-steps/{membershipSfId}.
func (h *OnboardingStepHandler) GetOnboardingSteps(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetByMembership(r.Context(), r.PathValue("membershipSfId"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchOnboardingSteps handles POST /onboarding-steps/search.
func (h *OnboardingStepHandler) SearchOnboardingSteps(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchOnboardingStepsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

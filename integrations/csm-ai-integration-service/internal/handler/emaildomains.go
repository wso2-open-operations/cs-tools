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
	"errors"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/emaildomains"
)

// domainClassifier is emaildomains.Classifier.
type domainClassifier interface {
	Classify(ctx context.Context, domain string) (emaildomains.Result, error)
}

// EmailDomainHandler serves POST /ai/email-domains/classify.
type EmailDomainHandler struct {
	classifier domainClassifier
}

// NewEmailDomainHandler returns the handler.
func NewEmailDomainHandler(classifier *emaildomains.Classifier) *EmailDomainHandler {
	return &EmailDomainHandler{classifier: classifier}
}

type classifyRequest struct {
	Domain string `json:"domain"`
}

type classifyResponse struct {
	Domain         string `json:"domain"`
	Classification string `json:"classification"`
	Source         string `json:"source"`
	Usage          usage  `json:"usage"`
}

// Classify classifies one email domain:
//
//   - 200 {domain, classification, source, usage}
//   - 400 invalid_request     the body or the domain
//   - 503 not_configured      the model is needed and no API key is set
//   - 422, 429, 502, 504      the model call, as for dashboards
func (h *EmailDomainHandler) Classify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body classifyRequest
	if msg := decodeOne(r.Body, &body); msg != "" {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, msg)
		return
	}
	res, err := h.classifier.Classify(ctx, body.Domain)
	if err != nil {
		writeClassifyError(w, r, err)
		return
	}
	slog.InfoContext(ctx, "email domain classify: done", "classification", res.Classification, "source", res.Source)
	writeJSON(w, http.StatusOK, classifyResponse{
		Domain:         res.Domain,
		Classification: string(res.Classification),
		Source:         string(res.Source),
		Usage:          usage{InputTokens: res.InputTokens, OutputTokens: res.OutputTokens},
	})
}

// writeClassifyError maps a classifier error to a status and code. Never logs the domain.
func writeClassifyError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	switch {
	case errors.Is(err, emaildomains.ErrInvalidDomain):
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "domain is not a valid domain name")
	case errors.Is(err, emaildomains.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, CodeNotConfigured, "model calls are not configured")
	case errors.Is(err, emaildomains.ErrRefused):
		writeError(w, http.StatusUnprocessableEntity, CodeRefused, "the model refused the request")
	case errors.Is(err, emaildomains.ErrTruncated):
		writeError(w, http.StatusBadGateway, CodeTruncated, "the model's reply was cut off")
	case errors.Is(err, emaildomains.ErrUnusable):
		slog.WarnContext(ctx, "email domain classify: unusable reply", "err", err)
		writeError(w, http.StatusBadGateway, CodeUnusable, "the model's reply could not be used")
	case errors.Is(err, emaildomains.ErrUsageLimit):
		slog.WarnContext(ctx, "email domain classify: model usage limit reached", "err", err)
		writeError(w, http.StatusTooManyRequests, CodeUsageLimit, "the model's usage limit was reached")
	case errors.Is(err, emaildomains.ErrCall) && isTimeout(err):
		writeError(w, http.StatusGatewayTimeout, CodeTimeout, "the model call timed out")
	case errors.Is(err, emaildomains.ErrCall):
		slog.ErrorContext(ctx, "email domain classify: model call failed", "err", err)
		writeError(w, http.StatusBadGateway, CodeModelCallFailed, "the model could not be reached")
	default:
		slog.ErrorContext(ctx, "email domain classify: failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternal, "internal error")
	}
}

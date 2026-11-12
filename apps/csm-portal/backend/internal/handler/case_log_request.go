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
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

const logCollectionEventType = "request_logs"

// Question ids of the "Component Log Request" catalog item. Entity-service
// migration 0292 fixes them, so they are the same in every environment.
const (
	clusterQuestionID     = "f239e6be-0b03-49e9-8d66-4ea06858f161"
	environmentQuestionID = "81f21d11-4ecd-43cb-ad44-256532137748"
	namespaceQuestionID   = "dd6c260d-049b-4dd4-969e-0d9acce0fbb1"
	podNameQuestionID     = "b7dcd5d4-2d62-4406-8dbd-1c7b4cd3bd78"
	startTimeQuestionID   = "48bcac1b-0c37-4882-bc7e-6f5afc699e9d"
	endTimeQuestionID     = "d3a37969-c277-4ab5-ba95-0df23a0f8478"
	namedEmailsQuestionID = "1cae4e36-b64d-48e6-b196-8f62e0cfe044"
	componentQuestionID   = "f8f732aa-42d6-4da6-8872-2f3bc5bd0217"
	searchTextQuestionID  = "fccb11bc-8911-4245-bf0b-f406f4840821"
)

// The portal form marks these mandatory; nothing else enforces it.
var requiredLogRequestQuestions = []struct{ id, text string }{
	{clusterQuestionID, "Cluster"},
	{environmentQuestionID, "Choreo Environment"},
	{podNameQuestionID, "Pod Name"},
	{startTimeQuestionID, "Start Time"},
	{endTimeQuestionID, "End Time"},
	{namedEmailsQuestionID, "People Who Need Access"},
}

// Variables so tests can shorten them.
var (
	dispatchRetryDelays       = []time.Duration{2 * time.Second, 8 * time.Second}
	logCollectionStartTimeout = 2 * time.Minute
)

type pipelineDispatcher interface {
	DispatchRepositoryEvent(ctx context.Context, orgName, owner, repoName, eventType string, payload map[string]any) error
}

// LogCollectionConfig says which catalog item starts the log collection
// pipeline and which repository runs it.
type LogCollectionConfig struct {
	CatalogItemID string
	// Owner is also the organisation the engineering service picks its token by.
	Owner string
	Repo  string
	// Team is optional: the pipeline's notification team.
	Team string
}

type logCollection struct {
	pipeline pipelineDispatcher
	cfg      LogCollectionConfig
	running  sync.WaitGroup
}

// WithLogCollection makes POST /cases start the log collection pipeline after
// a service request is created from cfg.CatalogItemID. Unset, POST /cases is
// unchanged.
func (h *CaseHandler) WithLogCollection(pipeline pipelineDispatcher, cfg LogCollectionConfig) *CaseHandler {
	h.logCollection = &logCollection{pipeline: pipeline, cfg: cfg}
	return h
}

// WaitForLogCollection lets starts still retrying finish before shutdown, up
// to ctx's deadline.
func (h *CaseHandler) WaitForLogCollection(ctx context.Context) {
	if h.logCollection == nil {
		return
	}
	allDone := make(chan struct{})
	go func() {
		h.logCollection.running.Wait()
		close(allDone)
	}()
	select {
	case <-allDone:
	case <-ctx.Done():
		slog.Warn("shutdown: abandoned log collection starts that were still retrying")
	}
}

type logRequestForm struct {
	Type          string `json:"type"`
	CatalogItemID string `json:"catalogItemId"`
	Variables     []struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	} `json:"variables"`
}

type createdCase struct {
	Case struct {
		ID     string `json:"id"`
		Number string `json:"number"`
	} `json:"case"`
}

// startLogCollection starts the pipeline in the background when the service
// request just created is a Component Log Request, so the response never
// waits on GitHub. The outcome is reported as a work note on the case.
func (h *CaseHandler) startLogCollection(ctx context.Context, user *middleware.UserInfo, requestBody, responseBody []byte) {
	lc := h.logCollection
	if lc == nil {
		return
	}
	var form logRequestForm
	if err := json.Unmarshal(requestBody, &form); err != nil || form.Type != "service_request" || form.CatalogItemID != lc.cfg.CatalogItemID {
		return
	}
	var created createdCase
	if err := json.Unmarshal(responseBody, &created); err != nil || created.Case.ID == "" {
		slog.ErrorContext(ctx, "log collection not started: the create response has no case id", "userID", user.UserID, "err", err)
		return
	}

	// Keep the request's values (user token, correlation id) but not its
	// cancellation, which comes as soon as the response is written.
	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), logCollectionStartTimeout)
	lc.running.Add(1)
	go func() {
		defer lc.running.Done()
		defer cancel()
		h.dispatchLogCollection(bgCtx, user, form, created.Case.ID, created.Case.Number)
	}()
}

func (h *CaseHandler) dispatchLogCollection(ctx context.Context, user *middleware.UserInfo, form logRequestForm, caseID, caseNumber string) {
	lc := h.logCollection
	answerByQuestionID := make(map[string]string, len(form.Variables))
	for _, v := range form.Variables {
		answerByQuestionID[v.ID] = strings.TrimSpace(v.Value)
	}

	var missingQuestions []string
	for _, q := range requiredLogRequestQuestions {
		if answerByQuestionID[q.id] == "" {
			missingQuestions = append(missingQuestions, q.text)
		}
	}
	if len(missingQuestions) > 0 {
		slog.WarnContext(ctx, "log collection not started: required answers missing", "userID", user.UserID, "caseID", caseID, "missing", missingQuestions)
		h.addLogCollectionWorkNote(ctx, user, caseID,
			"Log collection was not started: these answers are missing: "+strings.Join(missingQuestions, ", ")+". Raise a new request with them filled in.")
		return
	}

	payload := buildPipelinePayload(answerByQuestionID, caseID, caseNumber, strings.TrimSpace(user.Email), lc.cfg.Team)
	repo := lc.cfg.Owner + "/" + lc.cfg.Repo
	err := dispatchWithRetry(ctx,
		func(ctx context.Context) error {
			return lc.pipeline.DispatchRepositoryEvent(ctx, lc.cfg.Owner, lc.cfg.Owner, lc.cfg.Repo, logCollectionEventType, payload)
		},
		func(attempt int, err error) {
			slog.WarnContext(ctx, "log collection dispatch attempt failed", "userID", user.UserID, "caseID", caseID, "attempt", attempt, "err", err)
		})
	if err != nil {
		slog.ErrorContext(ctx, "log collection could not be started", "userID", user.UserID, "caseID", caseID, "repo", repo, "err", err)
		h.addLogCollectionWorkNote(ctx, user, caseID,
			"Log collection could not be started ("+dispatchFailureReason(err)+"). Raise the request again, or ask the SRE team to run it.")
		return
	}
	slog.InfoContext(ctx, "log collection started", "userID", user.UserID, "caseID", caseID, "requestID", caseNumber, "repo", repo)
	h.addLogCollectionWorkNote(ctx, user, caseID,
		"Log collection started. The result will be posted here when it finishes.")
}

// buildPipelinePayload leaves empty answers out. GitHub allows 10 top-level
// properties, so the optional answers go under "options".
func buildPipelinePayload(answerByQuestionID map[string]string, caseID, caseNumber, requestedBy, team string) map[string]any {
	setIfPresent := func(m map[string]any, key, value string) {
		if value != "" {
			m[key] = value
		}
	}
	payload := map[string]any{}
	setIfPresent(payload, "case_id", caseID)
	setIfPresent(payload, "request_id", caseNumber)
	setIfPresent(payload, "requested_by", requestedBy)
	setIfPresent(payload, "team", team)
	setIfPresent(payload, "cluster", answerByQuestionID[clusterQuestionID])
	setIfPresent(payload, "pod_name", answerByQuestionID[podNameQuestionID])
	setIfPresent(payload, "start_time", answerByQuestionID[startTimeQuestionID])
	setIfPresent(payload, "end_time", answerByQuestionID[endTimeQuestionID])
	setIfPresent(payload, "named_emails", answerByQuestionID[namedEmailsQuestionID])

	options := map[string]any{}
	setIfPresent(options, "environment", answerByQuestionID[environmentQuestionID])
	setIfPresent(options, "namespace", answerByQuestionID[namespaceQuestionID])
	setIfPresent(options, "component", answerByQuestionID[componentQuestionID])
	setIfPresent(options, "query", answerByQuestionID[searchTextQuestionID])
	if len(options) > 0 {
		payload["options"] = options
	}
	return payload
}

// dispatchWithRetry tries once plus once per entry in dispatchRetryDelays,
// with up to 25% jitter on each wait. Only errors that may pass on a second
// try are retried.
func dispatchWithRetry(ctx context.Context, dispatch func(context.Context) error, onAttemptFailed func(attempt int, err error)) error {
	for attempt := 1; ; attempt++ {
		err := dispatch(ctx)
		if err == nil {
			return nil
		}
		onAttemptFailed(attempt, err)
		if attempt > len(dispatchRetryDelays) || !isRetryableDispatchError(err) {
			return err
		}
		wait := dispatchRetryDelays[attempt-1]
		if wait > 0 {
			wait += rand.N(wait/4 + 1)
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return err
		}
	}
}

// isRetryableDispatchError: a network error, timeout, 429 or 5xx may pass on
// a second try; any other 4xx will fail the same way again.
func isRetryableDispatchError(err error) bool {
	var upstream *apierror.Error
	if errors.As(err, &upstream) {
		return upstream.StatusCode == http.StatusTooManyRequests || upstream.StatusCode >= http.StatusInternalServerError
	}
	return true
}

// dispatchFailureReason never includes the upstream response body.
func dispatchFailureReason(err error) string {
	var upstream *apierror.Error
	if errors.As(err, &upstream) {
		return fmt.Sprintf("the Engineering Entity Service answered %d", upstream.StatusCode)
	}
	return "the Engineering Entity Service could not be reached"
}

// addLogCollectionWorkNote is best-effort: the case already exists.
func (h *CaseHandler) addLogCollectionWorkNote(ctx context.Context, user *middleware.UserInfo, caseID, content string) {
	note, err := json.Marshal(map[string]string{"type": "work_note", "content": content})
	if err != nil {
		slog.ErrorContext(ctx, "failed to build log collection work note", "userID", user.UserID, "caseID", caseID, "err", err)
		return
	}
	if _, err := h.entity.CreateCaseComment(ctx, caseID, note); err != nil {
		slog.WarnContext(ctx, "failed to add log collection work note", "userID", user.UserID, "caseID", caseID, "err", err)
	}
}

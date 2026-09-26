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

package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The outbound half fires repository_dispatch events, exactly as ServiceNow
// did. It does not write to the issue.
//
// THIS IS NOT AN IMPLEMENTATION CHOICE, IT IS THE CONTRACT. Each mapped
// repository runs GitHub Actions workflows that listen for these three event
// types and decide what to do with them -- post the comment, add or remove the
// assigned label, close the issue, format the text. Writing to the issue from
// here as well would duplicate everything they do.
//
// The event type strings and every client_payload field name below are read by
// those workflows, so they are not ours to rename.
const (
	dispatchNote       = "servicenow-note"
	dispatchCaseUpdate = "servicenow-case-update"
	dispatchCRUpdate   = "servicenow-cr-update"
)

// Queue event names, which are internal, unlike the dispatch types above.
const (
	outboundCRCreated     = "cr_created"
	outboundCRUpdated     = "cr_updated"
	outboundCommentAdded  = "comment_added"
	outboundCaseClosed    = "case_closed"
	outboundCaseAssigned  = "case_assigned"
	outboundRecordCreated = "record_created"
)

// githubDispatcher is the one GitHub call this half makes.
type githubDispatcher interface {
	Dispatch(ctx context.Context, owner, repository, eventType string, payload map[string]any) error
}

// GithubOutboundService pushes queued work to GitHub.
type GithubOutboundService interface {
	Deliver(ctx context.Context, item repository.OutboundItem) error
}

type githubOutboundService struct {
	gh githubDispatcher
}

// NewGithubOutboundService constructs the outbound dispatcher.
func NewGithubOutboundService(gh githubDispatcher) GithubOutboundService {
	return &githubOutboundService{gh: gh}
}

// ErrOutboundPermanent marks a failure that retrying cannot fix.
var ErrOutboundPermanent = errors.New("github outbound: permanent failure")

// Deliver fires the event for one queued row.
func (s *githubOutboundService) Deliver(ctx context.Context, item repository.OutboundItem) error {
	if item.Owner == "" || item.Repository == "" {
		return fmt.Errorf("%w: queue row %d has no repository to dispatch to", ErrOutboundPermanent, item.ID)
	}

	eventType, err := dispatchTypeFor(item)
	if err != nil {
		return err
	}

	// The payload is built by the trigger, in the shape the workflow expects.
	// Passing it through unchanged is the point: a field renamed or a value
	// reformatted here is a field the workflow no longer finds.
	payload := item.Payload
	if payload == nil {
		payload = map[string]any{}
	}

	return s.gh.Dispatch(ctx, item.Owner, item.Repository, eventType, payload)
}

func dispatchTypeFor(item repository.OutboundItem) (string, error) {
	switch item.Event {
	case outboundCommentAdded:
		return dispatchNote, nil
	case outboundCaseClosed, outboundCaseAssigned, outboundRecordCreated:
		return dispatchCaseUpdate, nil
	case outboundCRCreated, outboundCRUpdated:
		return dispatchCRUpdate, nil
	}
	return "", fmt.Errorf("%w: unknown event %q", ErrOutboundPermanent, item.Event)
}

// Queue mechanics below. ServiceNow had none of this -- a failed dispatch was
// logged with gs.warn and dropped -- but a dropped notice is a notice the
// customer never sees, so the row is retried with backoff and abandoned only
// after a bounded number of tries. This changes nothing about what GitHub
// receives, only how reliably it arrives.
const (
	// outboundMaxAttempts bounds retrying. Past this the row is FAILED and
	// stays visible rather than being retried forever against something no
	// retry can fix -- a deleted repository, a revoked token.
	outboundMaxAttempts = 6
	outboundBaseBackoff = 30 * time.Second
	outboundMaxBackoff  = 2 * time.Hour
)

// OutboundBackoff is how long to wait before attempt n, doubling and capped.
func OutboundBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := time.Duration(float64(outboundBaseBackoff) * math.Pow(2, float64(attempt-1)))
	if d > outboundMaxBackoff {
		return outboundMaxBackoff
	}
	return d
}

// OutboundRetryAfter reports how long GitHub asked us to wait, when it did.
// Honouring it is what keeps us inside the rate limit rather than hammering
// through it.
func OutboundRetryAfter(err error) (time.Duration, bool) {
	var apiErr *github.Error
	if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		return apiErr.RetryAfter, true
	}
	return 0, false
}

// OutboundPermanent reports whether retrying is pointless.
func OutboundPermanent(err error) bool {
	if errors.Is(err, ErrOutboundPermanent) {
		return true
	}
	var apiErr *github.Error
	if errors.As(err, &apiErr) {
		// 404 and 410: the repository is gone or the token cannot see it.
		// 403 without a Retry-After is a permissions problem, not a rate limit.
		if apiErr.StatusCode == 404 || apiErr.StatusCode == 410 {
			return true
		}
		if apiErr.StatusCode == 403 && !apiErr.RateLimited() {
			return true
		}
		// 422 means no workflow in that repository listens for this event
		// type. Retrying will not make one appear.
		if apiErr.StatusCode == 422 {
			return true
		}
	}
	return false
}

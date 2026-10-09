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
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Every test here starts from a request context that has already been
// cancelled -- the caller disconnected or REQUEST_TIMEOUT passed after the
// write committed -- and checks the notification still goes out. Before
// detachedNotifyContext each of them saw context.Canceled and lost the event.

type notifyCtxKey struct{}

// cancelledRequestContext is a request context carrying a value (as caller
// identity and correlation id are carried) that has already been cancelled.
func cancelledRequestContext() context.Context {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), notifyCtxKey{}, "request-value"))
	cancel()
	return ctx
}

func TestDetachedNotifyContext_SurvivesCallerCancellation(t *testing.T) {
	ctx, cancel := detachedNotifyContext(cancelledRequestContext())
	defer cancel()

	if err := ctx.Err(); err != nil {
		t.Fatalf("detached context is already done: %v", err)
	}
	if got := ctx.Value(notifyCtxKey{}); got != "request-value" {
		t.Errorf("request value = %v, want it carried over", got)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("detached context has no deadline, so a stuck notification could hang forever")
	}
	if left := time.Until(deadline); left <= 0 || left > notifyTimeout {
		t.Errorf("deadline in %v, want within (0, %v]", left, notifyTimeout)
	}

	cancel()
	if ctx.Err() == nil {
		t.Error("cancel func does not cancel the detached context")
	}
}

// ctxKafka records the context the Event Hub write ran on.
type ctxKafka struct {
	err         error
	hasDeadline bool
	left        time.Duration
	value       any
}

func (k *ctxKafka) Publish(ctx context.Context, _, _ []byte) error {
	k.err = ctx.Err()
	if d, ok := ctx.Deadline(); ok {
		k.hasDeadline, k.left = true, time.Until(d)
	}
	k.value = ctx.Value(notifyCtxKey{})
	return k.err
}

func TestPublish_WritesEvenWhenTheCallerIsGone(t *testing.T) {
	k := &ctxKafka{}
	store := &recordingFailures{}
	pub := NewEventPublisherService(k, store)

	if err := pub.Publish(cancelledRequestContext(), events.TypeCaseCreated, "case-1", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if k.err != nil {
		t.Errorf("Event Hub write ran on a done context: %v", k.err)
	}
	if !k.hasDeadline || k.left <= 0 || k.left > publishTimeout {
		t.Errorf("Event Hub write deadline in %v (set: %v), want within (0, %v]", k.left, k.hasDeadline, publishTimeout)
	}
	if k.value != "request-value" {
		t.Errorf("request value = %v, want it carried over", k.value)
	}
	if store.calls != 0 {
		t.Errorf("recorded %d failures for a publish that succeeded", store.calls)
	}
}

// ctxCheckingPublisher records whether each publish arrived on a live context.
type ctxCheckingPublisher struct {
	mu        sync.Mutex
	published []events.Type
	doneErrs  []error
}

func (p *ctxCheckingPublisher) Publish(ctx context.Context, eventType events.Type, _ string, _ json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		p.doneErrs = append(p.doneErrs, err)
		return err
	}
	p.published = append(p.published, eventType)
	return nil
}

func (p *ctxCheckingPublisher) Close() {}

func (p *ctxCheckingPublisher) check(t *testing.T, want events.Type) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.doneErrs) != 0 {
		t.Fatalf("publish ran on a done context: %v", p.doneErrs)
	}
	if len(p.published) != 1 || p.published[0] != want {
		t.Fatalf("published %v, want [%s]", p.published, want)
	}
}

func TestPostCommitHelpers_PublishAfterTheCallerIsGone(t *testing.T) {
	t.Run("case.workaround_provided", func(t *testing.T) {
		pub := &ctxCheckingPublisher{}
		publishWorkaroundProvidedEvent(cancelledRequestContext(), pub, "case-1")
		pub.check(t, events.TypeWorkaroundProvided)
	})

	t.Run("incident.assigned", func(t *testing.T) {
		pub := &ctxCheckingPublisher{}
		publishIncidentAssignedEvent(cancelledRequestContext(), pub, "inc-1",
			domain.EntityRef{ID: "user-1", Name: "Jane"})
		pub.check(t, events.TypeIncidentAssigned)
	})
}

// ctxAwareSRNoticeRepo fails a read on a done context, as the real repository
// does.
type ctxAwareSRNoticeRepo struct {
	fakeSRNoticeRepo
}

func (r *ctxAwareSRNoticeRepo) GetServiceRequest(ctx context.Context, caseID string) (repository.ServiceRequest, bool, error) {
	if err := ctx.Err(); err != nil {
		return repository.ServiceRequest{}, false, err
	}
	return r.fakeSRNoticeRepo.GetServiceRequest(ctx, caseID)
}

func TestSRNotices_RunAfterTheCallerIsGone(t *testing.T) {
	t.Run("sr.comment_added (the devops-sm alert)", func(t *testing.T) {
		repo := &ctxAwareSRNoticeRepo{fakeSRNoticeRepo{sr: newTestSR(srTestTeamID, "MS/PC SRE Group"), found: true}}
		pub := &ctxCheckingPublisher{}
		NewSRNoticeService(repo, pub, nil).OnComment(cancelledRequestContext(), srTestCaseID, "cm-1",
			domain.CommentTypeComment, "it is still broken", "jane@acme.test", "Jane", time.Now())
		pub.check(t, events.TypeSRCommentAdded)
	})

	t.Run("sr.created, assignment and acknowledgement", func(t *testing.T) {
		repo := &ctxAwareSRNoticeRepo{fakeSRNoticeRepo{sr: newTestSR(srTestTeamID, "MS/PC SRE Group"), found: true}}
		pub := &ctxCheckingPublisher{}
		NewSRNoticeService(repo, pub, []string{srTestTeamID}).OnCreated(cancelledRequestContext(), srTestCaseID)

		pub.mu.Lock()
		defer pub.mu.Unlock()
		if len(pub.doneErrs) != 0 {
			t.Fatalf("publish ran on a done context: %v", pub.doneErrs)
		}
		if len(pub.published) != 2 || pub.published[0] != events.TypeSRCreated || pub.published[1] != events.TypeSRAcknowledged {
			t.Errorf("published %v, want sr.created then sr.acknowledged", pub.published)
		}
		if len(repo.assigned) != 1 || len(repo.acknowledged) != 1 {
			t.Errorf("assigned %v, acknowledged %v; want one each", repo.assigned, repo.acknowledged)
		}
	})
}

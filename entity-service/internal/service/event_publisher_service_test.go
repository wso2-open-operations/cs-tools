// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/License-2.0
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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

type failKafka struct{}

func (failKafka) Publish(context.Context, []byte, []byte) error {
	return errors.New("broker down")
}

type recordingFailures struct {
	calls int
}

func (r *recordingFailures) CreateEventPublishFailure(context.Context, domain.CreateEventPublishFailureRequest) (domain.EventPublishFailure, error) {
	r.calls++
	return domain.EventPublishFailure{ID: "f1"}, nil
}

func (r *recordingFailures) ResolveEventPublishFailure(context.Context, string) (domain.EventPublishFailure, error) {
	return domain.EventPublishFailure{}, errors.New("unexpected ResolveEventPublishFailure")
}

func (r *recordingFailures) SearchEventPublishFailures(context.Context, domain.SearchEventPublishFailuresRequest) (domain.SearchEventPublishFailuresResponse, error) {
	return domain.SearchEventPublishFailuresResponse{}, errors.New("unexpected SearchEventPublishFailures")
}

func TestPublish_SkipsRecordWhenFailuresNil(t *testing.T) {
	pub := NewEventPublisherService(failKafka{}, nil)
	err := pub.Publish(context.Background(), events.TypeCaseCreated, "case-1", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected publish error")
	}
}

func TestPublish_RecordsWhenFailuresConfigured(t *testing.T) {
	store := &recordingFailures{}
	pub := NewEventPublisherService(failKafka{}, store)
	err := pub.Publish(context.Background(), events.TypeCaseCreated, "case-1", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected publish error")
	}
	if store.calls != 1 {
		t.Fatalf("CreateEventPublishFailure calls = %d, want 1", store.calls)
	}
}

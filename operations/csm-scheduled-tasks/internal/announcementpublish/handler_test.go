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

package announcementpublish

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeDueClient struct {
	ids           []string
	searchErr     error
	autoPublishFn func(id string) error
	published     []string
}

func (f *fakeDueClient) SearchDueIDs(context.Context) ([]string, error) {
	return f.ids, f.searchErr
}

func (f *fakeDueClient) AutoPublish(_ context.Context, id string) error {
	f.published = append(f.published, id)
	if f.autoPublishFn != nil {
		return f.autoPublishFn(id)
	}
	return nil
}

func TestPublishDue_NoDueRequestsSucceedsWithoutCallingAutoPublish(t *testing.T) {
	client := &fakeDueClient{}
	if err := PublishDue(client)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.published) != 0 {
		t.Fatalf("expected AutoPublish never called, got %v", client.published)
	}
}

func TestPublishDue_PropagatesSearchFailure(t *testing.T) {
	client := &fakeDueClient{searchErr: errors.New("boom")}
	if err := PublishDue(client)(context.Background()); err == nil {
		t.Fatal("expected an error when the search itself fails")
	}
}

func TestPublishDue_CallsAutoPublishForEveryDueID(t *testing.T) {
	client := &fakeDueClient{ids: []string{"req-1", "req-2", "req-3"}}
	if err := PublishDue(client)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.published) != 3 {
		t.Fatalf("expected all 3 due ids attempted, got %v", client.published)
	}
}

// TestPublishDue_OneFailureDoesNotStopTheRest is the core resilience
// guarantee: a single stuck/failing request must never block every other
// unrelated scheduled announcement from still being attempted this tick.
func TestPublishDue_OneFailureDoesNotStopTheRest(t *testing.T) {
	client := &fakeDueClient{
		ids: []string{"req-1", "req-2", "req-3"},
		autoPublishFn: func(id string) error {
			if id == "req-2" {
				return errors.New("still failing")
			}
			return nil
		},
	}
	err := PublishDue(client)(context.Background())
	if err == nil {
		t.Fatal("expected an error summarizing the one failed request")
	}
	if len(client.published) != 3 {
		t.Fatalf("expected every due id still attempted despite req-2 failing, got %v", client.published)
	}
	if !strings.Contains(err.Error(), "req-2") {
		t.Fatalf("expected the joined error to name the failed request, got: %v", err)
	}
}

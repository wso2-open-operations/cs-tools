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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// failingLookups is a repository whose link lookups fail; nothing else is called
// on the paths these tests take.
type failingLookups struct {
	repository.ProductIssueSyncRepository
}

func (failingLookups) LinkByIssue(context.Context, string, string, int) (*repository.LinkedIssue, error) {
	return nil, errors.New("database unavailable")
}

func (failingLookups) LinkByNodeID(context.Context, string) (*repository.LinkedIssue, error) {
	return nil, errors.New("database unavailable")
}

// A failed "is this ours" lookup must not break the customer repository sync: the
// delivery goes to the old path as if unlinked and the response is the old path's.
func TestProductIssueDeliveries_FailedLookupHandsTheDeliveryToTheOldPath(t *testing.T) {
	for _, fx := range []string{"02", "08", "07"} {
		d := pwhFixture(t, fx)
		old := &pwhOld{}
		svc := WithProductIssueDeliveries(old, NewProductIssueDeliveries(failingLookups{}, nil))
		out, err := svc.HandleWebhook(context.Background(), d.delivery(t))
		if err != nil || out.Action != "old_path" || len(old.got) != 1 {
			t.Fatalf("fixture %s: out=%+v err=%v old calls=%d", fx, out, err, len(old.got))
		}
		if got := old.got[0]; got.ID != d.ID || string(got.Raw) != string(d.Raw) {
			t.Errorf("fixture %s: the old path got a changed delivery", fx)
		}
	}
}

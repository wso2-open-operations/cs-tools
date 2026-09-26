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

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The Postgres data source has no feedback store. Every feedback operation
// must report the documented 503 ServiceUnavailableError rather than the
// route being absent (which, before this existed, the mux answered with an
// undocumented 404 -- found live as the webapp's case Activity timeline
// always showing "Could not load Case Feedback").
func TestUnavailableFeedbackService(t *testing.T) {
	svc := NewUnavailableFeedbackService()
	ctx := context.Background()

	calls := map[string]func() error{
		"SearchFeedback": func() error {
			_, err := svc.SearchFeedback(ctx, domain.SearchFeedbackRequest{})
			return err
		},
		"AggregateFeedback": func() error {
			_, err := svc.AggregateFeedback(ctx, domain.AggregateFeedbackRequest{})
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			var sue *apierror.ServiceUnavailableError
			if !errors.As(err, &sue) {
				t.Fatalf("err = %v (%T), want *apierror.ServiceUnavailableError", err, err)
			}
			if sue.Msg != feedbackUnavailableMsg {
				t.Errorf("Msg = %q, want %q", sue.Msg, feedbackUnavailableMsg)
			}
		})
	}
}

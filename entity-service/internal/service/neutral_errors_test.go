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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestDataSourceErrors_AreVendorNeutral pins that the "not supported by this
// data source" errors callers see do not name the backing system.
func TestDataSourceErrors_AreVendorNeutral(t *testing.T) {
	svc := &caseService{}
	_, feedbackErr := svc.GetCaseFeedback(context.Background(), authzCaseID)
	_, attachmentErr := svc.GetAttachment(context.Background(), authzCaseID)
	str := func(s string) *string { return &s }
	_, updateErr := (&caseService{}).UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: authzCaseID, Product: str("x")})

	msgs := []string{taskUnavailableMsg, feedbackUnavailableMsg}
	for _, err := range []error{feedbackErr, attachmentErr, updateErr} {
		if err == nil {
			t.Fatal("expected an error")
		}
		msgs = append(msgs, err.Error())
	}
	for _, m := range msgs {
		if strings.Contains(strings.ToLower(m), "servicenow") {
			t.Errorf("message names the backing system: %q", m)
		}
		if !strings.Contains(m, "not supported by this data source") {
			t.Errorf("message %q does not use the neutral wording", m)
		}
	}
}

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
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestCreateCommentReferenceTypes pins which reference types the reference-generic
// create-comment path accepts. "case" must be rejected before any downstream call:
// case comments belong to the dedicated case route, and accepting them here as well
// would leave two paths to the same outcome, free to drift apart.
func TestCreateCommentReferenceTypes(t *testing.T) {
	t.Parallel()

	newSvc := func(t *testing.T, called *bool) CommentService {
		t.Helper()
		client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*called = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"created","comment":{"id":"abc","createdOn":"2026-08-01 10:00:00","createdBy":"jane.doe@example.com"}}`))
		}))
		return NewServiceNowCommentService(client, nil)
	}

	req := func(refType domain.ReferenceType) domain.CreateCommentRequest {
		return domain.CreateCommentRequest{
			ReferenceID:   "00000000-0000-0000-0000-000000000000",
			ReferenceType: refType,
			Type:          domain.CommentTypeComment,
			Content:       "a comment",
		}
	}

	t.Run("rejects case without calling downstream", func(t *testing.T) {
		called := false
		svc := newSvc(t, &called)

		_, err := svc.CreateComment(context.Background(), req(domain.ReferenceTypeCase))
		if err == nil {
			t.Fatal("expected an error for referenceType case, got none")
		}
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T", err)
		}
		if want := "referenceType must be one of: conversation, change_request, deployment, incident"; ve.Msg != want {
			t.Errorf("got msg %q, want %q", ve.Msg, want)
		}
		if called {
			t.Error("downstream was called for referenceType case; it must be rejected before the request")
		}
	})

	t.Run("accepts every other supported reference type", func(t *testing.T) {
		for _, refType := range []domain.ReferenceType{
			domain.ReferenceTypeConversation,
			domain.ReferenceTypeChangeRequest,
			domain.ReferenceTypeDeployment,
			domain.ReferenceTypeIncident,
		} {
			called := false
			svc := newSvc(t, &called)

			resp, err := svc.CreateComment(context.Background(), req(refType))
			if err != nil {
				t.Errorf("referenceType %q: unexpected error: %v", refType, err)
				continue
			}
			if !called {
				t.Errorf("referenceType %q: downstream was not called", refType)
			}
			if resp.Message != "created" {
				t.Errorf("referenceType %q: got message %q, want %q", refType, resp.Message, "created")
			}
		}
	})
}

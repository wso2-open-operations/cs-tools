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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
)

// TestCallerEmail pins that the caller's e-mail comes only from the identity
// the auth middleware validated, never from the raw request header.
func TestCallerEmail(t *testing.T) {
	var sue *apierror.ServiceUnavailableError
	var ue *apierror.UnauthorizedError

	// Not validated: refused even when the header carries a well-formed token.
	unvalidated := auth.WithIdentity(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")), auth.Identity{})
	if _, err := callerEmail(unvalidated); !errors.As(err, &sue) {
		t.Fatalf("unvalidated identity: err = %v, want ServiceUnavailableError", err)
	}
	if _, err := optionalCallerEmail(unvalidated); !errors.As(err, &sue) {
		t.Fatalf("unvalidated identity (optional): err = %v, want ServiceUnavailableError", err)
	}
	if _, err := callerEmail(context.Background()); !errors.As(err, &sue) {
		t.Fatalf("no identity: err = %v, want ServiceUnavailableError", err)
	}

	// Validated, no user token.
	noUser := contextWithUserIDToken("")
	if _, err := callerEmail(noUser); !errors.As(err, &ue) {
		t.Fatalf("no user: err = %v, want UnauthorizedError", err)
	}
	if email, err := optionalCallerEmail(noUser); err != nil || email != "" {
		t.Fatalf("no user (optional): got %q, %v; want \"\", nil", email, err)
	}

	// Validated user: the identity's e-mail, not whatever the header says.
	ctx := auth.WithIdentity(contextWithUserIDToken(fakeJWTWithEmail(t, "someone.else@example.com")), auth.Identity{Validated: true, UserEmail: "jane.doe@example.com"})
	if email, err := callerEmail(ctx); err != nil || email != "jane.doe@example.com" {
		t.Fatalf("validated user: got %q, %v; want the identity's e-mail", email, err)
	}
}

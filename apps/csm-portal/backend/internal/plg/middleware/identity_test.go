package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	csm "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// This middleware is the authorisation gate on every PLG route: it is what
// keeps the section to active INTERNAL staff, and it is the only place that
// decides between 401, 403 and 503 for a caller PLG cannot place. Each of those
// three answers means something different to the frontend, so each is asserted
// rather than lumped into "not 200".

// resolverStub records the email it was asked about and answers with whatever
// the test set. A nil user with a nil error is the "authenticated, but not PLG
// staff" answer GetCSUser gives for an unknown address.
type resolverStub struct {
	user    *domain.UserRef
	err     error
	asked   []string
	callLog int
}

func (r *resolverStub) GetCSUser(_ context.Context, email string) (*domain.UserRef, error) {
	r.callLog++
	r.asked = append(r.asked, email)
	return r.user, r.err
}

// staffed is a resolver that recognises everyone as the same engineer.
func staffed() *resolverStub {
	return &resolverStub{user: &domain.UserRef{
		ID:    "11111111-1111-1111-1111-111111111111",
		Email: "jane@wso2.com",
		Name:  "Jane Doe",
	}}
}

// run drives one request through the middleware and reports what the wrapped
// handler saw, or that it was never reached.
type outcome struct {
	status   int
	body     string
	reached  bool
	userID   string
	userRef  domain.UserRef
	refFound bool
}

func run(t *testing.T, resolver UserResolver, path string, info *csm.UserInfo) outcome {
	t.Helper()

	var got outcome
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got.reached = true
		got.userID = UserIDFromContext(r.Context())
		got.userRef, got.refFound = UserRefFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if info != nil {
		req = req.WithContext(csm.WithUserInfo(req.Context(), info))
	}
	rec := httptest.NewRecorder()
	ResolveIdentity(resolver)(next).ServeHTTP(rec, req)

	got.status = rec.Code
	got.body = rec.Body.String()
	return got
}

// message pulls the message out of the JSON error body, so a test asserts on
// what a caller actually reads rather than on a substring of the raw bytes.
func message(t *testing.T, body string) string {
	t.Helper()
	var parsed struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("error body is not JSON: %q", body)
	}
	return parsed.Message
}

func TestResolvedCallerReachesTheHandler(t *testing.T) {
	resolver := staffed()
	got := run(t, resolver, "/plg/organizations", &csm.UserInfo{Email: "jane@wso2.com"})

	if !got.reached {
		t.Fatalf("handler was not reached: %d %s", got.status, got.body)
	}
	// The id is what every PLG write records as its actor, so the whole point
	// of the middleware is that it is present and is the database's UUID —
	// not the token's subject.
	if got.userID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("UserIDFromContext = %q", got.userID)
	}
	if !got.refFound || got.userRef.Name != "Jane Doe" || got.userRef.Email != "jane@wso2.com" {
		t.Errorf("UserRefFromContext = %+v, found=%v", got.userRef, got.refFound)
	}
	if len(resolver.asked) != 1 || resolver.asked[0] != "jane@wso2.com" {
		t.Errorf("resolver asked about %v, want the token's email once", resolver.asked)
	}
}

func TestUnauthenticatedCallerIs401(t *testing.T) {
	// csm-portal's Auth middleware rejects these before this runs, so reaching
	// here means the chain was assembled wrongly — but it must still refuse
	// rather than fall through to a handler with no actor.
	for _, tc := range []struct {
		name string
		info *csm.UserInfo
	}{
		{"no user info at all", nil},
		{"user info with no email", &csm.UserInfo{Email: ""}},
		{"user info with a blank email", &csm.UserInfo{Email: "   "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := staffed()
			got := run(t, resolver, "/plg/me", tc.info)

			if got.status != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", got.status)
			}
			if got.reached {
				t.Error("handler ran without a caller identity")
			}
			if resolver.callLog != 0 {
				t.Error("resolver was called with no email to resolve")
			}
		})
	}
}

func TestLookupFailureIs503NotARejection(t *testing.T) {
	// entity-service being unreachable says nothing about the caller. A 401 or
	// 403 here would tell a perfectly valid engineer to sign in again, or that
	// they are not staff, for what is an outage.
	resolver := &resolverStub{err: errors.New("dial tcp: connection refused")}
	got := run(t, resolver, "/plg/organizations", &csm.UserInfo{Email: "jane@wso2.com"})

	if got.status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", got.status)
	}
	if got.reached {
		t.Error("handler ran on an unresolved caller")
	}
	if msg := message(t, got.body); !strings.Contains(msg, "try again") {
		t.Errorf("message = %q, want it to invite a retry", msg)
	}
	// The downstream error must not reach the caller.
	if strings.Contains(got.body, "connection refused") {
		t.Errorf("body leaks the downstream error: %q", got.body)
	}
}

func TestNonStaffCallerIs403AndNamesTheAddress(t *testing.T) {
	// Authenticated, but not an active INTERNAL user. 403 rather than 401:
	// their token is valid and signing in again cannot fix it.
	resolver := &resolverStub{user: nil}
	got := run(t, resolver, "/plg/organizations", &csm.UserInfo{Email: "outsider@example.com"})

	if got.status != http.StatusForbidden {
		t.Errorf("status = %d, want 403", got.status)
	}
	if got.reached {
		t.Error("handler ran for a caller who is not PLG staff")
	}
	if msg := message(t, got.body); !strings.Contains(msg, "outsider@example.com") {
		t.Errorf("message = %q, want it to name the address that was not found", msg)
	}
}

func TestOnlyPlgPathsAreGated(t *testing.T) {
	// The middleware is mounted on the PLG subtree, so this guard is belt and
	// braces — but it is what stops a future mounting mistake from demanding a
	// PLG user for csm-portal's own routes.
	for _, path := range []string{"/cases", "/health", "/plgx/not-ours", "/"} {
		t.Run(path, func(t *testing.T) {
			resolver := &resolverStub{err: errors.New("must not be called")}
			got := run(t, resolver, path, nil)

			if !got.reached {
				t.Errorf("non-PLG path %s was gated: %d", path, got.status)
			}
			if resolver.callLog != 0 {
				t.Errorf("resolver was called for non-PLG path %s", path)
			}
			// Nothing was resolved, so nothing should be in the context.
			if got.userID != "" || got.refFound {
				t.Error("a non-PLG request carried a PLG identity")
			}
		})
	}
}

func TestContextHelpersAreSafeOnABareContext(t *testing.T) {
	// A handler reached outside this middleware must get zero values rather
	// than panic on a failed type assertion.
	if id := UserIDFromContext(context.Background()); id != "" {
		t.Errorf("UserIDFromContext = %q, want empty", id)
	}
	if ref, ok := UserRefFromContext(context.Background()); ok || ref != (domain.UserRef{}) {
		t.Errorf("UserRefFromContext = %+v, %v; want zero, false", ref, ok)
	}
}

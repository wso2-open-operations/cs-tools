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

package server

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestEveryRegisteredRouteIsPublished guards against a failure mode that is
// invisible locally: a route registered here but absent from the published
// openapi.yaml works in every local and in-cluster test, then 404s in any
// deployment where a gateway routes only the operations the contract declares.
// The gateway rejects the request before this service ever sees it, so no
// amount of service-side testing catches it.
//
// Comparison is on the method plus the path shape with parameter names
// normalized away, because the contract and the router are free to name a path
// parameter differently ({id} vs {caseId}) without being in disagreement.
func TestEveryRegisteredRouteIsPublished(t *testing.T) {
	registered := registeredRoutes(t)
	if len(registered) == 0 {
		t.Fatal("parsed no routes from routes.go; the parser is broken, not the routes")
	}
	published := publishedOperations(t)
	if len(published) == 0 {
		t.Fatal("parsed no operations from openapi.yaml; the parser is broken, not the contract")
	}

	// The liveness probe is published in both contracts (it is also served on
	// the health listener); it is exempt here only because this check is
	// about business routes a gateway must route.
	skip := map[string]bool{"GET /health": true}

	var missing []string
	for _, r := range registered {
		if skip[r] {
			continue
		}
		if _, ok := published[r]; !ok {
			missing = append(missing, r)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("routes registered in routes.go but not declared in openapi.yaml:\n  %s\n\n"+
			"An undeclared operation is unreachable behind an API gateway even though it works locally. "+
			"Add it to openapi.yaml (and set its resource scopes when publishing).",
			strings.Join(missing, "\n  "))
	}
}

// TestAnonymousRoutesMatchTheContract pins the pairing between the
// caller-identity gate's anonymous allow-list (anonymousRoutes) and the
// operations the published contract declares with `security: []`. The two
// must agree in both directions: a route served without a resolvable caller
// must say so in the contract, and an operation the contract calls anonymous
// must actually be reachable without one.
func TestAnonymousRoutesMatchTheContract(t *testing.T) {
	published := publishedOperations(t)
	if len(published) == 0 {
		t.Fatal("parsed no operations from openapi.yaml; the parser is broken, not the contract")
	}

	declaredAnonymous := map[string]bool{}
	for op, spec := range published {
		if spec.anonymous {
			declaredAnonymous[op] = true
		}
	}
	gateAnonymous := map[string]bool{}
	for pattern := range anonymousRoutes {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Fatalf("anonymousRoutes entry %q is not a METHOD /path pattern", pattern)
		}
		gateAnonymous[normalizeRoute(method, path)] = true
	}

	for op := range gateAnonymous {
		if !declaredAnonymous[op] {
			t.Errorf("%s is served without a resolvable caller (anonymousRoutes) but openapi.yaml does not declare it with `security: []`", op)
		}
	}
	for op := range declaredAnonymous {
		if !gateAnonymous[op] {
			t.Errorf("openapi.yaml declares %s with `security: []` but callerIdentityMiddleware still requires a resolvable caller for it; add it to anonymousRoutes or drop the declaration", op)
		}
	}
}

// TestOperationalRoutesAreInternalOnly pins the route-level authorization
// wiring for the resources that have no row-level security and no per-project
// scope: each listed operation must be registered through internalOnly (or
// projectMemberOnly where noted), so that a resolvable but non-internal caller
// -- a customer with a valid user token -- is refused on the route itself,
// independent of which service or repository backs the handler.
func TestOperationalRoutesAreInternalOnly(t *testing.T) {
	guarded := guardedRoutes(t)
	if len(guarded) == 0 {
		t.Fatal("parsed no guarded routes; the parser is broken, not the routes")
	}

	required := []string{
		"POST /salesforce/events",
		"POST /event-publish-failures", "POST /event-publish-failures/search", "POST /event-publish-failures/{id}/resolve",
		"POST /scheduled-tasks/attempts", "PATCH /scheduled-tasks/attempts/{id}", "GET /scheduled-tasks/attempts", "DELETE /scheduled-tasks/attempts",
		"POST /alert-incident-mappings", "POST /alert-incident-mappings/lookup",
		"GET /teams/{id}/members", "GET /products/github-repo",
		"POST /internal/cloud-status/sweep", "GET /internal/cloud-status/pending", "POST /internal/cloud-status/{id}/delivery",
		"GET /accounts/{id}", "POST /accounts/search", "POST /accounts/{id}/contacts/search",
		"POST /time-cards", "PATCH /time-cards/{id}", "DELETE /time-cards/{id}",
		"POST /incidents", "POST /problems", "POST /incident-tasks/search",
	}
	plg := registeredRoutesIn(t, "plg_routes.go")
	if len(plg) == 0 {
		t.Fatal("parsed no routes from plg_routes.go; the parser is broken, not the routes")
	}
	required = append(required, plg...)

	for _, pattern := range required {
		method, path, _ := strings.Cut(pattern, " ")
		if guard := guarded[normalizeRoute(method, path)]; guard != "internalOnly" {
			t.Errorf("%s is registered with guard %q, want internalOnly", pattern, guard)
		}
	}
	for _, pattern := range []string{"POST /projects/{id}/contacts/search", "GET /projects/{id}/contacts/{contactId}"} {
		method, path, _ := strings.Cut(pattern, " ")
		if guard := guarded[normalizeRoute(method, path)]; guard != "projectMemberOnly" {
			t.Errorf("%s is registered with guard %q, want projectMemberOnly", pattern, guard)
		}
	}
}

// TestGuardedOperationsDeclareTheirRefusals: every operation registered
// through internalOnly or projectMemberOnly can answer 401 (no resolvable
// caller) and 403 (a caller the guard refuses), so the published contract must
// say so -- a consumer reading the spec otherwise treats them as generic
// failures.
func TestGuardedOperationsDeclareTheirRefusals(t *testing.T) {
	published := publishedOperations(t)
	guarded := guardedRoutes(t)
	if len(guarded) == 0 || len(published) == 0 {
		t.Fatal("parsed nothing; the parser is broken, not the routes or the contract")
	}
	for route := range guarded {
		op, ok := published[route]
		if !ok {
			continue // TestEveryRegisteredRouteIsPublished reports this
		}
		for _, code := range []string{"401", "403"} {
			if !op.responses[code] {
				t.Errorf("%s is guarded by %s but openapi.yaml does not declare %s", route, guarded[route], code)
			}
		}
	}
}

var (
	handleFuncRE = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/[^"]*)"`)
	guardedRE    = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/[^"]*)",\s*(internalOnly|projectMemberOnly)\(`)
	pathParamRE  = regexp.MustCompile(`\{[^}]*\}`)
)

// normalizeRoute renders a method and path as a comparable key with path
// parameter names collapsed, e.g. `POST /cases/{id}/tags` -> `POST /cases/{}/tags`.
func normalizeRoute(method, path string) string {
	return strings.ToUpper(method) + " " + pathParamRE.ReplaceAllString(strings.TrimSuffix(path, "/"), "{}")
}

// guardedRoutes maps every route registered through a route-level guard, in
// routes.go and plg_routes.go, to the guard's name. The outage writes, whose
// guard is chosen at wiring time by data source, are not literal and so are
// not reported here.
func guardedRoutes(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, file := range []string{"routes.go", "plg_routes.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, m := range guardedRE.FindAllStringSubmatch(string(src), -1) {
			out[normalizeRoute(m[1], m[2])] = m[3]
		}
	}
	return out
}

// registeredRoutes extracts every route literal registered in routes.go.
// Reading the source rather than the built mux avoids having to construct the
// full handler dependency graph, and every registration in this package is a
// string literal (a non-literal pattern would simply not be seen, which the
// empty-result guard in the test catches if it ever becomes the norm).
func registeredRoutes(t *testing.T) []string {
	return registeredRoutesIn(t, "routes.go")
}

// registeredRoutesIn is registeredRoutes over one named source file.
func registeredRoutesIn(t *testing.T, file string) []string {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range handleFuncRE.FindAllStringSubmatch(string(src), -1) {
		key := normalizeRoute(m[1], m[2])
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

var (
	// A path key under `paths:`, at two-space indentation, e.g. `  /cases/{id}:`.
	specPathRE = regexp.MustCompile(`^  (/\S*):\s*$`)
	// An HTTP method under a path, at four-space indentation.
	specMethodRE = regexp.MustCompile(`^    (get|put|post|delete|patch|options|head|trace):\s*$`)
	// The operation-level `security: []` override, at six-space indentation.
	specAnonymousRE = regexp.MustCompile(`^      security:\s*\[\s*\]\s*$`)
	// The `responses:` key of an operation, at six-space indentation.
	specResponsesRE = regexp.MustCompile(`^      responses:\s*$`)
	// A status code key under `responses:`, at eight-space indentation,
	// quoted or not, e.g. `        '403':` or `        "403":` or `        403:`.
	specStatusRE = regexp.MustCompile(`^        ['"]?(\d{3}|default)['"]?:\s*$`)
)

// specOperation is what the contract says about one published operation.
type specOperation struct {
	// anonymous is true when the operation declares `security: []`, i.e. the
	// gateway and this service both serve it without a caller identity.
	anonymous bool
	// responses is the set of status codes the operation declares.
	responses map[string]bool
}

// publishedOperations extracts every method+path declared in the published
// contract, with the per-operation facts the contract tests check. The spec is
// parsed line-wise on indentation rather than with a YAML library so that this
// check adds no dependency; the tests' empty-result guards fail loudly if the
// spec's formatting ever moves out from under it.
func publishedOperations(t *testing.T) map[string]*specOperation {
	t.Helper()
	specPath := filepath.Join("..", "..", "openapi.yaml")
	src, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}

	ops := map[string]*specOperation{}
	inPaths := false
	current := ""
	var op *specOperation
	inResponses := false
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(line, "paths:") {
			inPaths = true
			continue
		}
		if !inPaths {
			continue
		}
		// A new top-level key ends the paths section.
		if len(line) > 0 && line[0] != ' ' && line[0] != '#' {
			if !strings.HasPrefix(line, "paths:") {
				inPaths = false
			}
			continue
		}
		if m := specPathRE.FindStringSubmatch(line); m != nil {
			current = m[1]
			op, inResponses = nil, false
			continue
		}
		if current == "" {
			continue
		}
		if m := specMethodRE.FindStringSubmatch(line); m != nil {
			op = &specOperation{responses: map[string]bool{}}
			ops[normalizeRoute(m[1], current)] = op
			inResponses = false
			continue
		}
		if op == nil {
			continue
		}
		switch {
		case specAnonymousRE.MatchString(line):
			op.anonymous = true
			inResponses = false
		case specResponsesRE.MatchString(line):
			inResponses = true
		case inResponses:
			if m := specStatusRE.FindStringSubmatch(line); m != nil {
				op.responses[m[1]] = true
			} else if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "        ") {
				// Back out to a sibling of `responses:` (or shallower).
				inResponses = false
			}
		}
	}
	return ops
}

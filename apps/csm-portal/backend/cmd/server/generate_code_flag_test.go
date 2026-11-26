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

package main

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateCodeEnabledIsOnOnlyForExactlyTrue(t *testing.T) {
	for raw, want := range map[string]bool{
		"": false, "false": false, "1": false, "TRUE": false, "True": false, "yes": false, "on": false,
		"true": true, "  true\n": true,
	} {
		if got := generateCodeEnabled(raw); got != want {
			t.Errorf("generateCodeEnabled(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestLoadGenerateCodeEnabledReadsEnv(t *testing.T) {
	t.Setenv(generateCodeFlag, "true")
	if !loadGenerateCodeEnabled() {
		t.Error("on: want true for exactly \"true\"")
	}
	t.Setenv(generateCodeFlag, "")
	if loadGenerateCodeEnabled() {
		t.Error("unset: want off")
	}
}

// The code approach's routes are registered only inside the flag's branch,
// so a deployment with the flag off has no POST /dashboards/generate-code or
// POST /dashboards/resolved-filters at all.
func TestTheGenerateCodeRouteIsRegisteredOnlyBehindTheFlag(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, route := range []string{`route("POST /dashboards/generate-code"`, `route("POST /dashboards/resolved-filters"`} {
		if strings.Count(text, route) != 1 {
			t.Fatalf("want exactly one registration of %s", route)
		}
		before := text[:strings.Index(text, route)]
		guard := strings.LastIndex(before, "if dashboardGenerateCodeHandler != nil {")
		if guard < 0 || strings.Contains(before[guard:], "}") {
			t.Errorf("%s must be registered inside `if dashboardGenerateCodeHandler != nil {`", route)
		}
	}
	if !strings.Contains(text, "if loadGenerateCodeEnabled() {") {
		t.Error("the code handler must be built only when loadGenerateCodeEnabled() is true")
	}
}

// Saving advanced dashboards is part of the same phase: its routes are
// registered only inside the code dashboard handler's branch, and that
// handler is built only inside the flag's branch, so a deployment with the
// flag off has none of them.
func TestTheCodeDashboardRoutesAreRegisteredOnlyBehindTheFlag(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, route := range []string{
		`route("GET /dashboards/code",`, `route("GET /dashboards/code/{id}"`,
		`route("PUT /dashboards/code/{id}"`, `route("DELETE /dashboards/code/{id}"`,
	} {
		if strings.Count(text, route) != 1 {
			t.Fatalf("want exactly one registration of %s", route)
		}
		before := text[:strings.Index(text, route)]
		guard := strings.LastIndex(before, "if codeDashboardHandler != nil {")
		// The route lines hold "{id}", so the block's end is a line of its own.
		if guard < 0 || strings.Contains(before[guard:], "\n\t}\n") {
			t.Errorf("%s must be registered inside `if codeDashboardHandler != nil {`", route)
		}
	}
	build := "codeDashboardHandler = handler.NewCodeDashboardHandler("
	if strings.Count(text, build) != 1 {
		t.Fatalf("want exactly one %s", build)
	}
	before := text[:strings.Index(text, build)]
	guard := strings.LastIndex(before, "if loadGenerateCodeEnabled() {")
	if guard < 0 || strings.Count(before[guard:], "{") != strings.Count(before[guard:], "}")+1 {
		t.Error("the code dashboard handler must be built only inside `if loadGenerateCodeEnabled() {`")
	}
}

// The advanced dashboard sharing routes, and the handler behind them, exist
// only with the flag on, like the saved advanced dashboards.
func TestTheCodeDashboardShareRoutesAreRegisteredOnlyBehindTheFlag(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, route := range []string{
		`route("GET /dashboards/code/{id}/shares"`, `route("PUT /dashboards/code/{id}/shares/{userId}"`,
		`route("DELETE /dashboards/code/{id}/shares/{userId}"`, `route("GET /dashboards/code/{id}/team-shares"`,
		`route("PUT /dashboards/code/{id}/team-shares/{teamKey}"`, `route("DELETE /dashboards/code/{id}/team-shares/{teamKey}"`,
		`route("GET /dashboards/code/{id}/link"`, `route("PUT /dashboards/code/{id}/link"`,
		`route("DELETE /dashboards/code/{id}/link"`, `route("GET /dashboards/shared/code",`,
		`route("GET /dashboards/shared/code/{id}"`, `route("POST /dashboards/shared/code/link"`,
	} {
		if strings.Count(text, route) != 1 {
			t.Fatalf("want exactly one registration of %s", route)
		}
		before := text[:strings.Index(text, route)]
		guard := strings.LastIndex(before, "if codeDashboardShareHandler != nil {")
		if guard < 0 || strings.Contains(before[guard:], "\n\t}\n") {
			t.Errorf("%s must be registered inside `if codeDashboardShareHandler != nil {`", route)
		}
	}
	build := "codeDashboardShareHandler = handler.NewCodeDashboardShareHandler("
	if strings.Count(text, build) != 1 {
		t.Fatalf("want exactly one %s", build)
	}
	before := text[:strings.Index(text, build)]
	guard := strings.LastIndex(before, "if loadGenerateCodeEnabled() {")
	if guard < 0 || strings.Count(before[guard:], "{") != strings.Count(before[guard:], "}")+1 {
		t.Error("the code dashboard share handler must be built only inside `if loadGenerateCodeEnabled() {`")
	}
}

// Both approaches spend the same model budget, so main.go gives the two
// handlers one GenerateRateLimiter: a user's code and JSON calls count
// together.
func TestBothGenerateHandlersShareOneLimiter(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	if strings.Count(text, "handler.NewGenerateRateLimiter(") != 1 {
		t.Error("main.go must build exactly one generate limiter")
	}
	for _, want := range []string{
		"handler.NewDashboardGenerateHandler(dashboardGenerator).WithRateLimit(generateLimiter)",
		"handler.NewDashboardGenerateCodeHandler(codeDashboardGenerator).WithRateLimit(generateLimiter)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("main.go lacks %s", want)
		}
	}
}

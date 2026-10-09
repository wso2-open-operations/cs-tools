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
	"os"
	"strings"
	"testing"
)

// TestCompanyIDForLogNamesTheEmptyCase covers the one branch that is easy to get
// wrong and impossible to notice in production.
//
// companyId is not one of the source map's RequiredPortalFields, so an empty
// value is reachable. Printing it bare would render "moesif_company_id=:" and
// read as a formatting bug rather than as a record that arrived without the
// identifier.
func TestCompanyIDForLogNamesTheEmptyCase(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"a real id passes through", "6ed053ee-6c22-449c-994b-94cc84652583", "6ed053ee-6c22-449c-994b-94cc84652583"},
		{"empty is named", "", "(unmapped)"},
		{"whitespace counts as empty", "   ", "(unmapped)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := companyIDForLog(c.in); got != c.want {
				t.Errorf("companyIDForLog(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestIngestFailureLogIdentifiesByCompanyID pins how a failed registration is
// identified in the log.
//
// Asserting the company id is present is not enough on its own: a line could
// carry both identifiers and still pass. So this also asserts the absence of the
// field it replaced, against the real source of the function that logs.
func TestIngestFailureLogIdentifiesByCompanyID(t *testing.T) {
	src := readIngestServiceSource(t)
	if contains(src, "reg.OrganizationName, err") {
		t.Error("the ingest failure log is identifying the record by name again; " +
			"use reg.CompanyID (moesif_company_id), the column the row is stored under")
	}
	if !contains(src, "moesif_company_id=%s") {
		t.Error("the ingest failure log no longer identifies the record by its " +
			"Moesif company id; a failure nobody can tie to a record is not actionable")
	}
}

func readIngestServiceSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("plg_ingest_service.go")
	if err != nil {
		t.Fatalf("read plg_ingest_service.go: %v", err)
	}
	return string(b)
}

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }

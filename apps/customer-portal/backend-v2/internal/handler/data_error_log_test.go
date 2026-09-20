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

package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
)

// TestSummarizeErr_NamesUnusableUpstreamData is the regression test for a real
// diagnosis failure. ProcessLicenseDownload can fail three ways that are not
// transport errors at all: a PENDING application with no name, a response with
// no application id, a status outside the known set. As plain errors they fell
// through every case in summarizeErr and logged as "upstream request failed" —
// which says the request failed when in fact it succeeded and the data was
// unusable, sending whoever is debugging to look for an outage.
func TestSummarizeErr_NamesUnusableUpstreamData(t *testing.T) {
	err := apierror.NewDataError("application is PENDING but the licensing service supplied no name/description for project %s", "proj-1")

	got := summarizeErr(err)
	if got == "upstream request failed" {
		t.Fatal("logged as a transport failure; the request succeeded and the data was unusable")
	}
	if !strings.Contains(got, "proj-1") || !strings.Contains(got, "PENDING") {
		t.Errorf("summarizeErr = %q, want the specific reason", got)
	}
}

// TestSummarizeErr_UnwrapsAWrappedDataError: callers wrap with %w as they
// return, so the type must still be found through the chain.
func TestSummarizeErr_UnwrapsAWrappedDataError(t *testing.T) {
	inner := apierror.NewDataError("no application id for project %s", "proj-2")
	wrapped := fmt.Errorf("productconsumption: get deployment license: %w", inner)

	if got := summarizeErr(wrapped); !strings.Contains(got, "proj-2") {
		t.Errorf("summarizeErr = %q, want the wrapped reason", got)
	}
}

// TestDataError_DoesNotLeakToTheClient: the gain is in the log. A DataError is
// not an *apierror.Error, so mapUpstreamError still falls through to the
// handler's own generic message and the upstream's internals stay internal.
func TestDataError_DoesNotLeakToTheClient(t *testing.T) {
	rec := httptest.NewRecorder()
	mapUpstreamError(rec, apierror.NewDataError("licensing service said something internal"), "Failed to retrieve license.")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 — the client contract is unchanged", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "something internal") {
		t.Error("the upstream reason reached the client; it belongs only in the log")
	}
	if !strings.Contains(rec.Body.String(), "Failed to retrieve license.") {
		t.Errorf("body = %s, want the handler's own message", rec.Body.String())
	}
}

// TestSummarizeErr_StillHandlesTransportErrors guards the ordering: the
// DataError check runs first, and must not swallow the categories that were
// already handled.
func TestSummarizeErr_StillHandlesTransportErrors(t *testing.T) {
	if got := summarizeErr(errors.New("something else entirely")); got != "upstream request failed" {
		t.Errorf("plain error = %q, want the existing fallback", got)
	}
	apiErr := &apierror.Error{StatusCode: http.StatusForbidden, Body: "nope"}
	if got := summarizeErr(apiErr); !strings.Contains(got, "403") {
		t.Errorf("apierror = %q, want the upstream status", got)
	}
}

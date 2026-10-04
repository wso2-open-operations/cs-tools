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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/apierror"
)

// assertCloudStatusEnvelope checks the legacy-compatible wrapper and,
// more importantly, that `data` inside it is byte-for-byte what upstream
// sent.
//
// The inner bytes matter more than the wrapper: `availability` is a JSON
// STRING for six clouds and a bare NUMBER for asgardeo, and anything that
// decoded and re-encoded the payload would flatten that distinction. The
// handler splices the raw bytes in for exactly this reason.
func assertCloudStatusEnvelope(t *testing.T, rec *httptest.ResponseRecorder, cloud string, wantData []byte) {
	t.Helper()

	var env struct {
		Result struct {
			Code    int             `json:"code"`
			Message string          `json:"message"`
			Cloud   string          `json:"cloud"`
			Data    json.RawMessage `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response is not the legacy envelope: %v\nbody: %s", err, rec.Body.String())
	}
	if env.Result.Code != 0 || env.Result.Message != "success" {
		t.Errorf("code/message = %d/%q, want 0/\"success\" -- the dashboard gates on this",
			env.Result.Code, env.Result.Message)
	}
	if env.Result.Cloud != cloud {
		t.Errorf("cloud = %q, want %q", env.Result.Cloud, cloud)
	}
	if string(env.Result.Data) != string(wantData) {
		t.Errorf("data was not forwarded verbatim:\n got %s\nwant %s", env.Result.Data, wantData)
	}
}

type fakeCloudStatusClient struct {
	monitors  []byte
	incidents []byte
	avail     []byte
	availHist []byte
	err       error
	gotCloud  string
	detail    []byte
	gotID     string
}

func (f *fakeCloudStatusClient) GetCloudStatusMonitors(_ context.Context, cloud string) ([]byte, error) {
	f.gotCloud = cloud
	return f.monitors, f.err
}

func (f *fakeCloudStatusClient) GetCloudStatusIncidents(_ context.Context, cloud string) ([]byte, error) {
	f.gotCloud = cloud
	return f.incidents, f.err
}

func (f *fakeCloudStatusClient) GetCloudStatusAvailabilities(_ context.Context, cloud string) ([]byte, error) {
	f.gotCloud = cloud
	return f.avail, f.err
}

func (f *fakeCloudStatusClient) GetCloudStatusAvailabilityHistory(_ context.Context, cloud string) ([]byte, error) {
	f.gotCloud = cloud
	return f.availHist, f.err
}

func (f *fakeCloudStatusClient) GetCloudStatusIncidentDetail(_ context.Context, id, cloud string) ([]byte, error) {
	f.gotCloud = cloud
	f.gotID = id
	return f.detail, f.err
}

// TestCloudStatus_ForwardsCloudAndBodyVerbatim is the whole contract: the
// upstream payload reaches the dashboard untouched, inside the envelope
// that makes these endpoints a drop-in replacement for the legacy endpoints.
func TestCloudStatus_ForwardsCloudAndBodyVerbatim(t *testing.T) {
	body := []byte(`{"cp - eu":[{"display_name":"Login","subgroups":[]}]}`)
	f := &fakeCloudStatusClient{monitors: body}
	h := NewCloudStatusHandler(f)

	rec := httptest.NewRecorder()
	h.GetMonitors(rec, httptest.NewRequest(http.MethodGet, "/cloud-status/monitors?cloud=choreo-eu", nil))

	if f.gotCloud != "choreo-eu" {
		t.Errorf("cloud forwarded as %q, want %q", f.gotCloud, "choreo-eu")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	assertCloudStatusEnvelope(t, rec, "choreo-eu", body)
}

// TestCloudStatus_MissingCloudIsForwarded: validation belongs upstream. The
// entity service owns the list of valid clouds, and duplicating it here would
// give two places to update when one is added.
func TestCloudStatus_MissingCloudIsForwarded(t *testing.T) {
	f := &fakeCloudStatusClient{incidents: []byte(`{}`)}
	h := NewCloudStatusHandler(f)

	rec := httptest.NewRecorder()
	h.GetIncidents(rec, httptest.NewRequest(http.MethodGet, "/cloud-status/incidents", nil))

	if f.gotCloud != "" {
		t.Errorf("expected the empty cloud to be forwarded, got %q", f.gotCloud)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d — this handler must not invent its own validation", rec.Code)
	}
}

// TestCloudStatus_UpstreamErrorIsMapped keeps a 400 from upstream a 400 here,
// rather than becoming a 500.
func TestCloudStatus_UpstreamErrorIsMapped(t *testing.T) {
	f := &fakeCloudStatusClient{err: &apierror.Error{StatusCode: http.StatusBadRequest, Body: "unknown cloud"}}
	h := NewCloudStatusHandler(f)

	rec := httptest.NewRecorder()
	h.GetMonitors(rec, httptest.NewRequest(http.MethodGet, "/cloud-status/monitors?cloud=nonsense", nil))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want the upstream 400 preserved", rec.Code)
	}
}

// TestCloudStatus_TransportErrorIsNotA200 guards the obvious regression.
func TestCloudStatus_TransportErrorIsNotA200(t *testing.T) {
	f := &fakeCloudStatusClient{err: errors.New("dial tcp: connection refused")}
	h := NewCloudStatusHandler(f)

	rec := httptest.NewRecorder()
	h.GetIncidents(rec, httptest.NewRequest(http.MethodGet, "/cloud-status/incidents?cloud=choreo", nil))

	if rec.Code == http.StatusOK {
		t.Error("a transport failure must not be reported as success")
	}
}

// TestCloudStatusAvailabilities_ForwardVerbatim covers the two endpoints
// added for the availabilities port, and specifically the thing a
// pass-through is most likely to break: the availability field is a JSON
// STRING for six clouds and a bare NUMBER for asgardeo, and anything that
// decoded and re-encoded the body would flatten that distinction.
func TestCloudStatusAvailabilities_ForwardVerbatim(t *testing.T) {
	t.Run("availabilities keep their mixed wire types", func(t *testing.T) {
		body := []byte(`{"us":[{"availability":100,"duration":"Last 7 days"}],` +
			`"cp":[{"availability":"99.822","duration":"Last 7 days"}]}`)
		f := &fakeCloudStatusClient{avail: body}
		rec := httptest.NewRecorder()
		NewCloudStatusHandler(f).GetAvailabilities(rec,
			httptest.NewRequest(http.MethodGet, "/cloud-status/availabilities?cloud=asgardeo", nil))

		if f.gotCloud != "asgardeo" {
			t.Errorf("cloud forwarded as %q, want asgardeo", f.gotCloud)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		assertCloudStatusEnvelope(t, rec, "asgardeo", body)
	})

	t.Run("history forwards verbatim", func(t *testing.T) {
		body := []byte(`{"us":[{"display_name":"Login","subgroups":` +
			`[{"display_name":"Console","history":[{"availability":100,"date":"2026-09-29"}]}]}]}`)
		f := &fakeCloudStatusClient{availHist: body}
		rec := httptest.NewRecorder()
		NewCloudStatusHandler(f).GetAvailabilityHistory(rec,
			httptest.NewRequest(http.MethodGet, "/cloud-status/availability-history?cloud=moesif", nil))

		if f.gotCloud != "moesif" {
			t.Errorf("cloud forwarded as %q, want moesif", f.gotCloud)
		}
		assertCloudStatusEnvelope(t, rec, "moesif", body)
	})

	t.Run("upstream failure is mapped, not masked", func(t *testing.T) {
		f := &fakeCloudStatusClient{err: &apierror.Error{StatusCode: http.StatusBadRequest, Body: "unknown cloud"}}
		rec := httptest.NewRecorder()
		NewCloudStatusHandler(f).GetAvailabilities(rec,
			httptest.NewRequest(http.MethodGet, "/cloud-status/availabilities?cloud=choreo", nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want the upstream 400 propagated", rec.Code)
		}
	})
}

// TestCloudStatusIncidentDetail_ForwardsBothShapes covers the endpoint's two
// payloads. The sparse one is the common case -- an outage whose incident
// does not qualify carries only `attachments` -- and a pass-through that
// "helpfully" filled in the missing keys would change what the page renders.
func TestCloudStatusIncidentDetail_ForwardsBothShapes(t *testing.T) {
	t.Run("full detail", func(t *testing.T) {
		body := []byte(`{"id":"7af3a9683bab839091404c6aa5e45a10","begin":"2026-09-22 09:26:39",` +
			`"end":"","type":"Degraded","status":"In Progress","short_description":"x",` +
			`"comments":[],"attachments":[]}`)
		f := &fakeCloudStatusClient{detail: body}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet,
			"/cloud-status/incidents/7af3a9683bab839091404c6aa5e45a10?cloud=asgardeo", nil)
		req.SetPathValue("id", "7af3a9683bab839091404c6aa5e45a10")
		NewCloudStatusHandler(f).GetIncidentDetail(rec, req)

		if f.gotID != "7af3a9683bab839091404c6aa5e45a10" {
			t.Errorf("id forwarded as %q", f.gotID)
		}
		assertCloudStatusEnvelope(t, rec, "asgardeo", body)
	})

	t.Run("attachments-only stays sparse", func(t *testing.T) {
		body := []byte(`{"attachments":[]}`)
		f := &fakeCloudStatusClient{detail: body}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/cloud-status/incidents/x?cloud=choreo", nil)
		req.SetPathValue("id", "x")
		NewCloudStatusHandler(f).GetIncidentDetail(rec, req)
		assertCloudStatusEnvelope(t, rec, "choreo", body)
	})
}

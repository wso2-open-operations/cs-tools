// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package csm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newSearchTestClient serves searchResponse for every /incidents/search and keeps the last request body.
func newSearchTestClient(t *testing.T, searchResponse string) (*Client, *searchIncidentsRequest) {
	t.Helper()
	var last searchIncidentsRequest
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			_, _ = io.WriteString(w, `{"access_token":"t","token_type":"bearer","expires_in":3600}`)
		case "/incidents/search":
			_ = json.NewDecoder(r.Body).Decode(&last)
			_, _ = io.WriteString(w, searchResponse)
		default:
			http.NotFound(w, r)
		}
	}))
	// The client only speaks HTTPS; trust the test server's certificate for this run.
	saved := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	t.Cleanup(func() {
		srv.Client().CloseIdleConnections()
		http.DefaultTransport = saved
		srv.Close()
	})
	return NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s"}), &last
}

const (
	testID     = "8902587d-eb73-8b10-fcf5-f5dabad0cd60"
	testNumber = "INC0100206"
)

func TestIncidentState_ReadsTheMatchingIncident(t *testing.T) {
	client, last := newSearchTestClient(t, `{"incidents":[{"id":"`+testID+`","number":"`+testNumber+`","state":"CLOSED"}],"total":1}`)

	open, found, err := client.IncidentState(context.Background(), testID, testNumber)
	if err != nil || !found || open {
		t.Fatalf("open=%v found=%v err=%v; want a found, closed incident", open, found, err)
	}
	if last.Filters.Number != testNumber {
		t.Errorf("searched number %q, want %q", last.Filters.Number, testNumber)
	}
}

func TestIncidentState_NoRowIsNotFound(t *testing.T) {
	client, _ := newSearchTestClient(t, `{"incidents":[],"total":0}`)

	_, found, err := client.IncidentState(context.Background(), testID, testNumber)
	if err != nil || found {
		t.Fatalf("found=%v err=%v; want a clean miss", found, err)
	}
}

// A backend that drops the number filter returns some other incident; its state must not be taken as this one's.
func TestIncidentState_AnotherIncidentIsAnError(t *testing.T) {
	cases := map[string]string{
		"other number": `{"incidents":[{"id":"` + testID + `","number":"INC0040603","state":"CLOSED"}],"total":88257}`,
		"other id":     `{"incidents":[{"id":"12e6082d-1b38-4710-a002-c9d3604bcbdf","number":"` + testNumber + `","state":"CLOSED"}],"total":1}`,
	}
	for name, resp := range cases {
		t.Run(name, func(t *testing.T) {
			client, _ := newSearchTestClient(t, resp)

			_, found, err := client.IncidentState(context.Background(), testID, testNumber)
			if err == nil || found {
				t.Fatalf("found=%v err=%v; want an error, not another incident's state", found, err)
			}
		})
	}
}

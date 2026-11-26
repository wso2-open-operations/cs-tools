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
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// Upstream fixture: product version rows as entity-service returns them
// (RFC 3339 dates, upper-case status).
//
//   - 758: APIM 3.1.0, six profile rows (stored out of order), carbon only on
//     one row, linked twice to the same release line.
//   - 1072: Message Broker 2.2.0, no status and no profile (the legacy row
//     that returned currentStatusOfSupport and deploymentProfile null).
//   - 1000 / 1001: same release date (tie -> id ascending); 1000 is tagged
//     OB, uses the public name/abbreviation overrides and an unknown profile.
//   - a row without publicId and a retired row, never returned.
const legacyFixtureVersions = `{"productVersions":[
 {"productName":"WSO2 API Manager","productCode":"wso2am","version":"3.1.0","deploymentProfile":"Developer Portal","currentSupportStatus":"EXTENDED","releaseDate":"2020-04-07T00:00:00Z","earliestPossibleSupportEolDate":null,"supportEolDate":"2024-04-07T00:00:00Z","supportEolAnnouncementDate":"2023-04-07T00:00:00Z","publicId":758,"publicName":"API Manager","publicAbbreviation":null,"description":"WSO2 API Manager 3.1.0 & more","isPublic":true,"showInSupportMatrix":true,"tags":[],"releaseLinks":null,"legacyStatus":null,"releaseLineId":"rl-1","releaseLineName":"APIM-4.x","carbon":null,"retiredOn":null},
 {"productName":"WSO2 API Manager","productCode":"wso2am","version":"3.1.0","deploymentProfile":"All In One","currentSupportStatus":"EXTENDED","releaseDate":"2020-04-07T00:00:00Z","earliestPossibleSupportEolDate":null,"supportEolDate":"2024-04-07T00:00:00Z","supportEolAnnouncementDate":"2023-04-07T00:00:00Z","publicId":758,"publicName":"API Manager","publicAbbreviation":null,"description":null,"isPublic":true,"showInSupportMatrix":true,"tags":[],"releaseLinks":null,"legacyStatus":null,"releaseLineId":"rl-1","releaseLineName":"APIM-4.x","carbon":{"platformRelease":"Wilkinson","platformVersion":"4.6.0","carbonKernelVersion":"4.6.0","releaseChunk":null,"kernelPatchNo":null},"retiredOn":null},
 {"productName":"WSO2 API Manager","productCode":"wso2am","version":"3.1.0","deploymentProfile":"Traffic Manager","currentSupportStatus":"EXTENDED","releaseDate":"2020-04-07T00:00:00Z","supportEolDate":"2024-04-07T00:00:00Z","supportEolAnnouncementDate":"2023-04-07T00:00:00Z","publicId":758,"publicName":"API Manager","isPublic":true,"showInSupportMatrix":true,"tags":null,"releaseLineId":null,"carbon":null,"retiredOn":null},
 {"productName":"WSO2 API Manager","productCode":"wso2am","version":"3.1.0","deploymentProfile":"Gateway worker","currentSupportStatus":"EXTENDED","releaseDate":"2020-04-07T00:00:00Z","supportEolDate":"2024-04-07T00:00:00Z","supportEolAnnouncementDate":"2023-04-07T00:00:00Z","publicId":758,"publicName":"API Manager","isPublic":true,"showInSupportMatrix":true,"tags":null,"releaseLineId":null,"carbon":null,"retiredOn":null},
 {"productName":"WSO2 API Manager","productCode":"wso2am","version":"3.1.0","deploymentProfile":"API Publisher","currentSupportStatus":"EXTENDED","releaseDate":"2020-04-07T00:00:00Z","supportEolDate":"2024-04-07T00:00:00Z","supportEolAnnouncementDate":"2023-04-07T00:00:00Z","publicId":758,"publicName":"API Manager","isPublic":true,"showInSupportMatrix":true,"tags":null,"releaseLineId":null,"carbon":null,"retiredOn":null},
 {"productName":"WSO2 API Manager","productCode":"wso2am","version":"3.1.0","deploymentProfile":"Key Manager","currentSupportStatus":"EXTENDED","releaseDate":"2020-04-07T00:00:00Z","supportEolDate":"2024-04-07T00:00:00Z","supportEolAnnouncementDate":"2023-04-07T00:00:00Z","publicId":758,"publicName":"API Manager","isPublic":true,"showInSupportMatrix":true,"tags":null,"releaseLineId":null,"carbon":null,"retiredOn":null},
 {"productName":"WSO2 Message Broker","productCode":"wso2mb","version":"2.2.0","deploymentProfile":null,"currentSupportStatus":null,"releaseDate":"2014-05-23T00:00:00Z","publicId":1072,"publicName":"Message Broker","description":"WSO2 Message Broker 2.2.0","isPublic":true,"showInSupportMatrix":false,"tags":null,"releaseLinks":"P2 Repo","legacyStatus":true,"releaseLineId":null,"carbon":{"platformRelease":"Turing","platformVersion":"4.2.0","carbonKernelVersion":"4.2.0","releaseChunk":"Chunk 11","kernelPatchNo":"Patch0008"},"retiredOn":null},
 {"productName":"WSO2 Open Banking","productCode":"wso2ob","version":"3.0.0","deploymentProfile":"Zeta","currentSupportStatus":"DISCONTINUED","releaseDate":"2026-08-07T00:00:00Z","publicId":1000,"publicName":"Open Banking","publicAbbreviation":"wso2-ob","isPublic":false,"showInSupportMatrix":true,"tags":["OB"],"legacyStatus":false,"releaseLineId":"rl-2","releaseLineName":"New-1.x","carbon":null,"retiredOn":null},
 {"productName":"WSO2 Open Banking","productCode":"wso2ob","version":"3.0.0","deploymentProfile":"Analytics","currentSupportStatus":"DISCONTINUED","releaseDate":"2026-08-07T00:00:00Z","publicId":1000,"publicName":"Open Banking","publicAbbreviation":"wso2-ob","isPublic":false,"showInSupportMatrix":true,"tags":["OB"],"legacyStatus":false,"releaseLineId":"rl-3","releaseLineName":"Old-1.x","carbon":null,"retiredOn":null},
 {"productName":"WSO2 Open Banking","productCode":"wso2ob","version":"3.0.0","deploymentProfile":"All In One","currentSupportStatus":"DISCONTINUED","releaseDate":"2026-08-07T00:00:00Z","publicId":1000,"publicName":"Open Banking","publicAbbreviation":"wso2-ob","isPublic":false,"showInSupportMatrix":true,"tags":["OB"],"legacyStatus":false,"releaseLineId":null,"carbon":null,"retiredOn":null},
 {"productName":"WSO2 Identity Server","productCode":"wso2is","version":"7.2.0","deploymentProfile":"All In One","currentSupportStatus":"AVAILABLE","releaseDate":"2026-08-07T00:00:00Z","publicId":1001,"isPublic":true,"showInSupportMatrix":true,"tags":null,"retiredOn":null},
 {"productName":"WSO2 Identity Server","productCode":"wso2is","version":"7.3.0","deploymentProfile":"All In One","currentSupportStatus":"AVAILABLE","releaseDate":"2027-01-01T00:00:00Z","publicId":null,"isPublic":true,"showInSupportMatrix":true,"retiredOn":null},
 {"productName":"WSO2 Identity Server","productCode":"wso2is","version":"5.0.0","deploymentProfile":"All In One","currentSupportStatus":"DISCONTINUED","releaseDate":"2014-01-01T00:00:00Z","publicId":999,"isPublic":true,"showInSupportMatrix":true,"retiredOn":"2026-10-01T00:00:00Z"}
]}`

const legacyFixtureReleaseLines = `{"releaseLines":[
 {"id":"rl-2","name":"New-1.x","startDate":"2026-05-19T00:00:00Z","status":"Active","legacyId":null,"retiredOn":null},
 {"id":"rl-1","name":"APIM-4.x","startDate":"2024-04-10T00:00:00Z","status":"Active","legacyId":3,"retiredOn":null},
 {"id":"rl-4","name":"Integrator-4.x","startDate":null,"status":null,"legacyId":1,"retiredOn":null},
 {"id":"rl-3","name":"Old-1.x","startDate":"2020-01-02T00:00:00Z","status":"EOL","legacyId":9,"retiredOn":"2026-10-01T00:00:00Z"}
]}`

// Carbon platform history: release_date_text is the legacy M/d/yyyy text.
const legacyFixtureCarbon = `{"items":[
 {"id":1,"carbonVersion":"4.1.7","releaseDate":null,"releaseDateText":null,"messageBrokerVersion":"2.1.1"},
 {"id":2,"carbonVersion":"4.1.5","releaseDate":null,"releaseDateText":null,"apiManagerVersion":"1.5.0"},
 {"id":9,"carbonVersion":"4.0.6","releaseDate":"2013-01-29","releaseDateText":"1/29/2013","esbVersion":"4.6.0"},
 {"id":10,"carbonVersion":"4.0.5","releaseDate":"2012-12-19","releaseDateText":"12/19/2012","apiManagerVersion":"1.3.0"},
 {"id":21,"carbonVersion":"3.0.1","releaseDate":"2010-09-08","releaseDateText":"9/8/2010","identityServerVersion":"2.0.1"},
 {"id":28,"carbonVersion":"1.5.0","releaseDate":"2009-06-02","releaseDateText":"6/2/2009","esbVersion":"2.0.0, 2.0.2"}
]}`

const legacyFixtureKernels = `{"items":[
 {"id":1,"kernelRelease":"5.0.0","releaseDate":"2015-12-21","carbonPlatform":"Hamming"},
 {"id":2,"kernelRelease":"4.4.0","releaseDate":"2015-03-30","carbonPlatform":"Wilkes"},
 {"id":3,"kernelRelease":"4.3.0","releaseDate":"2014-12-20","carbonPlatform":"Perlis"},
 {"id":4,"kernelRelease":"4.2.0","releaseDate":"2013-09-02","carbonPlatform":"Turing"},
 {"id":5,"kernelRelease":"4.4.1","releaseDate":"2015-06-01","carbonPlatform":"Wilkes"}
]}`

// legacyFixtureClient serves the fixtures and records the feeds and
// includeRetired values it was asked for.
func legacyFixtureClient(feeds *[]string, retired *[]bool) *mockEntityProductAPIClient {
	return &mockEntityProductAPIClient{
		versionsFn: func(_ context.Context, feed string) ([]byte, error) {
			if feeds != nil {
				*feeds = append(*feeds, feed)
			}
			return []byte(legacyFixtureVersions), nil
		},
		releaseLinesFn: func(_ context.Context, includeRetired bool) ([]byte, error) {
			if retired != nil {
				*retired = append(*retired, includeRetired)
			}
			return []byte(legacyFixtureReleaseLines), nil
		},
		carbonFn:  func(context.Context) ([]byte, error) { return []byte(legacyFixtureCarbon), nil },
		kernelsFn: func(context.Context) ([]byte, error) { return []byte(legacyFixtureKernels), nil },
	}
}

// legacyServe routes a request through a real ServeMux with the handler's
// own patterns, so route specificity is part of every test.
func legacyServe(t *testing.T, client entityProductAPIClient, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewProductAPIHandler(client).RegisterRoutes(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(method, target, nil))
	return w
}

func legacyIDs(t *testing.T, w *httptest.ResponseRecorder) []int64 {
	t.Helper()
	rows := decodeJSON[[]struct {
		ID int64 `json:"id"`
	}](t, w)
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

// TestLegacyGetProductExactBody pins one row byte for byte: key names and
// order, yyyy-M-d dates, Title-case status, profile order, carbon from the
// one row that has it, de-duplicated release line, null tags, no HTML
// escaping.
func TestProductAPIGetProductExactBody(t *testing.T) {
	w := legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, "/products/758")
	assertStatus(t, w, http.StatusOK)
	assertContentType(t, w, "application/json")
	want := `{"abbreviation":"wso2am","active":1,"carbonPlatform":{"platformRelease":"Wilkinson","platformVersion":"4.6.0","carbonKernelVersion":"4.6.0","releaseChunk":null,"kernelPatchNo":null},"currentStatusOfSupport":"Extended","deploymentProfile":["All In One","Gateway worker","Key Manager","Traffic Manager","API Publisher","Developer Portal"],"description":"WSO2 API Manager 3.1.0 & more","earliestPossibleSupportEolDate":null,"id":758,"isPublic":true,"productGroup":null,"productName":"API Manager","productVersion":"3.1.0","releaseDate":"2020-4-7","releaseLines":[{"releaseLineId":3,"releaseLine":"APIM-4.x","releaseLineStartDate":"2024-4-10","releaseLineStatus":"Active"}],"releaseLinks":null,"showInSupportMatrix":true,"status":true,"supportEolAnnouncementDate":"2023-4-7","supportEolDate":"2024-4-7","tags":null}`
	if got := w.Body.String(); got != want {
		t.Errorf("body mismatch\n got %s\nwant %s", got, want)
	}
}

// TestLegacyRowWithoutStatusOrProfile matches legacy's Message Broker 2.2.0
// row: null status, null deploymentProfile, status from legacyStatus.
func TestProductAPIRowWithoutStatusOrProfile(t *testing.T) {
	w := legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, "/products/1072")
	assertStatus(t, w, http.StatusOK)
	want := `{"abbreviation":"wso2mb","active":1,"carbonPlatform":{"platformRelease":"Turing","platformVersion":"4.2.0","carbonKernelVersion":"4.2.0","releaseChunk":"Chunk 11","kernelPatchNo":"Patch0008"},"currentStatusOfSupport":null,"deploymentProfile":null,"description":"WSO2 Message Broker 2.2.0","earliestPossibleSupportEolDate":null,"id":1072,"isPublic":true,"productGroup":null,"productName":"Message Broker","productVersion":"2.2.0","releaseDate":"2014-5-23","releaseLines":[],"releaseLinks":"P2 Repo","showInSupportMatrix":false,"status":true,"supportEolAnnouncementDate":null,"supportEolDate":null,"tags":null}`
	if got := w.Body.String(); got != want {
		t.Errorf("body mismatch\n got %s\nwant %s", got, want)
	}
}

// TestLegacyOverridesAndLines covers the public name/abbreviation overrides,
// legacyStatus false, unknown profiles sorting last, empty carbon object, a
// CSM-only release line (null id) and a retired line still rendered.
func TestProductAPIOverridesAndLines(t *testing.T) {
	w := legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, "/products/1000")
	assertStatus(t, w, http.StatusOK)
	want := `{"abbreviation":"wso2-ob","active":1,"carbonPlatform":{"platformRelease":null,"platformVersion":null,"carbonKernelVersion":null,"releaseChunk":null,"kernelPatchNo":null},"currentStatusOfSupport":"Discontinued","deploymentProfile":["All In One","Analytics","Zeta"],"description":null,"earliestPossibleSupportEolDate":null,"id":1000,"isPublic":false,"productGroup":null,"productName":"Open Banking","productVersion":"3.0.0","releaseDate":"2026-8-7","releaseLines":[{"releaseLineId":null,"releaseLine":"New-1.x","releaseLineStartDate":"2026-5-19","releaseLineStatus":"Active"},{"releaseLineId":9,"releaseLine":"Old-1.x","releaseLineStartDate":"2020-1-2","releaseLineStatus":"EOL"}],"releaseLinks":null,"showInSupportMatrix":true,"status":false,"supportEolAnnouncementDate":null,"supportEolDate":null,"tags":["OB"]}`
	if got := w.Body.String(); got != want {
		t.Errorf("body mismatch\n got %s\nwant %s", got, want)
	}
}

// TestLegacyProductLists checks feeds, grouping, order and DTO shape of every
// product list endpoint.
// TestLegacyAbbreviationLowestIDAndTrailingSpace: when listings share an
// abbreviation the lowest id wins, and trailing spaces never matter, as in
// legacy's MySQL lookup. Names are not HTML-escaped.
func TestProductAPIAbbreviationLowestIDAndTrailingSpace(t *testing.T) {
	versions := `{"productVersions":[
 {"productName":"WSO2 X","productCode":"wso2x","version":"2.0.0","deploymentProfile":"All In One","releaseDate":"2024-01-01T00:00:00Z","publicId":900,"publicName":"X Newer","publicAbbreviation":"wso2x","isPublic":true,"showInSupportMatrix":true,"retiredOn":null},
 {"productName":"WSO2 X","productCode":"wso2x","version":"1.0.0","deploymentProfile":"All In One","releaseDate":"2020-01-01T00:00:00Z","publicId":800,"publicName":"X & Older","publicAbbreviation":"wso2x ","isPublic":true,"showInSupportMatrix":true,"retiredOn":null}
]}`
	client := legacyFixtureClient(nil, nil)
	client.versionsFn = func(context.Context, string) ([]byte, error) { return []byte(versions), nil }
	w := legacyServe(t, client, http.MethodGet, "/productNameForAbbreviation?abbrList=WSO2X,wso2x%20%20")
	assertStatus(t, w, http.StatusOK)
	if got, want := w.Body.String(), `{"WSO2X":"X & Older","wso2x":"X & Older"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestProductAPIProductLists(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		wantFeed    string
		wantIDs     []int64
		wantToggles bool
	}{
		{"products", "/products", "", []int64{1000, 1001, 758, 1072}, true},
		{"support matrix", "/products/support-matrix", "support-matrix", []int64{1000, 1001, 758, 1072}, false},
		{"release matrix", "/products/release-matrix", "release-matrix", []int64{1000, 1001, 758, 1072}, false},
		{"inclusion", "/products?inclusion=ob", "", []int64{1000}, true},
		{"exclusion", "/products?exclusion=%20OB%20,x", "", []int64{1001, 758, 1072}, true},
		{"empty filters", "/products?inclusion=&exclusion=,", "", []int64{1000, 1001, 758, 1072}, true},
		{"matrix with inclusion", "/products/support-matrix?inclusion=OB", "support-matrix", []int64{1000}, false},
		{"search prefix, case-insensitive", "/searchProduct?key=api", "", []int64{758}, true},
		{"search empty key, id order", "/searchProduct?key=", "", []int64{758, 1000, 1001, 1072}, true},
		{"search with exclusion", "/searchProduct?key=Open&exclusion=OB", "", []int64{}, true},
		{"all versions exact name", "/getAllProductVersion?product=message%20broker%20%20", "", []int64{1072}, true},
		{"all versions no prefix match", "/getAllProductVersion?product=Message", "", []int64{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var feeds []string
			var retired []bool
			w := legacyServe(t, legacyFixtureClient(&feeds, &retired), http.MethodGet, tc.target)
			assertStatus(t, w, http.StatusOK)
			body := w.Body.String()
			if !reflect.DeepEqual(legacyIDs(t, w), tc.wantIDs) {
				t.Errorf("ids = %s, want %v", body, tc.wantIDs)
			}
			if len(feeds) != 1 || feeds[0] != tc.wantFeed {
				t.Errorf("feeds = %q, want [%q]", feeds, tc.wantFeed)
			}
			if len(retired) != 1 || !retired[0] {
				t.Errorf("release lines includeRetired = %v, want [true]", retired)
			}
			if len(tc.wantIDs) > 0 {
				if got := strings.Contains(body, `"isPublic"`); got != tc.wantToggles {
					t.Errorf("isPublic present = %v, want %v", got, tc.wantToggles)
				}
				if got := strings.Contains(body, `"showInSupportMatrix"`); got != tc.wantToggles {
					t.Errorf("showInSupportMatrix present = %v, want %v", got, tc.wantToggles)
				}
			}
			if body == "null" {
				t.Error("empty list must be [], not null")
			}
		})
	}
}

func TestProductAPIShouldInclude(t *testing.T) {
	tests := []struct {
		name                       string
		tags, exclusion, inclusion []string
		want                       bool
	}{
		{"no filters", nil, nil, nil, true},
		{"untagged, exclusion only", nil, []string{"OB"}, nil, true},
		{"untagged, inclusion", nil, nil, []string{"OB"}, false},
		{"excluded", []string{"OB"}, []string{"ob"}, nil, false},
		{"excluded wins over included", []string{"OB"}, []string{"OB"}, []string{"OB"}, false},
		{"included", []string{"x", " OB "}, nil, []string{"ob"}, true},
		{"not included", []string{"x"}, nil, []string{"OB"}, false},
		{"exclusion miss", []string{"x"}, []string{"OB"}, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := legacyShouldInclude(tc.tags, tc.exclusion, tc.inclusion); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProductAPIDateAndStatus(t *testing.T) {
	s := func(v string) *string { return &v }
	tests := []struct {
		in   *string
		want *string
	}{
		{nil, nil},
		{s("2020-04-07"), s("2020-4-7")},
		{s("2024-12-21"), s("2024-12-21")},
		{s("2026-01-05"), s("2026-1-5")},
		{s("not a date"), s("not a date")},
	}
	for _, tc := range tests {
		got := legacyDate(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("legacyDate(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
	for in, want := range map[string]string{"available": "Available", "extended": "Extended", "deprecated": "Deprecated", "discontinued": "Discontinued", "AVAILABLE": "Available", "DISCONTINUED": "Discontinued"} {
		if got := legacyStatus(s(in)); got == nil || *got != want {
			t.Errorf("legacyStatus(%q) = %v, want %q", in, got, want)
		}
	}
	if legacyStatus(nil) != nil {
		t.Error("legacyStatus(nil) must stay nil")
	}
}

func TestProductAPIProfileOrder(t *testing.T) {
	got := []string{"Analytics", "Zeta", "Micro-Integrator", "Alpha", "Integrator", "Message Broker", "Business Process", "All In One"}
	want := []string{"All In One", "Integrator", "Business Process", "Message Broker", "Micro-Integrator", "Analytics", "Alpha", "Zeta"}
	sortProfiles := append([]string(nil), got...)
	for i := range sortProfiles {
		for j := i + 1; j < len(sortProfiles); j++ {
			if compareProfiles(sortProfiles[j], sortProfiles[i]) < 0 {
				sortProfiles[i], sortProfiles[j] = sortProfiles[j], sortProfiles[i]
			}
		}
	}
	if !reflect.DeepEqual(sortProfiles, want) {
		t.Errorf("order = %v, want %v", sortProfiles, want)
	}
}

// TestLegacyEmptyBodyStatuses: legacy answered these with no body.
func TestProductAPIEmptyBodyStatuses(t *testing.T) {
	tests := []struct {
		target string
		want   int
	}{
		{"/products/999999", http.StatusNotFound},
		{"/products/999", http.StatusNotFound}, // retired
		{"/products/0", http.StatusBadRequest},
		{"/products/abc", http.StatusBadRequest},
		{"/deploymentProfile?id=999999", http.StatusNotFound},
		{"/deploymentProfile?id=x", http.StatusBadRequest},
		{"/carbon-platform-history/99", http.StatusNotFound},
		{"/carbon-platform-history/-1", http.StatusBadRequest},
		{"/carbon-platform-history?productType=foo", http.StatusBadRequest},
		{"/carbon-platform-history?productType=", http.StatusBadRequest},
		{"/carbon-platform-history/version/bad_version", http.StatusBadRequest},
		{"/carbon-platform-history/version/123456789012345678901", http.StatusBadRequest},
		{"/carbon-kernel-releases/99", http.StatusNotFound},
		{"/carbon-kernel-releases/zero", http.StatusBadRequest},
		{"/carbon-kernel-releases/platform/bad_platform", http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			w := legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, tc.target)
			assertStatus(t, w, tc.want)
			if w.Body.Len() != 0 {
				t.Errorf("body = %q, want empty", w.Body.String())
			}
		})
	}
}

func TestProductAPIRequiredQueryParams(t *testing.T) {
	for _, target := range []string{"/searchProduct", "/getAllProductVersion", "/productNameForAbbreviation", "/deploymentProfile"} {
		t.Run(target, func(t *testing.T) {
			called := false
			client := &mockEntityProductAPIClient{versionsFn: func(context.Context, string) ([]byte, error) {
				called = true
				return []byte(`{"productVersions":[]}`), nil
			}}
			w := legacyServe(t, client, http.MethodGet, target)
			assertStatus(t, w, http.StatusBadRequest)
			if called {
				t.Error("upstream called despite missing parameter")
			}
		})
	}
}

// TestLegacyProductNameForAbbreviation pins the deliberate fix: every
// requested abbreviation is answered, in request order, null when unknown.
func TestProductAPIProductNameForAbbreviation(t *testing.T) {
	tests := []struct {
		target string
		want   string
	}{
		{"/productNameForAbbreviation?abbrList=wso2am,wso2is,wso2mi", `{"wso2am":"API Manager","wso2is":"WSO2 Identity Server","wso2mi":null}`},
		{"/productNameForAbbreviation?abbrList=WSO2-OB&abbrList=wso2mb,%20wso2am%20,wso2mb", `{"WSO2-OB":"Open Banking","wso2mb":"Message Broker","wso2am":"API Manager"}`},
		{"/productNameForAbbreviation?abbrList=", `{}`},
	}
	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			w := legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, tc.target)
			assertStatus(t, w, http.StatusOK)
			if got := w.Body.String(); got != tc.want {
				t.Errorf("body = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestProductAPIDeploymentProfile(t *testing.T) {
	tests := []struct {
		target, want string
	}{
		{"/deploymentProfile?id=758", `[{"id":"0","title":"All In One"},{"id":"1","title":"Gateway worker"},{"id":"2","title":"Key Manager"},{"id":"3","title":"Traffic Manager"},{"id":"4","title":"API Publisher"},{"id":"5","title":"Developer Portal"}]`},
		{"/deploymentProfile?id=1072", `[]`},
	}
	for _, tc := range tests {
		w := legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, tc.target)
		assertStatus(t, w, http.StatusOK)
		if got := w.Body.String(); got != tc.want {
			t.Errorf("%s: body = %s, want %s", tc.target, got, tc.want)
		}
	}
}

func TestProductAPIReleaseLinesAndGroups(t *testing.T) {
	var retired []bool
	w := legacyServe(t, legacyFixtureClient(nil, &retired), http.MethodGet, "/release-line")
	assertStatus(t, w, http.StatusOK)
	want := `[{"releaseLineId":1,"releaseLine":"Integrator-4.x","releaseLineStartDate":null,"releaseLineStatus":null},{"releaseLineId":3,"releaseLine":"APIM-4.x","releaseLineStartDate":"2024-4-10","releaseLineStatus":"Active"},{"releaseLineId":null,"releaseLine":"New-1.x","releaseLineStartDate":"2026-5-19","releaseLineStatus":"Active"}]`
	if got := w.Body.String(); got != want {
		t.Errorf("release lines\n got %s\nwant %s", got, want)
	}
	if !reflect.DeepEqual(retired, []bool{false}) {
		t.Errorf("includeRetired = %v, want [false]", retired)
	}

	w = legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, "/product-group")
	assertStatus(t, w, http.StatusOK)
	if got := w.Body.String(); got != "[]" {
		t.Errorf("product-group = %s, want []", got)
	}
}

// TestLegacyCarbonPlatformHistory checks the text-ordered list (legacy sorted
// the M/d/yyyy text DESC), the productType filters and the sub-resources.
func TestProductAPICarbonPlatformHistory(t *testing.T) {
	tests := []struct {
		target  string
		wantIDs []int64
	}{
		{"/carbon-platform-history", []int64{21, 28, 10, 9, 2, 1}},
		{"/carbon-platform-history?productType=API-Manager", []int64{10, 2}},
		{"/carbon-platform-history?productType=identity-server", []int64{21}},
		{"/carbon-platform-history?productType=esb", []int64{28, 9}},
		{"/carbon-platform-history/version/4.0.5", []int64{10}},
		{"/carbon-platform-history/version/9.9.9", []int64{}},
	}
	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			w := legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, tc.target)
			assertStatus(t, w, http.StatusOK)
			if got := legacyIDs(t, w); !reflect.DeepEqual(got, tc.wantIDs) {
				t.Errorf("ids = %v, want %v", got, tc.wantIDs)
			}
		})
	}

	w := legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, "/carbon-platform-history/28")
	assertStatus(t, w, http.StatusOK)
	want := `{"id":28,"carbonVersion":"1.5.0","releaseDate":"2009-6-2","productVersions":{"esbVersion":"2.0.0, 2.0.2","governanceRegistryVersion":null,"applicationServerVersion":null,"dataServicesServerVersion":null,"identityServerVersion":null,"businessProcessServerVersion":null,"mashupServerVersion":null,"gadgetServerVersion":null,"businessActivityMonitorVersion":null,"apiManagerVersion":null,"businessRulesServerVersion":null,"eventServerVersion":null,"messageBrokerVersion":null,"complexEventProcessorVersion":null,"elasticLoadBalancerVersion":null,"storageServerVersion":null,"stratosVersion":null,"cloudGatewayVersion":null,"wsasVersion":null,"appFactoryVersion":null,"userEngagementServerVersion":null}}`
	if got := w.Body.String(); got != want {
		t.Errorf("by id\n got %s\nwant %s", got, want)
	}

	w = legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, "/carbon-platform-history/latest")
	assertStatus(t, w, http.StatusOK)
	if got := decodeJSON[legacyCarbonHistory](t, w); got.ID != 21 {
		t.Errorf("latest id = %d, want 21 (first by text order)", got.ID)
	}

	w = legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, "/carbon-platform-history/versions")
	assertStatus(t, w, http.StatusOK)
	if got := w.Body.String(); got != `["1.5.0","3.0.1","4.0.5","4.0.6","4.1.5","4.1.7"]` {
		t.Errorf("versions = %s", got)
	}

	empty := &mockEntityProductAPIClient{}
	w = legacyServe(t, empty, http.MethodGet, "/carbon-platform-history/latest")
	assertStatus(t, w, http.StatusNotFound)
}

func TestProductAPICarbonDateFromText(t *testing.T) {
	s := func(v string) *string { return &v }
	tests := []struct {
		row  lpCarbonHistory
		want *string
	}{
		{lpCarbonHistory{ReleaseDate: s("2010-09-08"), ReleaseDateText: s("9/8/2010")}, s("2010-9-8")},
		{lpCarbonHistory{ReleaseDateText: s("9/8/2010")}, s("2010-9-8")},
		{lpCarbonHistory{ReleaseDateText: s("2010-09-08T00:00:00")}, s("2010-9-8")},
		{lpCarbonHistory{ReleaseDateText: s("Q3 2010")}, s("Q3")},
		{lpCarbonHistory{}, nil},
	}
	for _, tc := range tests {
		if got := legacyCarbonDate(tc.row); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("legacyCarbonDate(%+v) = %v, want %v", tc.row, got, tc.want)
		}
	}
}

func TestProductAPICarbonKernelReleases(t *testing.T) {
	tests := []struct {
		target string
		want   string
	}{
		{"/carbon-kernel-releases", `[{"id":1,"kernelRelease":"5.0.0","releaseDate":"2015-12-21","carbonPlatform":"Hamming"},{"id":5,"kernelRelease":"4.4.1","releaseDate":"2015-6-1","carbonPlatform":"Wilkes"},{"id":2,"kernelRelease":"4.4.0","releaseDate":"2015-3-30","carbonPlatform":"Wilkes"},{"id":3,"kernelRelease":"4.3.0","releaseDate":"2014-12-20","carbonPlatform":"Perlis"},{"id":4,"kernelRelease":"4.2.0","releaseDate":"2013-9-2","carbonPlatform":"Turing"}]`},
		{"/carbon-kernel-releases/2", `{"id":2,"kernelRelease":"4.4.0","releaseDate":"2015-3-30","carbonPlatform":"Wilkes"}`},
		{"/carbon-kernel-releases/platforms", `["Hamming","Perlis","Turing","Wilkes"]`},
		{"/carbon-kernel-releases/platform/wilkes", `[{"id":5,"kernelRelease":"4.4.1","releaseDate":"2015-6-1","carbonPlatform":"Wilkes"},{"id":2,"kernelRelease":"4.4.0","releaseDate":"2015-3-30","carbonPlatform":"Wilkes"}]`},
		{"/carbon-kernel-releases/platform/Nope", `[]`},
		{"/carbon-kernel-releases/latest-by-platform", `[{"id":1,"kernelRelease":"5.0.0","releaseDate":"2015-12-21","carbonPlatform":"Hamming"},{"id":3,"kernelRelease":"4.3.0","releaseDate":"2014-12-20","carbonPlatform":"Perlis"},{"id":4,"kernelRelease":"4.2.0","releaseDate":"2013-9-2","carbonPlatform":"Turing"},{"id":5,"kernelRelease":"4.4.1","releaseDate":"2015-6-1","carbonPlatform":"Wilkes"}]`},
	}
	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			w := legacyServe(t, legacyFixtureClient(nil, nil), http.MethodGet, tc.target)
			assertStatus(t, w, http.StatusOK)
			if got := w.Body.String(); got != tc.want {
				t.Errorf("body\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestProductAPIRouteSpecificity proves the literal segments win over the {id}
// wildcards they sit beside: each would be a 400 from the {id} handler.
func TestProductAPIRouteSpecificity(t *testing.T) {
	tests := []struct {
		target   string
		wantFeed string // "-" when the versions endpoint must not be called
	}{
		{"/products/support-matrix", "support-matrix"},
		{"/products/release-matrix", "release-matrix"},
		{"/carbon-platform-history/versions", "-"},
		{"/carbon-platform-history/latest", "-"},
		{"/carbon-kernel-releases/platforms", "-"},
		{"/carbon-kernel-releases/latest-by-platform", "-"},
	}
	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			var feeds []string
			w := legacyServe(t, legacyFixtureClient(&feeds, nil), http.MethodGet, tc.target)
			assertStatus(t, w, http.StatusOK)
			if tc.wantFeed == "-" {
				if len(feeds) != 0 {
					t.Errorf("versions fetched %v", feeds)
				}
			} else if !reflect.DeepEqual(feeds, []string{tc.wantFeed}) {
				t.Errorf("feeds = %v, want [%s]", feeds, tc.wantFeed)
			}
		})
	}

	// The paths are served at the root only, as legacy served them.
	mux := http.NewServeMux()
	NewProductAPIHandler(legacyFixtureClient(nil, nil)).RegisterRoutes(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/product-api/products/support-matrix", nil))
	assertStatus(t, w, http.StatusNotFound)
}

func TestProductAPIWritesAreReadOnly(t *testing.T) {
	tests := []struct{ method, target string }{
		{http.MethodPost, "/products"},
		{http.MethodPut, "/products/758"},
		{http.MethodDelete, "/products/758"},
		{http.MethodPut, "/products/support-matrix"},
		{http.MethodPost, "/release-line"},
		{http.MethodPost, "/product-group"},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			called := false
			client := &mockEntityProductAPIClient{versionsFn: func(context.Context, string) ([]byte, error) {
				called = true
				return nil, nil
			}}
			w := legacyServe(t, client, tc.method, tc.target)
			assertStatus(t, w, http.StatusMethodNotAllowed)
			assertContentType(t, w, "application/json")
			if allow := w.Header().Get("Allow"); allow != "GET, HEAD" {
				t.Errorf("Allow = %q", allow)
			}
			assertErrorMessage(t, w, errMsgLegacyReadOnly)
			if called {
				t.Error("write route called upstream")
			}
		})
	}
}

func TestProductAPIUpstreamErrors(t *testing.T) {
	endpoints := []struct {
		target, fallback string
		client           func(err error) *mockEntityProductAPIClient
	}{
		{"/products", errMsgLegacyProducts, func(err error) *mockEntityProductAPIClient {
			return &mockEntityProductAPIClient{versionsFn: func(context.Context, string) ([]byte, error) { return nil, err }}
		}},
		{"/products/support-matrix", errMsgLegacyProducts, func(err error) *mockEntityProductAPIClient {
			return &mockEntityProductAPIClient{releaseLinesFn: func(context.Context, bool) ([]byte, error) { return nil, err }}
		}},
		{"/release-line", errMsgLegacyLines, func(err error) *mockEntityProductAPIClient {
			return &mockEntityProductAPIClient{releaseLinesFn: func(context.Context, bool) ([]byte, error) { return nil, err }}
		}},
		{"/carbon-platform-history", errMsgLegacyCarbon, func(err error) *mockEntityProductAPIClient {
			return &mockEntityProductAPIClient{carbonFn: func(context.Context) ([]byte, error) { return nil, err }}
		}},
		{"/carbon-kernel-releases", errMsgLegacyKernel, func(err error) *mockEntityProductAPIClient {
			return &mockEntityProductAPIClient{kernelsFn: func(context.Context) ([]byte, error) { return nil, err }}
		}},
	}
	for _, ep := range endpoints {
		for _, tc := range upstreamErrors(ep.fallback) {
			// Legacy only ever failed with 500, or 503 when unavailable: an
			// upstream 4xx is never passed through to a legacy caller.
			wantCode := http.StatusInternalServerError
			if tc.wantCode == http.StatusServiceUnavailable {
				wantCode = http.StatusServiceUnavailable
			}
			t.Run(ep.target+" "+tc.name, func(t *testing.T) {
				w := legacyServe(t, ep.client(tc.err), http.MethodGet, ep.target)
				assertStatus(t, w, wantCode)
				assertErrorMessage(t, w, ep.fallback)
			})
		}
	}
}

func TestProductAPIUndecodableUpstreamBody(t *testing.T) {
	client := &mockEntityProductAPIClient{versionsFn: func(context.Context, string) ([]byte, error) {
		return []byte(`not json`), nil
	}}
	w := legacyServe(t, client, http.MethodGet, "/products")
	assertStatus(t, w, http.StatusInternalServerError)
	assertErrorMessage(t, w, errMsgLegacyProducts)
}

// TestLegacyEmptyCatalogue: empty upstream lists are [] downstream.
func TestProductAPIEmptyCatalogue(t *testing.T) {
	for _, target := range []string{"/products", "/products/support-matrix", "/release-line", "/carbon-platform-history", "/carbon-platform-history/versions", "/carbon-kernel-releases", "/carbon-kernel-releases/platforms", "/carbon-kernel-releases/latest-by-platform"} {
		w := legacyServe(t, &mockEntityProductAPIClient{}, http.MethodGet, target)
		assertStatus(t, w, http.StatusOK)
		var v []json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil || w.Body.String() != "[]" {
			t.Errorf("%s: body = %q, want []", target, w.Body.String())
		}
	}
}

func TestDateOnly(t *testing.T) {
	s := func(v string) *string { return &v }
	tests := []struct {
		in   *string
		want *string
	}{
		{nil, nil},
		{s("2020-04-07T00:00:00Z"), s("2020-04-07")},
		{s("2020-04-07"), s("2020-04-07")},
		{s("not a timestamp"), s("not a timestamp")},
	}
	for _, tc := range tests {
		got := dateOnly(tc.in)
		if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
			t.Errorf("dateOnly(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

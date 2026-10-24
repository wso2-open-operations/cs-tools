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
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/entry"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/middleware"
)

func TestCreateTakesTheAuthorFromTheToken(t *testing.T) {
	api := newTestAPI(t)
	e := api.create(t, jane, `{"title":" T ","who":"Someone Else","where":"Customer","whereDetail":" Acme Corp ","what":"<p>Learned X.</p>"}`)
	if e.SubmittedByEmail != "jane@example.com" || e.Who != "Jane Doe" || e.Title != "T" || *e.WhereDetail != "Acme Corp" {
		t.Fatalf("created = %+v", e)
	}
	if e.UpdatedAt != nil {
		t.Fatal("a new entry must not be marked edited")
	}

	e = api.create(t, jane, `{"title":"T","where":"Internal","what":"<p>Learned Y.</p>"}`)
	if e.Who != "Jane Doe" {
		t.Fatalf("without who: byline = %q", e.Who)
	}
}

func TestCreateAnnouncesTheEntryWithAnArticleLink(t *testing.T) {
	api := newTestAPI(t)
	e := api.create(t, jane, validBody)
	if len(api.notifier.sent) != 1 {
		t.Fatalf("announcements = %d, want 1", len(api.notifier.sent))
	}
	got := api.notifier.sent[0]
	if got.entry.ID != e.ID || got.entryURL != "https://one.example/knowledge/today-i-learned/"+e.ID+"?view=article" {
		t.Fatalf("announced %+v", got)
	}
}

func TestCreateAnnouncesWithoutALinkWhenTheWebappAddressIsUnset(t *testing.T) {
	api := newTestAPI(t, func(c *Config) { c.WebappBaseURL = "" })
	api.create(t, jane, validBody)
	if got := api.notifier.sent[0].entryURL; got != "" {
		t.Fatalf("entryURL = %q, want none", got)
	}
}

func TestCreateWorksWithAnnouncementsOff(t *testing.T) {
	api := newTestAPI(t, func(c *Config) { c.Notifier = nil })
	api.create(t, jane, validBody)
}

func TestCreateAndEditAreOnlyForTheWebapp(t *testing.T) {
	api := newTestAPI(t)
	assertError(t, api.do(t, janeViaOtherApp, http.MethodPost, "/submissions", validBody), http.StatusForbidden, msgWebappOnly)

	e := api.create(t, jane, validBody)
	assertError(t, api.do(t, janeViaOtherApp, http.MethodPut, "/submissions/"+e.ID, `{"title":"T2","where":"Internal","what":"x"}`),
		http.StatusForbidden, msgWebappOnly)
	if len(api.notifier.sent) != 1 {
		t.Fatal("a refused create must not be announced")
	}
}

func TestCreateIsRefusedToEveryoneWhenTheWebappClientIsUnset(t *testing.T) {
	api := newTestAPI(t, func(c *Config) { c.WebappClientID = "" })
	assertError(t, api.do(t, jane, http.MethodPost, "/submissions", validBody), http.StatusForbidden, msgWebappOnly)
}

func TestCreateValidates(t *testing.T) {
	api := newTestAPI(t)
	tests := []struct{ body, want string }{
		{`{"title":"","who":"Jane","where":"Internal","what":"x"}`, "Title is required."},
		{`{"title":"T","who":"Jane","where":"Customer","what":"x"}`, "Customer name is required."},
		{`{"title":"T","who":"Jane","where":"Elsewhere","what":"x"}`, "Choose a source: Customer, Partner, Internal or Other."},
		{`{"title":"T","who":"Jane","where":"Internal","what":"<p><br></p>"}`, "Add what you learned."},
	}
	for _, tc := range tests {
		assertError(t, api.do(t, jane, http.MethodPost, "/submissions", tc.body), http.StatusBadRequest, tc.want)
	}
	if len(api.notifier.sent) != 0 {
		t.Fatal("an invalid entry must not be announced")
	}
}

func TestCreateRejectsMalformedBodies(t *testing.T) {
	api := newTestAPI(t)
	for _, body := range []string{
		`not json`,
		`["not", "an", "object"]`,
		`{"title":"T","who":"Jane","where":"Internal","what":"x","extra":1}`,
		validBody + `{}`,
		`{"title":5,"who":"Jane","where":"Internal","what":"x"}`,
	} {
		assertError(t, api.do(t, jane, http.MethodPost, "/submissions", body), http.StatusBadRequest, msgBadRequest)
	}
}

func TestCreateSanitises(t *testing.T) {
	api := newTestAPI(t)
	e := api.create(t, jane, `{"title":"T","who":"Jane","where":"Internal","what":"<p>hi</p><script>alert(1)</script><img src=\"https://x.example/a.png\">"}`)
	if e.What != "<p>hi</p>" {
		t.Fatalf("what = %q", e.What)
	}
}

func TestCreateReportsAStoreFailure(t *testing.T) {
	api := newTestAPI(t)
	api.store.fail = errors.New("database is down")
	assertError(t, api.do(t, jane, http.MethodPost, "/submissions", validBody), http.StatusInternalServerError, msgInternal)
	if len(api.notifier.sent) != 0 {
		t.Fatal("an unsaved entry must not be announced")
	}
}

func TestGetEntry(t *testing.T) {
	api := newTestAPI(t)
	e := api.create(t, jane, validBody)
	w := api.do(t, sam, http.MethodGet, "/submissions/"+e.ID, "")
	if w.Code != http.StatusOK || decodeJSON[entry.Entry](t, w).ID != e.ID {
		t.Fatalf("any signed-in employee can read: %d %s", w.Code, w.Body)
	}
	assertError(t, api.do(t, sam, http.MethodGet, "/submissions/00000000-0000-4000-8000-999999999999", ""), http.StatusNotFound, msgNotFound)
	assertError(t, api.do(t, sam, http.MethodGet, "/submissions/not-an-id", ""), http.StatusNotFound, msgNotFound)
}

func TestAuthorCanEditTheirOwnEntry(t *testing.T) {
	api := newTestAPI(t)
	e := api.create(t, jane, validBody)
	w := api.do(t, jane, http.MethodPut, "/submissions/"+e.ID,
		`{"title":"New title","where":"Customer","whereDetail":"Acme Corp","what":"<p>Changed</p>"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	got := decodeJSON[entry.Entry](t, w)
	if got.Title != "New title" || got.Where != "Customer" || *got.WhereDetail != "Acme Corp" || got.What != "<p>Changed</p>" || got.UpdatedAt == nil {
		t.Fatalf("updated = %+v", got)
	}
	if got.Who != e.Who || got.SubmittedByEmail != e.SubmittedByEmail || !got.CreatedAt.Equal(e.CreatedAt) {
		t.Fatalf("byline or creation time changed: %+v", got)
	}
}

func TestModeratorEditKeepsTheAuthorsByline(t *testing.T) {
	api := newTestAPI(t)
	e := api.create(t, jane, validBody)
	w := api.do(t, mod, http.MethodPut, "/submissions/"+e.ID, `{"title":"Fixed typo","where":"Internal","what":"<p>x</p>"}`)
	got := decodeJSON[entry.Entry](t, w)
	if w.Code != http.StatusOK || got.Who != "Jane Doe" || got.SubmittedByEmail != "jane@example.com" {
		t.Fatalf("status %d, entry %+v", w.Code, got)
	}
}

func TestEditRejectsAByline(t *testing.T) {
	// The byline can't be changed, so a body carrying one is refused.
	api := newTestAPI(t)
	e := api.create(t, jane, validBody)
	assertError(t, api.do(t, mod, http.MethodPut, "/submissions/"+e.ID, `{"title":"T","who":"Mod","where":"Internal","what":"x"}`),
		http.StatusBadRequest, msgBadRequest)
}

func TestOthersCannotEditOrDelete(t *testing.T) {
	api := newTestAPI(t)
	e := api.create(t, jane, validBody)
	assertError(t, api.do(t, sam, http.MethodPut, "/submissions/"+e.ID, `{"title":"Mine now","where":"Internal","what":"x"}`),
		http.StatusForbidden, msgCannotEdit)
	assertError(t, api.do(t, sam, http.MethodDelete, "/submissions/"+e.ID, ""), http.StatusForbidden, msgCannotDelete)
	if got := api.store.entries[e.ID]; got.Title != "T" {
		t.Fatalf("entry changed: %+v", got)
	}
}

func TestEditOrDeleteOfAMissingEntryIsNotFound(t *testing.T) {
	api := newTestAPI(t)
	missing := "/submissions/00000000-0000-4000-8000-999999999999"
	assertError(t, api.do(t, mod, http.MethodPut, missing, `{"title":"T","where":"Internal","what":"x"}`), http.StatusNotFound, msgNotFound)
	assertError(t, api.do(t, mod, http.MethodDelete, missing, ""), http.StatusNotFound, msgNotFound)
}

func TestEditOfAnEntryDeletedMidEditIsNotFound(t *testing.T) {
	// Someone deletes the entry after it was loaded but before the update
	// lands: nothing was saved, so the edit must not report success.
	api := newTestAPI(t)
	e := api.create(t, jane, validBody)
	api.store.beforeUpdate = func(id string) { delete(api.store.entries, id) }
	assertError(t, api.do(t, jane, http.MethodPut, "/submissions/"+e.ID, `{"title":"T2","where":"Internal","what":"x"}`),
		http.StatusNotFound, msgNotFound)
}

func TestUnchangedEditDoesNotMarkTheEntryEdited(t *testing.T) {
	api := newTestAPI(t)
	e := api.create(t, jane, `{"title":"T","who":"Jane","where":"Customer","whereDetail":"Acme","what":"<p>It's \"x\"</p>"}`)
	w := api.do(t, jane, http.MethodPut, "/submissions/"+e.ID, `{"title":" T ","where":"Customer","whereDetail":"Acme ","what":"<p>It's \"x\"</p>"}`)
	if got := decodeJSON[entry.Entry](t, w); w.Code != http.StatusOK || got.UpdatedAt != nil {
		t.Fatalf("status %d, updatedAt %v", w.Code, got.UpdatedAt)
	}
}

func TestEditValidatesAndIsNotAnnounced(t *testing.T) {
	api := newTestAPI(t)
	e := api.create(t, jane, validBody)
	assertError(t, api.do(t, jane, http.MethodPut, "/submissions/"+e.ID, `{"title":"T","where":"Customer","what":"x"}`),
		http.StatusBadRequest, "Customer name is required.")
	api.do(t, jane, http.MethodPut, "/submissions/"+e.ID, `{"title":"T2","where":"Internal","what":"x"}`)
	if len(api.notifier.sent) != 1 {
		t.Fatalf("announcements = %d, want only the create", len(api.notifier.sent))
	}
}

func TestDelete(t *testing.T) {
	api := newTestAPI(t)
	own := api.create(t, jane, validBody)
	if w := api.do(t, jane, http.MethodDelete, "/submissions/"+own.ID, ""); w.Code != http.StatusNoContent {
		t.Fatalf("author delete: %d", w.Code)
	}
	other := api.create(t, jane, validBody)
	if w := api.do(t, mod, http.MethodDelete, "/submissions/"+other.ID, ""); w.Code != http.StatusNoContent {
		t.Fatalf("moderator delete: %d", w.Code)
	}
	if len(api.store.entries) != 0 {
		t.Fatal("entries not deleted")
	}
}

func TestDeleteNeedsNoWebappToken(t *testing.T) {
	// Only creating and editing are tied to the webapp.
	api := newTestAPI(t)
	e := api.create(t, jane, validBody)
	if w := api.do(t, janeViaOtherApp, http.MethodDelete, "/submissions/"+e.ID, ""); w.Code != http.StatusNoContent {
		t.Fatalf("status %d", w.Code)
	}
}

func TestList(t *testing.T) {
	api := newTestAPI(t)
	for range 3 {
		api.create(t, jane, validBody)
	}
	api.create(t, sam, validBody)

	w := api.do(t, jane, http.MethodGet, "/submissions?limit=2&offset=1", "")
	page := decodeJSON[listResponse](t, w)
	if w.Code != http.StatusOK || len(page.Items) != 2 || page.Total != 4 || page.Limit != 2 || page.Offset != 1 {
		t.Fatalf("page = %+v", page)
	}

	page = decodeJSON[listResponse](t, api.do(t, jane, http.MethodGet, "/submissions?mine=true", ""))
	if page.Total != 3 || api.store.lastQuery.AuthorEmail != "jane@example.com" || page.Limit != defaultPageSize {
		t.Fatalf("mine: total %d, query %+v", page.Total, api.store.lastQuery)
	}

	// A reader at +05:30 asking for 1-31 October in their own days.
	api.do(t, jane, http.MethodGet, "/submissions?q=+retries++idempotency+&createdFrom=2026-09-30T18:30:00.000Z&createdBefore=2026-10-31T18:30:00%2B00:00", "")
	q := api.store.lastQuery
	if q.Search != "retries  idempotency" || q.From == nil || q.Before == nil ||
		!q.From.Equal(time.Date(2026, 9, 30, 18, 30, 0, 0, time.UTC)) || !q.Before.Equal(time.Date(2026, 10, 31, 18, 30, 0, 0, time.UTC)) {
		t.Fatalf("query = %+v", q)
	}

	api.do(t, jane, http.MethodGet, "/submissions?author=+Ale+Sam+&source=Customer", "")
	if q := api.store.lastQuery; q.Author != "Ale Sam" || q.Source != "Customer" {
		t.Fatalf("author and source: query = %+v", q)
	}

	w = api.do(t, jane, http.MethodGet, "/submissions?offset=50", "")
	if page := decodeJSON[listResponse](t, w); page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("past the end: %s", w.Body)
	}
}

func TestListRejectsBadQueries(t *testing.T) {
	api := newTestAPI(t)
	for query, want := range map[string]string{
		"limit=0":                  "limit must be a whole number from 1 to 100.",
		"limit=101":                "limit must be a whole number from 1 to 100.",
		"limit=ten":                "limit must be a whole number from 1 to 100.",
		"offset=-1":                "offset must be a whole number, 0 or more.",
		"createdFrom=2026-10-01":   "createdFrom must be a time like 2026-10-31T00:00:00Z.",
		"createdBefore=not-a-time": "createdBefore must be a time like 2026-10-31T00:00:00Z.",
		"createdFrom=2026-10-31T00:00:00Z&createdBefore=2026-10-01T00:00:00Z": "createdFrom must be before createdBefore.",
		"createdFrom=2026-10-01T00:00:00Z&createdBefore=2026-10-01T00:00:00Z": "createdFrom must be before createdBefore.",
		"mine=yes": "mine must be true or false.",
		"q=" + strings.Repeat("a", maxSearchLength+1):            "Search must be 200 characters or fewer.",
		"author=" + strings.Repeat("a", entry.AuthorMaxLength+1): "author must be 200 characters or fewer.",
		"source=customer": "source must be Customer, Partner, Internal or Other.",
		"source=Vendor":   "source must be Customer, Partner, Internal or Other.",
	} {
		assertError(t, api.do(t, jane, http.MethodGet, "/submissions?"+query, ""), http.StatusBadRequest, want)
	}
}

func TestUserInfo(t *testing.T) {
	api := newTestAPI(t)
	got := decodeJSON[map[string]any](t, api.do(t, mod, http.MethodGet, "/user-info", ""))
	if got["email"] != "mod@example.com" || got["displayName"] != "Mod" || got["canModerate"] != true {
		t.Fatalf("user-info = %v", got)
	}
}

func TestEveryRouteNeedsASignedInUser(t *testing.T) {
	api := newTestAPI(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/user-info"},
		{http.MethodGet, "/submissions"},
		{http.MethodPost, "/submissions"},
		{http.MethodGet, "/submissions/00000000-0000-4000-8000-000000000001"},
		{http.MethodPut, "/submissions/00000000-0000-4000-8000-000000000001"},
		{http.MethodDelete, "/submissions/00000000-0000-4000-8000-000000000001"},
	} {
		assertError(t, api.do(t, nil, route.method, route.path, ""), http.StatusUnauthorized, msgNotSignedIn)
	}
}

func TestOversizedBodyIsRejected(t *testing.T) {
	api := newTestAPI(t)
	big := `{"title":"T","who":"Jane","where":"Internal","what":"` + strings.Repeat("a", 2048) + `"}`
	h := middleware.BodyLimit(1024)(api.mux)
	req := newRequest(t, jane, http.MethodPost, "/submissions", big)
	req.ContentLength = -1 // as if sent without a length
	w := record(h, req)
	assertError(t, w, http.StatusRequestEntityTooLarge, msgTooLarge)
}

func TestSettings(t *testing.T) {
	got := decodeJSON[map[string]any](t, newTestAPI(t).do(t, jane, http.MethodGet, "/settings", ""))
	if got["whatMaxLength"] != float64(500) {
		t.Fatalf("default settings = %v", got)
	}
	api := newTestAPI(t, func(c *Config) { c.WhatMaxLength = 800 })
	if got := decodeJSON[map[string]any](t, api.do(t, jane, http.MethodGet, "/settings", "")); got["whatMaxLength"] != float64(800) {
		t.Fatalf("configured settings = %v", got)
	}
}

func TestCreateEnforcesTheConfiguredLimit(t *testing.T) {
	api := newTestAPI(t, func(c *Config) { c.WhatMaxLength = 20 })
	body := func(n int) string {
		return `{"title":"T","who":"Jane","where":"Internal","what":"<p>` + strings.Repeat("x", n) + `</p>"}`
	}
	// The form stops at 20; the margin past it is still accepted.
	if w := api.do(t, jane, http.MethodPost, "/submissions", body(20+entry.WhatLengthMargin)); w.Code != http.StatusCreated {
		t.Fatalf("within the margin: %d %s", w.Code, w.Body)
	}
	assertError(t, api.do(t, jane, http.MethodPost, "/submissions", body(21+entry.WhatLengthMargin)), http.StatusBadRequest, "What you learned must be 20 characters or fewer.")
}

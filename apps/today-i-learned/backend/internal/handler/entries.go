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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/entry"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/store"
)

const (
	defaultPageSize = 10
	// The feed asks for 5 or 10 a page. 100 stays allowed so a webapp still
	// loading its feed in one request (limit=100) keeps working.
	maxPageSize     = 100
	maxSearchLength = 200
	// notifyTimeout bounds an announcement, which runs before the response so
	// the caller learns whether Novera accepted it. It stays under the 15 s a
	// shutdown waits for requests in flight (cmd/server), so a redeploy never
	// stops a send half-way and leaves the entry claimed but unsent. Novera
	// answers at once, after checking only that it can reach Google Chat.
	notifyTimeout = 10 * time.Second
)

var idPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// createRequest's Who is ignored: the byline is the caller's name from their
// token. It's still accepted so a client that sends it isn't refused.
type createRequest struct {
	Title       string  `json:"title"`
	Who         string  `json:"who"`
	Where       string  `json:"where"`
	WhereDetail *string `json:"whereDetail"`
	What        string  `json:"what"`
}

// updateRequest has no "who": an edit never changes the byline, even when a
// moderator makes it.
type updateRequest struct {
	Title       string  `json:"title"`
	Where       string  `json:"where"`
	WhereDetail *string `json:"whereDetail"`
	What        string  `json:"what"`
}

type listResponse struct {
	Items  []entry.Entry `json:"items"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

// ListEntries returns a page of entries, newest first, with how many match
// in total.
func (h *Handler) ListEntries(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	q, err := parseListQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if r.URL.Query().Get("mine") == "true" {
		q.AuthorEmail = user.Email
	}
	page, err := h.cfg.Store.List(r.Context(), q)
	if err != nil {
		slog.ErrorContext(r.Context(), "list entries failed", "err", err)
		writeError(w, http.StatusInternalServerError, msgInternal)
		return
	}
	writeJSON(w, http.StatusOK, listResponse{Items: page.Items, Total: page.Total, Limit: q.Limit, Offset: q.Offset})
}

func parseListQuery(v url.Values) (store.Query, error) {
	q := store.Query{Limit: defaultPageSize}

	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > maxPageSize {
			return q, fmt.Errorf("limit must be a whole number from 1 to %d.", maxPageSize)
		}
		q.Limit = n
	}
	if s := v.Get("offset"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return q, errors.New("offset must be a whole number, 0 or more.")
		}
		q.Offset = n
	}

	q.Search = strings.TrimSpace(v.Get("q"))
	if utf8.RuneCountInString(q.Search) > maxSearchLength {
		return q, fmt.Errorf("Search must be %d characters or fewer.", maxSearchLength)
	}

	for _, b := range []struct {
		name string
		dst  **time.Time
	}{{"createdFrom", &q.From}, {"createdBefore", &q.Before}} {
		s := v.Get(b.name)
		if s == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return q, fmt.Errorf("%s must be a time like 2026-10-31T00:00:00Z.", b.name)
		}
		t = t.UTC()
		*b.dst = &t
	}
	if q.From != nil && q.Before != nil && !q.From.Before(*q.Before) {
		return q, errors.New("createdFrom must be before createdBefore.")
	}

	q.Author = strings.TrimSpace(v.Get("author"))
	if utf8.RuneCountInString(q.Author) > entry.AuthorMaxLength {
		return q, fmt.Errorf("author must be %d characters or fewer.", entry.AuthorMaxLength)
	}
	if q.Source = v.Get("source"); q.Source != "" && !slices.Contains(entry.Sources, q.Source) {
		return q, errors.New("source must be Customer, Partner, Internal or Other.")
	}

	switch v.Get("mine") {
	case "", "true", "false":
	default:
		return q, errors.New("mine must be true or false.")
	}
	return q, nil
}

// GetEntry returns one entry.
func (h *Handler) GetEntry(w http.ResponseWriter, r *http.Request) {
	if _, ok := currentUser(w, r); !ok {
		return
	}
	e, ok := h.load(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, e)
}

// CreateEntry shares a new entry under the caller's verified email and
// announces it.
func (h *Handler) CreateEntry(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	if !user.IssuedTo(h.cfg.WebappClientID) {
		writeError(w, http.StatusForbidden, msgWebappOnly)
		return
	}
	var req createRequest
	if !decode(w, r, &req) {
		return
	}
	who, err := entry.ValidateAuthor(user.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	content, err := entry.Prepare(entry.Draft{Title: req.Title, Where: req.Where, WhereDetail: req.WhereDetail, What: req.What}, h.cfg.WhatMaxLength)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	e, err := h.cfg.Store.Create(r.Context(), store.Author{Name: who, Email: user.Email}, content)
	if err != nil {
		slog.ErrorContext(r.Context(), "create entry failed", "err", err)
		writeError(w, http.StatusInternalServerError, msgInternal)
		return
	}
	slog.InfoContext(r.Context(), "entry created", "entryID", e.ID)

	// Best effort: the entry is saved either way. If the announcement fails,
	// the entry stays unannounced for a moderator to send again.
	if h.cfg.Notifier != nil {
		if at, err := h.announce(r.Context(), e); err == nil {
			e.NotifiedAt = &at
		}
	}
	writeJSON(w, http.StatusCreated, e)
}

// NotifyEntry sends an entry to Google Chat when its announcement failed.
// Only a moderator may, from the webapp, and only while the entry hasn't been
// sent, so no one ever gets the same card twice.
func (h *Handler) NotifyEntry(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	if !user.IssuedTo(h.cfg.WebappClientID) {
		writeError(w, http.StatusForbidden, msgWebappOnly)
		return
	}
	if !user.IsModerator {
		writeError(w, http.StatusForbidden, msgCannotNotify)
		return
	}
	if h.cfg.Notifier == nil {
		writeError(w, http.StatusServiceUnavailable, msgNoNotifier)
		return
	}
	e, ok := h.load(w, r)
	if !ok {
		return
	}
	if e.NotifiedAt != nil {
		writeError(w, http.StatusConflict, msgNotified)
		return
	}
	at, err := h.announce(r.Context(), e)
	switch {
	case errors.Is(err, errAlreadyAnnounced):
		writeError(w, http.StatusConflict, msgNotified)
		return
	case errors.Is(err, errNotAccepted):
		writeError(w, http.StatusBadGateway, msgNotifyFailed)
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, msgInternal)
		return
	}
	slog.InfoContext(r.Context(), "entry sent to Google Chat", "entryID", e.ID)
	e.NotifiedAt = &at
	writeJSON(w, http.StatusOK, e)
}

var (
	// errAlreadyAnnounced: another request announced the entry first.
	errAlreadyAnnounced = errors.New("entry already announced")
	// errNotAccepted: Novera didn't accept the announcement.
	errNotAccepted = errors.New("announcement not accepted")
)

// announce sends e to Google Chat and returns when it was marked announced.
// The entry is claimed first, so two requests can't both send it; if Novera
// doesn't accept the announcement, the claim is released and the entry counts
// as not announced again. Errors are logged here.
func (h *Handler) announce(ctx context.Context, e entry.Entry) (time.Time, error) {
	// Detached from the request, so a client hanging up can't leave a claim
	// behind or cut an announcement off half-way.
	ctx = context.WithoutCancel(ctx)
	at, claimed, err := h.cfg.Store.ClaimNotification(ctx, e.ID)
	if err != nil {
		slog.ErrorContext(ctx, "claim notification failed", "entryID", e.ID, "err", err)
		return time.Time{}, err
	}
	if !claimed {
		return time.Time{}, errAlreadyAnnounced
	}

	notifyCtx, cancel := context.WithTimeout(ctx, notifyTimeout)
	err = h.cfg.Notifier.Notify(notifyCtx, e, h.articleURL(e.ID))
	cancel()
	if err == nil {
		return at, nil
	}
	if rerr := h.cfg.Store.ReleaseNotification(ctx, e.ID, at); rerr != nil {
		slog.ErrorContext(ctx, "release notification failed", "entryID", e.ID, "err", rerr)
	}
	return time.Time{}, fmt.Errorf("%w: %w", errNotAccepted, err)
}

// UpdateEntry replaces an entry's content. Only the author or a moderator
// may, and only from the webapp. The byline stays the author's.
func (h *Handler) UpdateEntry(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	if !user.IssuedTo(h.cfg.WebappClientID) {
		writeError(w, http.StatusForbidden, msgWebappOnly)
		return
	}
	existing, ok := h.load(w, r)
	if !ok {
		return
	}
	if !canChange(user, existing) {
		writeError(w, http.StatusForbidden, msgCannotEdit)
		return
	}
	var req updateRequest
	if !decode(w, r, &req) {
		return
	}
	content, err := entry.Prepare(entry.Draft{Title: req.Title, Where: req.Where, WhereDetail: req.WhereDetail, What: req.What}, h.cfg.WhatMaxLength)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Saving without a change would still mark the entry as edited.
	if entry.SameContent(existing, content) {
		writeJSON(w, http.StatusOK, existing)
		return
	}

	updated, err := h.cfg.Store.Update(r.Context(), existing.ID, content)
	if errors.Is(err, store.ErrNotFound) {
		// Deleted since it was loaded above: nothing was saved.
		writeError(w, http.StatusNotFound, msgNotFound)
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "update entry failed", "entryID", existing.ID, "err", err)
		writeError(w, http.StatusInternalServerError, msgInternal)
		return
	}
	slog.InfoContext(r.Context(), "entry updated", "entryID", existing.ID, "byAuthor", isAuthor(user, existing))
	writeJSON(w, http.StatusOK, updated)
}

// DeleteEntry removes an entry. Only the author or a moderator may.
func (h *Handler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	existing, ok := h.load(w, r)
	if !ok {
		return
	}
	if !canChange(user, existing) {
		writeError(w, http.StatusForbidden, msgCannotDelete)
		return
	}
	err := h.cfg.Store.Delete(r.Context(), existing.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, msgNotFound)
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "delete entry failed", "entryID", existing.ID, "err", err)
		writeError(w, http.StatusInternalServerError, msgInternal)
		return
	}
	// The row is gone, so this line is the only record of the delete.
	slog.InfoContext(r.Context(), "entry deleted", "entryID", existing.ID, "byAuthor", isAuthor(user, existing))
	w.WriteHeader(http.StatusNoContent)
}

// load reads the entry named by the {id} path value, writing a 404 when
// there isn't one.
func (h *Handler) load(w http.ResponseWriter, r *http.Request) (entry.Entry, bool) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		writeError(w, http.StatusNotFound, msgNotFound)
		return entry.Entry{}, false
	}
	e, err := h.cfg.Store.Get(r.Context(), strings.ToLower(id))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, msgNotFound)
		return entry.Entry{}, false
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "get entry failed", "err", err)
		writeError(w, http.StatusInternalServerError, msgInternal)
		return entry.Entry{}, false
	}
	return e, true
}

// canChange reports whether user may edit or delete e: its author or a
// moderator.
func canChange(user *middleware.User, e entry.Entry) bool {
	return user.IsModerator || isAuthor(user, e)
}

// isAuthor reports whether user shared e, matched by verified email, never
// the byline.
func isAuthor(user *middleware.User, e entry.Entry) bool {
	return strings.EqualFold(user.Email, e.SubmittedByEmail)
}

// articleURL is the link to an entry's full-page article in the webapp, or
// "" when the webapp's address isn't configured.
func (h *Handler) articleURL(id string) string {
	if h.cfg.WebappBaseURL == "" {
		return ""
	}
	return h.cfg.WebappBaseURL + "/knowledge/today-i-learned/" + url.PathEscape(id) + "?view=article"
}

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

// Package handler serves the TIL API. Every route except /health runs behind
// middleware.Auth, so a handler always has the signed-in user.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/entry"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/store"
)

// Messages for people: the webapp shows them as they are.
const (
	msgNotSignedIn  = "Your session isn't valid or has expired. Sign in again."
	msgNotFound     = "This entry doesn't exist. It may have been deleted."
	msgWebappOnly   = "Entries can only be shared and edited from One WSO2."
	msgCannotEdit   = "Only the author or a moderator can edit this entry."
	msgCannotDelete = "Only the author or a moderator can delete this entry."
	msgCannotNotify = "Only a moderator can send an entry to Google Chat."
	msgNoNotifier   = "Sending entries to Google Chat isn't set up."
	msgNotified     = "This entry was already sent to Google Chat."
	msgNotifyFailed = "Couldn't send this entry to Google Chat. Try again later."
	msgBadRequest   = "The request isn't valid."
	msgTooLarge     = "The request is too large."
	msgInternal     = "Something went wrong. Please try again."
)

// Store is where entries are kept.
type Store interface {
	Create(ctx context.Context, author store.Author, c entry.Content) (entry.Entry, error)
	Get(ctx context.Context, id string) (entry.Entry, error)
	Update(ctx context.Context, id string, c entry.Content) (entry.Entry, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, q store.Query) (store.Page, error)
	ClaimNotification(ctx context.Context, id string) (at time.Time, claimed bool, err error)
	ReleaseNotification(ctx context.Context, id string, at time.Time) error
}

// Notifier announces an entry in Google Chat. It returns an error when the
// announcement wasn't accepted.
type Notifier interface {
	Notify(ctx context.Context, e entry.Entry, entryURL string) error
}

// Config wires a Handler.
type Config struct {
	Store Store
	// Notifier is nil when new entries aren't announced.
	Notifier Notifier
	// WebappClientID is the One WSO2 webapp's client id. Only a token issued
	// to it may create or edit entries; empty means no one can.
	WebappClientID string
	// WebappBaseURL is the One WSO2 webapp's own address, for the link in a
	// new-entry announcement. Empty means the announcement has no link.
	WebappBaseURL string
	// WhatMaxLength is the most characters of plain text a learning may have;
	// zero means entry.DefaultWhatMaxLength.
	WhatMaxLength int
}

// Handler serves the API's routes.
type Handler struct {
	cfg Config
}

// New returns a Handler for cfg.
func New(cfg Config) *Handler {
	cfg.WebappBaseURL = strings.TrimRight(cfg.WebappBaseURL, "/")
	if cfg.WhatMaxLength <= 0 {
		cfg.WhatMaxLength = entry.DefaultWhatMaxLength
	}
	return &Handler{cfg: cfg}
}

// Health answers the liveness probe.
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UserInfo returns who the caller is and whether they moderate.
func (h *Handler) UserInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"email":       user.Email,
		"displayName": user.DisplayName,
		"canModerate": user.IsModerator,
	})
}

// Settings returns the rules the webapp's form shows, so they always match
// what this service enforces.
func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	if _, ok := currentUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"whatMaxLength": h.cfg.WhatMaxLength,
		// Whether entries are announced, so the webapp flags unsent ones only
		// when there's something to send them to.
		"notificationsEnabled": h.cfg.Notifier != nil,
	})
}

func currentUser(w http.ResponseWriter, r *http.Request) (*middleware.User, bool) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, msgNotSignedIn)
		return nil, false
	}
	return user, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "err", err)
	}
}

// writeError writes {"message": "..."}.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Message string `json:"message"`
	}{message})
}

// decode reads a JSON body into dst. Unknown fields and anything after the
// JSON value are rejected. It writes the error response itself and reports
// whether to carry on.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil {
		if extra := dec.Decode(&struct{}{}); !errors.Is(extra, io.EOF) {
			err = errors.New("unexpected data after the JSON body")
			if extra != nil {
				err = extra
			}
		}
	}
	if err == nil {
		return true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, msgTooLarge)
		return false
	}
	writeError(w, http.StatusBadRequest, msgBadRequest)
	return false
}

package handler

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// PlgUsersHandler serves the user search.
type PlgUsersHandler struct {
	svc service.PlgUserService
}

// NewPlgUsersHandler wires the handler over its service.
func NewPlgUsersHandler(svc service.PlgUserService) *PlgUsersHandler {
	return &PlgUsersHandler{svc: svc}
}

// SearchUsers serves POST /plg/users/search.
//
// This is the endpoint the whole re-key rests on: it is how the BFF turns the
// email in X-PLG-User into the `users.id` that every subsequent write carries.
// It is also what the owner pickers read, which is deliberate — one query
// answers "who is on the team" and "who is this caller", so the two cannot
// disagree about which engineers exist.
//
// A body is required, even an empty `{}`. entity-service's decodeRequest
// rejects an absent body rather than treating it as no filters, and matching
// that matters more than the convenience: this handler is read alongside its
// siblings.
func (h *PlgUsersHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	var req domain.PlgSearchUsersRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	result, err := h.svc.SearchUsers(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, result)
}

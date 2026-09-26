package handler

import (
	"net/http"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// PlgAnalyticsHandler serves the dashboard and the work queue.
type PlgAnalyticsHandler struct {
	svc service.AnalyticsService
}

// NewPlgAnalyticsHandler wires the handler over its service.
func NewPlgAnalyticsHandler(svc service.AnalyticsService) *PlgAnalyticsHandler {
	return &PlgAnalyticsHandler{svc: svc}
}

// Dashboard serves GET /plg/analytics/dashboard?from=&to=.
//
// A GET with query parameters, unlike the work queue beside it: two optional
// dates are exactly what a query string is for, and the aggregation takes no
// other input.
func (h *PlgAnalyticsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	rng, err := parseAnalyticsRange(r)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	result, err := h.svc.Dashboard(r.Context(), rng)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, result)
}

// SearchWorkQueue serves POST /plg/work-queue/search.
func (h *PlgAnalyticsHandler) SearchWorkQueue(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchWorkQueueRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	result, err := h.svc.WorkQueue(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeOK(w, result)
}

// parseAnalyticsRange reads the optional from/to dates.
//
// Both are date-only (YYYY-MM-DD). A malformed one is rejected rather than
// ignored: silently widening a range the caller thought they had narrowed
// produces a chart that is wrong without looking wrong.
func parseAnalyticsRange(r *http.Request) (domain.AnalyticsRange, error) {
	var rng domain.AnalyticsRange
	q := r.URL.Query()

	if v := q.Get("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return rng, &apierror.ValidationError{Msg: "from must be YYYY-MM-DD"}
		}
		rng.From = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return rng, &apierror.ValidationError{Msg: "to must be YYYY-MM-DD"}
		}
		rng.To = &t
	}
	if rng.From != nil && rng.To != nil && rng.To.Before(*rng.From) {
		return rng, &apierror.ValidationError{Msg: "to must not be earlier than from"}
	}
	return rng, nil
}

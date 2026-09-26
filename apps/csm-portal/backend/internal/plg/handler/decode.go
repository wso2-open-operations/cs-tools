// Package handler holds the HTTP layer: decode, delegate to a service, encode.
// No business logic and no SQL lives here.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/apierror"
)

const maxRequestBodySize = 1 << 20 // 1 MiB

var sanitizeLog = strings.NewReplacer("\n", `\n`, "\r", `\r`).Replace

// decodeRequest decodes a JSON body into dst with unknown-field rejection, a
// 1 MiB cap, and no trailing data. It writes the 400 itself and returns false
// when decoding fails.
func decodeRequest[T any](w http.ResponseWriter, r *http.Request, dst *T) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		apierror.WriteJSON(w, http.StatusBadRequest, decodeErrMsg(err))
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		apierror.WriteJSON(w, http.StatusBadRequest, "request body must contain a single JSON object")
		return false
	}
	return true
}

// decodeOptionalRequest is decodeRequest for endpoints where an empty body is
// meaningful (a search with no filters, for example).
func decodeOptionalRequest[T any](w http.ResponseWriter, r *http.Request, dst *T) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		apierror.WriteJSON(w, http.StatusBadRequest, decodeErrMsg(err))
		return false
	}
	return true
}

func decodeErrMsg(err error) string {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return "request body too large"
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return fmt.Sprintf("request body contains malformed JSON at position %d", syntaxErr.Offset)
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		typeName := typeErr.Type.String()
		switch typeErr.Type.Kind() {
		case reflect.String:
			typeName = "string"
		case reflect.Int, reflect.Int64:
			typeName = "integer"
		case reflect.Bool:
			typeName = "boolean"
		}
		return fmt.Sprintf("invalid value for field %q: expected %s", typeErr.Field, typeName)
	}
	// An error raised by one of our own UnmarshalJSON implementations already
	// says exactly what is wrong with which field, so it must not be flattened
	// into "invalid request body" — that message helps nobody.
	var fieldErr *apierror.ValidationError
	if errors.As(err, &fieldErr) {
		return fieldErr.Msg
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "json: unknown field ") {
		return "unknown field " + strings.TrimPrefix(msg, "json: unknown field ") + " in request body"
	}
	return "invalid request body"
}

// writeJSON encodes a successful response.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeServiceError maps a service-layer error onto an HTTP status. Full detail
// is logged server-side; the caller only ever receives a safe message.
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		ve  *apierror.ValidationError
		nfe *apierror.NotFoundError
		fe  *apierror.ForbiddenError
		ce  *apierror.ConflictError
		ue  *apierror.UnauthorizedError
		sue *apierror.ServiceUnavailableError
	)
	switch {
	case errors.As(err, &ve):
		slog.WarnContext(r.Context(), "bad request",
			"method", r.Method, "path", sanitizeLog(r.URL.Path), "reason", sanitizeLog(ve.Msg))
		apierror.WriteJSON(w, http.StatusBadRequest, ve.Msg)
	case errors.As(err, &ue):
		slog.WarnContext(r.Context(), "unauthorized",
			"method", r.Method, "path", sanitizeLog(r.URL.Path))
		apierror.WriteJSON(w, http.StatusUnauthorized, ue.Msg)
	case errors.As(err, &fe):
		slog.WarnContext(r.Context(), "forbidden",
			"method", r.Method, "path", sanitizeLog(r.URL.Path), "reason", sanitizeLog(fe.Msg))
		apierror.WriteJSON(w, http.StatusForbidden, fe.Msg)
	case errors.As(err, &nfe):
		slog.InfoContext(r.Context(), "not found",
			"method", r.Method, "path", sanitizeLog(r.URL.Path), "reason", sanitizeLog(nfe.Msg))
		apierror.WriteJSON(w, http.StatusNotFound, nfe.Msg)
	case errors.As(err, &ce):
		slog.WarnContext(r.Context(), "conflict",
			"method", r.Method, "path", sanitizeLog(r.URL.Path), "reason", sanitizeLog(ce.Msg))
		apierror.WriteJSON(w, http.StatusConflict, ce.Msg)
	case errors.As(err, &sue):
		slog.ErrorContext(r.Context(), "service unavailable",
			"method", r.Method, "path", sanitizeLog(r.URL.Path), "reason", sanitizeLog(sue.Msg))
		apierror.WriteJSON(w, http.StatusServiceUnavailable, "service temporarily unavailable, please try again later")
	case errors.Is(err, context.DeadlineExceeded):
		slog.WarnContext(r.Context(), "request timeout",
			"method", r.Method, "path", sanitizeLog(r.URL.Path))
		apierror.WriteJSON(w, http.StatusRequestTimeout, "request timed out")
	case errors.Is(err, context.Canceled):
		slog.InfoContext(r.Context(), "request canceled by the caller",
			"method", r.Method, "path", sanitizeLog(r.URL.Path))
	default:
		slog.ErrorContext(r.Context(), "internal error",
			"method", r.Method, "path", sanitizeLog(r.URL.Path), "err", sanitizeLog(err.Error()))
		apierror.WriteJSON(w, http.StatusInternalServerError, "internal server error")
	}
}

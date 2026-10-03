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

// Package server is the HTTP layer: the source webhook route, health endpoints, request ids, the body limit and the auth hook.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"sre-alert-ingestion-service/internal/model"
	"sre-alert-ingestion-service/internal/transport/auth"
)

// SourceRoutePrefix is the path every source webhook lives under.
const SourceRoutePrefix = "/api/wso2/v1/sre_alert_api/"

// retryAfterSeconds is sent with every 503 so sources back off before retrying.
const retryAfterSeconds = 60

// Request is an accepted source webhook handed to the Pipeline.
type Request struct {
	Source      string
	RequestID   string
	Route       string
	RemoteAddr  string
	ContentType string
	Body        []byte
}

// Result is the Pipeline's outcome: Status is one of 200, 201, 400 or 503; Error is the body message for 400/503.
type Result struct {
	Status int
	AltIDs []string
	Alerts []model.Alert
	Error  string
}

// Pipeline transforms and stores a source webhook; it must not let ctx cancel writes for ids it has already claimed.
type Pipeline interface {
	Ingest(ctx context.Context, req Request) Result
}

// Rejection describes a webhook answered 400 or 413. It never claimed an id.
type Rejection struct {
	Source      string
	Status      int
	Error       string
	Route       string
	RequestID   string
	RemoteAddr  string
	ContentType string
	// Body is the start of the request body (the card preview); BodySize is the full size.
	Body     []byte
	BodySize int64
	// SourceTotal is this replica's rejection count for Source since it started.
	SourceTotal int64
}

// RejectNotifier is told about every rejected webhook. It must not block the request.
type RejectNotifier interface {
	Rejected(Rejection)
}

// logPreviewChars bounds how much of a rejected body goes into the log line.
const logPreviewChars = 200

// Options configures a Server.
type Options struct {
	Logger       *slog.Logger
	Auth         auth.Authenticator
	Pipeline     Pipeline       // nil answers 503 on every source route
	Rejects      RejectNotifier // nil only logs rejections
	Sources      []string
	MaxBodyBytes int64
	// PreviewChars is how much of a body a rejection keeps (reject.body_preview_chars).
	PreviewChars int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// IdleTimeout closes keep-alive connections nobody is using, so idle source connections can't pile up.
	IdleTimeout time.Duration
}

// Server serves the source routes and health endpoints.
type Server struct {
	logger       *slog.Logger
	auth         auth.Authenticator
	pipeline     Pipeline
	rejects      RejectNotifier
	rejectCounts sync.Map // source -> *atomic.Int64
	sources      map[string]bool
	maxBodyBytes int64
	previewChars int
	draining     atomic.Bool
	handler      http.Handler
	readTimeout  time.Duration
	writeTimeout time.Duration
	idleTimeout  time.Duration
}

// New builds a Server; call Handler to mount it or ListenAndServe via HTTPServer.
func New(opts Options) *Server {
	s := &Server{
		logger:       opts.Logger,
		auth:         opts.Auth,
		pipeline:     opts.Pipeline,
		rejects:      opts.Rejects,
		sources:      make(map[string]bool, len(opts.Sources)),
		maxBodyBytes: opts.MaxBodyBytes,
		previewChars: opts.PreviewChars,
		readTimeout:  opts.ReadTimeout,
		writeTimeout: opts.WriteTimeout,
		idleTimeout:  opts.IdleTimeout,
	}
	for _, v := range opts.Sources {
		s.sources[v] = true
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", s.livez)
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc(SourceRoutePrefix+"{source}", s.sourceRoute)
	s.handler = s.withRequestID(s.withAccessLog(mux))
	return s
}

// Handler returns the root handler, with request-id and access-log middleware applied.
func (s *Server) Handler() http.Handler { return s.handler }

// HTTPServer returns an http.Server for addr with the configured read/write timeouts.
func (s *Server) HTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           s.handler,
		ReadTimeout:       s.readTimeout,
		ReadHeaderTimeout: s.readTimeout,
		WriteTimeout:      s.writeTimeout,
		IdleTimeout:       s.idleTimeout,
	}
}

// StartDraining makes /healthz answer 503 so the platform stops routing new traffic here.
func (s *Server) StartDraining() { s.draining.Store(true) }

// livez reports the process is up; it never checks dependencies, so a DB outage never restarts pods.
func (s *Server) livez(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// healthz reports readiness, 200 unless shutting down, without checking Postgres so pods stay in rotation during a DB outage.
func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	if s.draining.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) sourceRoute(w http.ResponseWriter, r *http.Request) {
	source := r.PathValue("source")
	info := requestInfoFrom(r.Context())
	info.source = source
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.sources[source] {
		writeJSON(w, http.StatusNotFound, rejected("unknown source"))
		return
	}

	// Before the body is read, so an unauthenticated caller costs only a header parse.
	if err := s.auth.Authenticate(r, source); err != nil {
		// GCP only sends webhook credentials after a 401 carrying this header.
		w.Header().Set("WWW-Authenticate", `Basic realm="sre_alert_api", charset="UTF-8"`)
		writeJSON(w, http.StatusUnauthorized, rejected("unauthorized"))
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, s.maxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			// Too large never reaches a transform, so it never claims an id.
			size := r.ContentLength
			if size < 0 {
				size = int64(len(body))
			}
			s.reject(r, source, http.StatusRequestEntityTooLarge, "payload too large", s.preview(body), size)
			writeJSON(w, http.StatusRequestEntityTooLarge, rejected("payload too large"))
			return
		}
		s.logger.Warn("webhook body read failed", "request_id", RequestID(r.Context()), "source", source, "error", err)
		writeJSON(w, http.StatusBadRequest, rejected("could not read request body"))
		return
	}

	if s.pipeline == nil {
		writeUnavailable(w, "ingestion not configured")
		return
	}
	// Only the preview is used after this, so the full body can be freed while Submit waits.
	preview, size := s.preview(body), int64(len(body))
	req := Request{
		Source:      source,
		RequestID:   RequestID(r.Context()),
		Route:       r.URL.Path,
		RemoteAddr:  r.RemoteAddr,
		ContentType: r.Header.Get("Content-Type"),
		Body:        body,
	}
	res := s.pipeline.Ingest(r.Context(), req)
	info.altIDs, info.err = res.AltIDs, res.Error
	switch res.Status {
	case http.StatusOK:
		writeJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	case http.StatusCreated:
		writeJSON(w, http.StatusCreated, map[string]string{"status": "stored"})
	case http.StatusBadRequest:
		s.reject(r, source, http.StatusBadRequest, res.Error, preview, size)
		writeJSON(w, http.StatusBadRequest, rejected(res.Error))
	default:
		writeUnavailable(w, res.Error)
	}
}

// reject logs a rejected webhook, counts it per source, and hands it to the RejectNotifier.
func (s *Server) reject(r *http.Request, source string, status int, msg, preview string, size int64) {
	counter, _ := s.rejectCounts.LoadOrStore(source, new(atomic.Int64))
	total := counter.(*atomic.Int64).Add(1)
	logPreview, _ := truncate(preview, logPreviewChars)
	s.logger.Warn("webhook rejected", "request_id", RequestID(r.Context()), "source", source,
		"status", status, "error", msg, "body_size", size, "body_preview", logPreview,
		"source_rejections_total", total)
	if s.rejects != nil {
		s.rejects.Rejected(Rejection{
			Source: source, Status: status, Error: msg, Route: r.URL.Path,
			RequestID: RequestID(r.Context()), RemoteAddr: r.RemoteAddr,
			ContentType: r.Header.Get("Content-Type"), Body: []byte(preview), BodySize: size, SourceTotal: total,
		})
	}
}

// preview keeps the start of body needed by the log line and the rejection notifier.
func (s *Server) preview(body []byte) string {
	p, _ := truncate(string(body), max(logPreviewChars, s.previewChars))
	return p
}

// truncate cuts s to n characters without splitting a multi-byte one.
func truncate(s string, n int) (string, bool) {
	if utf8.RuneCountInString(s) <= n {
		return s, false
	}
	return string([]rune(s)[:n]), true
}

func rejected(msg string) map[string]string {
	return map[string]string{"status": "rejected", "error": msg}
}

func writeUnavailable(w http.ResponseWriter, msg string) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type requestIDKey struct{}

// RequestIDHeader carries the request id in and out; an incoming value is reused end to end.
const RequestIDHeader = "X-Request-ID"

// maxIncomingRequestID bounds a caller-supplied id so it can't bloat logs.
const maxIncomingRequestID = 128

// RequestID returns the id withRequestID stored in ctx, or "" outside a request.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func (s *Server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" || len(id) > maxIncomingRequestID {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// requestInfo is filled in by the source route so the access log can carry the outcome.
type requestInfo struct {
	source string
	altIDs []string
	err    string
}

type requestInfoKey struct{}

func requestInfoFrom(ctx context.Context) *requestInfo {
	if info, ok := ctx.Value(requestInfoKey{}).(*requestInfo); ok {
		return info
	}
	return &requestInfo{}
}

// withAccessLog writes one line per request with the source, alt ids and latency, skipping health probes.
func (s *Server) withAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/livez" || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		info := &requestInfo{}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), requestInfoKey{}, info)))

		attrs := []any{"request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "duration_ms", time.Since(start).Milliseconds()}
		if info.source != "" {
			attrs = append(attrs, "source", info.source)
		}
		if len(info.altIDs) > 0 {
			attrs = append(attrs, "alt_ids", info.altIDs, "count", len(info.altIDs))
		}
		if info.err != "" {
			attrs = append(attrs, "error", info.err)
		}
		if rec.status >= http.StatusInternalServerError {
			s.logger.Warn("request", attrs...)
			return
		}
		s.logger.Info("request", attrs...)
	})
}

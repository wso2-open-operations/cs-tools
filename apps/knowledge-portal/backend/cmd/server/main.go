// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

// Command server runs the public Knowledge Base API. Everything it serves
// is public and read-only: published KB articles, knowledge bases, search,
// view counts, and related-article lookups. There is no authentication --
// per the Sep 30 decision, rating (the only feature that needed login) was
// dropped, so this whole service is unauthenticated by design.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/handler"
)

func mustEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	port := mustEnv("PORT", "8085")

	kb := handler.NewKBHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /public/knowledge-bases", kb.ListKnowledgeBases)
	mux.HandleFunc("POST /public/kb-articles/search", kb.SearchArticles)
	mux.HandleFunc("GET /public/kb-articles/{id}", kb.GetArticle)
	mux.HandleFunc("POST /public/kb-articles/{id}/view", kb.RecordView)
	mux.HandleFunc("GET /public/kb-articles/{id}/related", kb.RelatedArticles)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler.CORS(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	slog.Info("knowledge-portal backend started", "port", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

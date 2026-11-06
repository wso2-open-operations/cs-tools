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

// Command server polls a Gmail mailbox and publishes an event for each email
// that matches a configured rule.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-mail-paging-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-mail-paging-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-mail-paging-service/internal/gmail"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-mail-paging-service/internal/gmailauth"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-mail-paging-service/internal/poller"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-mail-paging-service/internal/rules"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "gmail-auth" {
		if err := gmailauth.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "gmail-auth:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("csm-mail-paging-service stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	rs, err := rules.Parse(cfg.RulesJSON)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mb, err := gmail.New(ctx, gmail.Config{
		ClientID:       cfg.GmailClientID,
		ClientSecret:   cfg.GmailClientSecret,
		RefreshToken:   cfg.GmailRefreshToken,
		ProcessedLabel: cfg.ProcessedLabel,
	})
	if err != nil {
		return err
	}
	if err := mb.EnsureLabel(ctx); err != nil {
		return err
	}
	prod := eventbus.NewProducer(eventbus.Config{Broker: cfg.KafkaBroker, ConnectionString: cfg.KafkaConnection, Topic: cfg.KafkaTopic})
	defer prod.Close()

	// The only HTTP surface is /health, Choreo's liveness probe.
	ln, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           healthMux(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("health server exited", "err", err)
			stop()
		}
	}()

	slog.Info("csm-mail-paging-service started", "rules", len(rs), "interval", cfg.PollInterval.String(), "port", cfg.Port)
	poller.New(mb, prod, rs, cfg.PollInterval, cfg.Lookback).Run(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// healthMux serves liveness only: a poller has no inbound API, and a Gmail
// outage must not get a healthy instance restarted.
func healthMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

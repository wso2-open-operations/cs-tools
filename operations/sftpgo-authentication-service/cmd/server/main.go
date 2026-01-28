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

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/service"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/util"

	applog "github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/log"
)

func main() {
	// 1. Load configuration from environment variables
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration: %v", err)
	}

	// 2. Initialize the custom logger
	logger := applog.NewAppLogger(cfg.LogLevel)

	// 3. Initialize email regex with custom pattern if provided
	if err := util.InitEmailRegex(cfg.EmailRegexPattern); err != nil {
		logger.Error("Failed to initialize email regex: %v", err)
	}

	// 4. Initialize database service
	dbService, err := service.NewDBService(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database: %v", err)
	}

	// 5. Initialize external API clients/services
	idpService := service.NewIdPService(cfg, logger)
	sftpgoService := service.NewSFTPGoService(cfg, logger)
	subscriptionService := service.NewSubscriptionService(cfg, logger)

	// 6. Initialize the HTTP handler, injecting all dependencies
	h := handler.NewHandler(cfg, logger, dbService, idpService, sftpgoService, subscriptionService)

	// 7. Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("/prelogin-hook", h.PreLoginHook)
	mux.HandleFunc("/auth-hook", h.AuthHandler)

	// 8. Start the server with graceful shutdown support
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	// Channel to listen for shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("SFTPGo Hooks Server listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop

	logger.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited gracefully")
}

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

// Command kbmigrate performs a one-shot backfill of ServiceNow Knowledge
// Management data (kb_knowledge, kb_knowledge_base -- the DEV tenant,
// wso2sndev.service-now.com, only, and only ever read from) into
// entity-service's own already-built kb_articles/kb_article_history/
// knowledge_bases/kb_managers schema (migrations/000020-000030).
//
// This is a standalone backfill, not a long-running service: it runs to
// completion, prints a summary, and exits. It is explicitly segregated from
// digiops-cs/operations/csm-sync-service -- a separate, continuously-syncing
// tool in a different repo with its own differently-shaped schema -- and
// does not import or depend on that repo's code in any way.
//
// Safe to re-run: every insert is idempotent (see store.go's
// articleExistsBySourceSysID/historyExistsBySourceSysID, and
// upsertKnowledgeBase's dedupe-by-name), so a partial or interrupted run can
// simply be re-invoked.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("load .env: %v", err)
	}

	toolCfg, err := loadToolConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	entityCfg := config.Load()
	if !entityCfg.HasDatabase() {
		log.Fatalf("DB_USER, DB_PASSWORD, and DB_NAME must all be set -- this tool writes directly to entity-service's own database")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	entityPool, err := db.NewPoolFromConfig(entityCfg)
	if err != nil {
		log.Fatalf("connect to entity-service database: %v", err)
	}
	defer entityPool.Close()

	csmSyncPool, err := pgxpool.New(ctx, toolCfg.CSMSyncDatabaseURL)
	if err != nil {
		log.Fatalf("construct csm-sync-service database pool: %v", err)
	}
	defer csmSyncPool.Close()
	if err := csmSyncPool.Ping(ctx); err != nil {
		log.Fatalf("ping csm-sync-service database (read-only dependency): %v", err)
	}

	snHTTPClient := &http.Client{Timeout: 60 * time.Second}
	sn := newSNClient(toolCfg.ServiceNowInstance, toolCfg.ServiceNowClientID, toolCfg.ServiceNowClientSecret, snHTTPClient)

	store := newKBStore(entityPool)
	users := newRealUserResolver(csmSyncPool, entityPool)
	cases := newRealCaseResolver(entityPool)

	log.Printf("kbmigrate: starting one-shot ServiceNow Knowledge Management backfill against %s", toolCfg.ServiceNowInstance)
	sum, err := runMigration(ctx, sn, store, users, cases)
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	printSummary(sum)
}

func printSummary(s summary) {
	fmt.Println()
	fmt.Println("=== kbmigrate run summary ===")
	fmt.Printf("Knowledge bases created: %d, reused (already existed by name): %d\n", s.KnowledgeBasesCreated, s.KnowledgeBasesReused)
	fmt.Printf("KB managers inserted: %d, skipped: %d\n", s.KBManagersInserted, len(s.KBManagersSkipped))
	fmt.Printf("Articles inserted: %d, skipped: %d\n", s.ArticlesInserted, len(s.ArticlesSkipped))
	fmt.Printf("History rows inserted: %d, skipped: %d\n", s.HistoryRowsInserted, len(s.HistoryRowsSkipped))
	fmt.Printf("Version-chain warnings: %d\n", len(s.ChainWarnings))

	printSkips := func(label string, records []skipRecord) {
		if len(records) == 0 {
			return
		}
		fmt.Printf("\n-- %s --\n", label)
		for _, r := range records {
			fmt.Printf("  %s: %s\n", r.SysID, r.Reason)
		}
	}
	printSkips("Skipped KB managers", s.KBManagersSkipped)
	printSkips("Skipped articles", s.ArticlesSkipped)
	printSkips("Skipped history rows", s.HistoryRowsSkipped)
	printSkips("Chain walk warnings", s.ChainWarnings)
}

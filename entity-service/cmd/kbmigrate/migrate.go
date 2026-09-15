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
	"fmt"
	"log"
)

// skipRecord logs one row this run declined to migrate, and why.
type skipRecord struct {
	SysID  string
	Reason string
}

// summary tallies what one migration run did, for the final printed report.
type summary struct {
	KnowledgeBasesCreated int
	KnowledgeBasesReused  int
	KBManagersInserted    int
	KBManagersSkipped     []skipRecord

	ArticlesInserted int
	ArticlesSkipped  []skipRecord // already migrated, unresolved author, or bad state

	HistoryRowsInserted int
	HistoryRowsSkipped  []skipRecord

	ChainWarnings []skipRecord // non-fatal base_version walk anomalies (chain.go)
}

func (s *summary) logAndSkipArticle(sysID, reason string) {
	s.ArticlesSkipped = append(s.ArticlesSkipped, skipRecord{SysID: sysID, Reason: reason})
	log.Printf("SKIP kb_knowledge %s: %s", sysID, reason)
}

func (s *summary) logAndSkipHistory(sysID, reason string) {
	s.HistoryRowsSkipped = append(s.HistoryRowsSkipped, skipRecord{SysID: sysID, Reason: reason})
	log.Printf("SKIP kb_article_history for kb_knowledge %s: %s", sysID, reason)
}

func (s *summary) logAndSkipManager(sysID, reason string) {
	s.KBManagersSkipped = append(s.KBManagersSkipped, skipRecord{SysID: sysID, Reason: reason})
	log.Printf("SKIP kb_manager sys_user %s: %s", sysID, reason)
}

// runMigration is the top-level backfill orchestrator: extract every
// kb_knowledge_base and kb_knowledge row from ServiceNow, then migrate each
// knowledge base (and its managers) and each article chain
// (kb_articles + kb_article_history) into entity-service's own database,
// idempotently and without ever aborting the whole run over one bad row.
func runMigration(ctx context.Context, sn *snClient, store *kbStore, users userResolver, cases caseResolver) (summary, error) {
	var sum summary

	knowledgeBases, err := sn.fetchKnowledgeBases(ctx)
	if err != nil {
		return sum, fmt.Errorf("fetch kb_knowledge_base: %w", err)
	}
	articles, err := sn.fetchKnowledgeArticles(ctx)
	if err != nil {
		return sum, fmt.Errorf("fetch kb_knowledge: %w", err)
	}

	// In-memory index for the version-chain walk: the full unfiltered
	// extraction already pulled every row, so no further network calls are
	// needed while walking base_version links backward.
	bySysID := make(map[string]snKnowledge, len(articles))
	for _, a := range articles {
		bySysID[a.SysID] = a
	}
	lookup := func(sysID string) (snKnowledge, bool, error) {
		row, ok := bySysID[sysID]
		return row, ok, nil
	}

	kbSysIDToLocalID := make(map[string]string, len(knowledgeBases))
	for _, kb := range knowledgeBases {
		localID, created, err := store.upsertKnowledgeBase(ctx, kb)
		if err != nil {
			return sum, fmt.Errorf("upsert knowledge_base %s (%q): %w", kb.SysID, kb.Title, err)
		}
		kbSysIDToLocalID[kb.SysID] = localID
		if created {
			sum.KnowledgeBasesCreated++
		} else {
			sum.KnowledgeBasesReused++
		}

		for _, managerSysID := range kb.KBManagers {
			userID, err := users.ResolveUser(ctx, managerSysID)
			if err != nil {
				sum.logAndSkipManager(managerSysID, err.Error())
				continue
			}
			inserted, err := store.insertKBManager(ctx, localID, userID)
			if err != nil {
				return sum, fmt.Errorf("insert kb_manager for kb %s: %w", kb.SysID, err)
			}
			if inserted {
				sum.KBManagersInserted++
			}
		}
	}

	for _, article := range articles {
		if !article.Latest {
			continue // only latest=true rows seed a chain; see chain.go
		}
		if err := migrateOneChain(ctx, article, lookup, store, users, cases, kbSysIDToLocalID, &sum); err != nil {
			return sum, fmt.Errorf("migrate article chain rooted at %s: %w", article.SysID, err)
		}
	}

	return sum, nil
}

// migrateOneChain migrates a single "latest=true" kb_knowledge row and every
// historical version reachable by walking its base_version chain. Any
// row-level problem (unresolved user, unrecognised state, already migrated)
// skips just that row/chain and is logged -- never returns an error for
// those; migrateOneChain only returns an error for an actual DB failure.
func migrateOneChain(
	ctx context.Context,
	latest snKnowledge,
	lookup kbLookup,
	store *kbStore,
	users userResolver,
	cases caseResolver,
	kbSysIDToLocalID map[string]string,
	sum *summary,
) error {
	alreadyMigrated, err := store.articleExistsBySourceSysID(ctx, latest.SysID)
	if err != nil {
		return err
	}
	if alreadyMigrated {
		sum.logAndSkipArticle(latest.SysID, "already migrated (source_sys_id already present in kb_articles)")
		return nil
	}

	knowledgeBaseID, ok := kbSysIDToLocalID[latest.KnowledgeBase]
	if !ok {
		sum.logAndSkipArticle(latest.SysID, fmt.Sprintf("parent kb_knowledge_base %q was not migrated (not found among extracted knowledge bases)", latest.KnowledgeBase))
		return nil
	}

	authorID, err := users.ResolveUser(ctx, latest.Author)
	if err != nil {
		// author_id is NOT NULL -- an unresolved author blocks the whole
		// article, per the spec.
		sum.logAndSkipArticle(latest.SysID, fmt.Sprintf("author unresolved: %v", err))
		return nil
	}

	// updated_by is nullable: an unresolved revised_by degrades to NULL
	// rather than blocking the article (soft-fail), unlike author_id above.
	var updatedBy *string
	if latest.RevisedBy != "" {
		id, err := users.ResolveUser(ctx, latest.RevisedBy)
		if err != nil {
			log.Printf("WARN kb_knowledge %s: updated_by (revised_by %s) unresolved, leaving NULL: %v", latest.SysID, latest.RevisedBy, err)
		} else {
			updatedBy = &id
		}
	}

	sourceCaseID, err := resolveOptionalCase(ctx, cases, latest.CaseID)
	if err != nil {
		return err
	}

	plan, err := buildArticlePlan(latest, knowledgeBaseID, authorID, updatedBy, sourceCaseID)
	if err != nil {
		sum.logAndSkipArticle(latest.SysID, err.Error())
		return nil
	}

	kbArticleID, err := store.insertArticle(ctx, plan)
	if err != nil {
		return err
	}
	sum.ArticlesInserted++

	history, warning := walkVersionChain(latest, lookup)
	if warning != "" {
		sum.ChainWarnings = append(sum.ChainWarnings, skipRecord{SysID: latest.SysID, Reason: warning})
		log.Printf("WARN chain walk for %s: %s", latest.SysID, warning)
	}

	for _, histRow := range history {
		if err := migrateOneHistoryRow(ctx, histRow, kbArticleID, store, users, sum); err != nil {
			return err
		}
	}
	return nil
}

// migrateOneHistoryRow migrates one historical chain row into
// kb_article_history, skipping (and logging) it alone -- never the whole
// chain -- on an idempotency hit, unresolved changed_by, or bad state.
func migrateOneHistoryRow(ctx context.Context, row snKnowledge, kbArticleID string, store *kbStore, users userResolver, sum *summary) error {
	alreadyMigrated, err := store.historyExistsBySourceSysID(ctx, row.SysID)
	if err != nil {
		return err
	}
	if alreadyMigrated {
		sum.logAndSkipHistory(row.SysID, "already migrated (source_sys_id already present in kb_article_history)")
		return nil
	}

	changedBySysID := historyChangedBySource(row)
	changedBy, err := users.ResolveUser(ctx, changedBySysID)
	if err != nil {
		// changed_by is NOT NULL -- an unresolved user blocks just this one
		// history row, not the article it belongs to.
		sum.logAndSkipHistory(row.SysID, fmt.Sprintf("changed_by unresolved: %v", err))
		return nil
	}

	plan, err := buildHistoryPlan(row, changedBy)
	if err != nil {
		sum.logAndSkipHistory(row.SysID, err.Error())
		return nil
	}

	if err := store.insertHistory(ctx, kbArticleID, plan); err != nil {
		return err
	}
	sum.HistoryRowsInserted++
	return nil
}

// resolveOptionalCase resolves an optional u_case_id sys_id to a
// source_case_id pointer, or nil if snCaseSysID is empty or the derived
// case UUID does not exist in entity-service's own cases table -- both are
// treated the same way since source_case_id is nullable, not a hard error.
func resolveOptionalCase(ctx context.Context, cases caseResolver, snCaseSysID string) (*string, error) {
	if snCaseSysID == "" {
		return nil, nil
	}
	uuid, exists, err := cases.ResolveCase(ctx, snCaseSysID)
	if err != nil {
		return nil, fmt.Errorf("resolve case %s: %w", snCaseSysID, err)
	}
	if !exists {
		log.Printf("INFO u_case_id %s does not correspond to a migrated case; leaving source_case_id NULL", snCaseSysID)
		return nil, nil
	}
	return &uuid, nil
}

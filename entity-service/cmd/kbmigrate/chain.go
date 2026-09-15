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

import "fmt"

// kbLookup fetches a single kb_knowledge row by sys_id. The real
// implementation (snclient.go) hits the ServiceNow Table API per sys_id
// while walking a chain backward; tests supply an in-memory map-backed fake.
type kbLookup func(sysID string) (snKnowledge, bool, error)

// walkVersionChain walks a "latest=true" kb_knowledge row's base_version
// links backward to the chain's root, collecting every row visited along the
// way (excluding latest itself). Each collected row becomes one
// kb_article_history row, all pointing at the same kb_articles row (the one
// generated for latest).
//
// A row with an empty base_version ends the walk there (that row is the
// chain's root and is still collected, unless it IS latest itself, in which
// case the chain has no history at all -- an article that was never
// revised, which is normal).
//
// Cycle protection: base_version links are supposed to form a simple
// backward chain with no cycles, but SN data quality is not guaranteed, so
// visited sys_ids are tracked and a cycle stops the walk (see below).
//
// Interpretation not spelled out in the migration spec: a broken chain link
// (a base_version sys_id that can't be fetched, or a cycle) does not fail
// the whole article -- the latest row is a complete, independently valid
// kb_articles row regardless of whether its history is fully recoverable.
// So walkVersionChain never returns an error; a broken link simply truncates
// the walk at that point and is reported back via the returned warning
// string for the caller to log, matching the spec's general "skip/log,
// never abort" stance on row-level data problems elsewhere (user/case id
// resolution).
func walkVersionChain(latest snKnowledge, lookup kbLookup) (history []snKnowledge, warning string) {
	visited := map[string]bool{latest.SysID: true}

	current := latest
	for current.BaseVersion != "" {
		prevSysID := current.BaseVersion
		if visited[prevSysID] {
			return history, fmt.Sprintf("cycle detected in base_version chain at sys_id %q (chain root: %q); history truncated", prevSysID, latest.SysID)
		}
		prev, ok, err := lookup(prevSysID)
		if err != nil {
			return history, fmt.Sprintf("fetch base_version %q failed: %v (chain root: %q); history truncated", prevSysID, err, latest.SysID)
		}
		if !ok {
			return history, fmt.Sprintf("base_version %q not found (chain root: %q); history truncated", prevSysID, latest.SysID)
		}
		visited[prevSysID] = true
		history = append(history, prev)
		current = prev
	}
	return history, ""
}

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
	"strconv"
	"strings"
	"time"
)

// glideDateTimeLayout is ServiceNow's glide_date_time wire format, always UTC.
const glideDateTimeLayout = "2006-01-02 15:04:05"

// parseGlideDateTime parses a ServiceNow sys_created_on/sys_updated_on value.
func parseGlideDateTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty glide_date_time value")
	}
	t, err := time.Parse(glideDateTimeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse glide_date_time %q: %w", s, err)
	}
	return t.UTC(), nil
}

// parseGlideBool parses ServiceNow's boolean-as-string wire format ("true"/"false").
// Any value other than exactly "true" is treated as false, matching SN's own
// convention (an empty/unset boolean field serializes as "false").
func parseGlideBool(s string) bool {
	return s == "true"
}

// kbKnowledgeBaseFields lists the exact kb_knowledge_base fields requested
// from the Table API (see snClient.fetchAllRows).
var kbKnowledgeBaseFields = []string{
	"sys_id", "title", "active", "kb_managers", "sys_created_on", "sys_updated_on",
}

// kbKnowledgeFields lists the exact kb_knowledge fields requested from the
// Table API (see snClient.fetchAllRows).
var kbKnowledgeFields = []string{
	"sys_id", "number", "short_description", "text", "workflow_state",
	"author", "revised_by", "kb_knowledge_base", "u_case_id", "u_reject_reason",
	"generated_with_now_assist", "u_generated_by", "helpful_count", "rating",
	"use_count", "sys_view_count", "base_version", "latest",
	"sys_created_on", "sys_updated_on",
}

// convertKnowledgeBase converts one raw kb_knowledge_base Table API row into
// an snKnowledgeBase. kb_managers is a comma-separated glide_list of
// sys_user sys_ids and may be empty.
func convertKnowledgeBase(row map[string]string) (snKnowledgeBase, error) {
	createdOn, err := parseGlideDateTime(row["sys_created_on"])
	if err != nil {
		return snKnowledgeBase{}, fmt.Errorf("kb_knowledge_base %s: %w", row["sys_id"], err)
	}
	updatedOn, err := parseGlideDateTime(row["sys_updated_on"])
	if err != nil {
		return snKnowledgeBase{}, fmt.Errorf("kb_knowledge_base %s: %w", row["sys_id"], err)
	}

	var managers []string
	if raw := strings.TrimSpace(row["kb_managers"]); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				managers = append(managers, id)
			}
		}
	}

	return snKnowledgeBase{
		SysID:      row["sys_id"],
		Title:      row["title"],
		Active:     parseGlideBool(row["active"]),
		KBManagers: managers,
		CreatedOn:  createdOn,
		UpdatedOn:  updatedOn,
	}, nil
}

// convertKnowledge converts one raw kb_knowledge Table API row into an snKnowledge.
func convertKnowledge(row map[string]string) (snKnowledge, error) {
	createdOn, err := parseGlideDateTime(row["sys_created_on"])
	if err != nil {
		return snKnowledge{}, fmt.Errorf("kb_knowledge %s: %w", row["sys_id"], err)
	}
	updatedOn, err := parseGlideDateTime(row["sys_updated_on"])
	if err != nil {
		return snKnowledge{}, fmt.Errorf("kb_knowledge %s: %w", row["sys_id"], err)
	}

	helpfulCount, _ := strconv.Atoi(row["helpful_count"])
	useCount, _ := strconv.Atoi(row["use_count"])
	viewCount, _ := strconv.Atoi(row["sys_view_count"])

	var rating *float64
	if raw := strings.TrimSpace(row["rating"]); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil {
			rating = &v
		}
	}

	return snKnowledge{
		SysID:                  row["sys_id"],
		Number:                 row["number"],
		ShortDescription:       row["short_description"],
		Text:                   row["text"],
		WorkflowState:          row["workflow_state"],
		Author:                 row["author"],
		RevisedBy:              row["revised_by"],
		KnowledgeBase:          row["kb_knowledge_base"],
		CaseID:                 row["u_case_id"],
		RejectReason:           row["u_reject_reason"],
		GeneratedWithNowAssist: parseGlideBool(row["generated_with_now_assist"]),
		GeneratedBy:            row["u_generated_by"],
		HelpfulCount:           helpfulCount,
		Rating:                 rating,
		UseCount:               useCount,
		ViewCount:              viewCount,
		BaseVersion:            row["base_version"],
		Latest:                 parseGlideBool(row["latest"]),
		CreatedOn:              createdOn,
		UpdatedOn:              updatedOn,
	}, nil
}

// fetchKnowledgeBases retrieves every kb_knowledge_base row (unfiltered, full
// extraction, per the spec).
func (c *snClient) fetchKnowledgeBases(ctx context.Context) ([]snKnowledgeBase, error) {
	rows, err := c.fetchAllRows(ctx, "kb_knowledge_base", kbKnowledgeBaseFields)
	if err != nil {
		return nil, err
	}
	out := make([]snKnowledgeBase, 0, len(rows))
	for _, row := range rows {
		kb, err := convertKnowledgeBase(row)
		if err != nil {
			return nil, err
		}
		out = append(out, kb)
	}
	return out, nil
}

// fetchKnowledgeArticles retrieves every kb_knowledge row (unfiltered, full
// extraction, per the spec) -- both latest=true rows and every historical
// version, since the version-chain walk needs the full set to resolve
// base_version links by sys_id.
func (c *snClient) fetchKnowledgeArticles(ctx context.Context) ([]snKnowledge, error) {
	rows, err := c.fetchAllRows(ctx, "kb_knowledge", kbKnowledgeFields)
	if err != nil {
		return nil, err
	}
	out := make([]snKnowledge, 0, len(rows))
	for _, row := range rows {
		k, err := convertKnowledge(row)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, nil
}

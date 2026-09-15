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
	"testing"
	"time"
)

func TestParseGlideDateTime(t *testing.T) {
	got, err := parseGlideDateTime("2026-03-14 09:26:53")
	if err != nil {
		t.Fatalf("parseGlideDateTime: %v", err)
	}
	want := time.Date(2026, 3, 14, 9, 26, 53, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if _, err := parseGlideDateTime(""); err == nil {
		t.Error("expected an error for an empty value")
	}
	if _, err := parseGlideDateTime("not-a-date"); err == nil {
		t.Error("expected an error for a malformed value")
	}
}

func TestParseGlideBool(t *testing.T) {
	if !parseGlideBool("true") {
		t.Error(`parseGlideBool("true") = false, want true`)
	}
	if parseGlideBool("false") {
		t.Error(`parseGlideBool("false") = true, want false`)
	}
	if parseGlideBool("") {
		t.Error(`parseGlideBool("") = true, want false`)
	}
}

func TestConvertKnowledgeBase_SplitsManagerList(t *testing.T) {
	row := map[string]string{
		"sys_id":         "abcd1234abcd1234abcd1234abcd1234",
		"title":          "API Manager KB",
		"active":         "true",
		"kb_managers":    "1111111111111111111111111111aaaa, 2222222222222222222222222222bbbb",
		"sys_created_on": "2025-01-01 00:00:00",
		"sys_updated_on": "2025-06-01 12:00:00",
	}
	kb, err := convertKnowledgeBase(row)
	if err != nil {
		t.Fatalf("convertKnowledgeBase: %v", err)
	}
	if kb.Title != "API Manager KB" || !kb.Active {
		t.Errorf("kb = %+v", kb)
	}
	if len(kb.KBManagers) != 2 {
		t.Fatalf("expected 2 managers, got %d: %v", len(kb.KBManagers), kb.KBManagers)
	}
	if kb.KBManagers[0] != "1111111111111111111111111111aaaa" || kb.KBManagers[1] != "2222222222222222222222222222bbbb" {
		t.Errorf("unexpected managers, whitespace not trimmed? got %v", kb.KBManagers)
	}
}

func TestConvertKnowledgeBase_EmptyManagerList(t *testing.T) {
	row := map[string]string{
		"sys_id":         "abcd1234abcd1234abcd1234abcd1234",
		"title":          "No managers KB",
		"active":         "false",
		"kb_managers":    "",
		"sys_created_on": "2025-01-01 00:00:00",
		"sys_updated_on": "2025-01-01 00:00:00",
	}
	kb, err := convertKnowledgeBase(row)
	if err != nil {
		t.Fatalf("convertKnowledgeBase: %v", err)
	}
	if len(kb.KBManagers) != 0 {
		t.Errorf("expected no managers, got %v", kb.KBManagers)
	}
	if kb.Active {
		t.Error("expected Active=false")
	}
}

func TestConvertKnowledge_FullRow(t *testing.T) {
	row := map[string]string{
		"sys_id":                    "aaaa1111aaaa1111aaaa1111aaaa1111",
		"number":                    "KB0001234",
		"short_description":         "How to configure X",
		"text":                      "<p>Body HTML</p>",
		"workflow_state":            "published",
		"author":                    "bbbb2222bbbb2222bbbb2222bbbb2222",
		"revised_by":                "cccc3333cccc3333cccc3333cccc3333",
		"kb_knowledge_base":         "dddd4444dddd4444dddd4444dddd4444",
		"u_case_id":                 "eeee5555eeee5555eeee5555eeee5555",
		"u_reject_reason":           "",
		"generated_with_now_assist": "true",
		"u_generated_by":            "now_assist",
		"helpful_count":             "12",
		"rating":                    "4.5",
		"use_count":                 "99",
		"sys_view_count":            "500",
		"base_version":              "ffff6666ffff6666ffff6666ffff6666",
		"latest":                    "true",
		"sys_created_on":            "2025-01-01 00:00:00",
		"sys_updated_on":            "2025-02-01 00:00:00",
	}
	k, err := convertKnowledge(row)
	if err != nil {
		t.Fatalf("convertKnowledge: %v", err)
	}
	if k.ShortDescription != "How to configure X" || k.Text != "<p>Body HTML</p>" {
		t.Errorf("title/body not mapped correctly: %+v", k)
	}
	if k.Author != row["author"] || k.RevisedBy != row["revised_by"] {
		t.Errorf("author/revised_by not passed through as raw sys_ids: %+v", k)
	}
	if !k.GeneratedWithNowAssist || k.GeneratedBy != "now_assist" {
		t.Errorf("AI fields wrong: %+v", k)
	}
	if k.HelpfulCount != 12 || k.UseCount != 99 || k.ViewCount != 500 {
		t.Errorf("usage metrics wrong: %+v", k)
	}
	if k.Rating == nil || *k.Rating != 4.5 {
		t.Errorf("rating wrong: %v", k.Rating)
	}
	if !k.Latest {
		t.Error("expected Latest=true")
	}
}

func TestConvertKnowledge_EmptyOptionalReferenceFields(t *testing.T) {
	row := map[string]string{
		"sys_id":                    "aaaa1111aaaa1111aaaa1111aaaa1111",
		"short_description":         "Never revised article",
		"text":                      "<p>Body</p>",
		"workflow_state":            "draft",
		"author":                    "bbbb2222bbbb2222bbbb2222bbbb2222",
		"revised_by":                "",
		"kb_knowledge_base":         "dddd4444dddd4444dddd4444dddd4444",
		"u_case_id":                 "",
		"generated_with_now_assist": "false",
		"u_generated_by":            "",
		"helpful_count":             "0",
		"rating":                    "",
		"use_count":                 "0",
		"sys_view_count":            "0",
		"base_version":              "",
		"latest":                    "true",
		"sys_created_on":            "2025-01-01 00:00:00",
		"sys_updated_on":            "2025-01-01 00:00:00",
	}
	k, err := convertKnowledge(row)
	if err != nil {
		t.Fatalf("convertKnowledge: %v", err)
	}
	if k.RevisedBy != "" || k.CaseID != "" || k.BaseVersion != "" {
		t.Errorf("expected empty optional reference fields to stay empty, got %+v", k)
	}
	if k.Rating != nil {
		t.Errorf("expected nil rating for an empty string, got %v", *k.Rating)
	}
}

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

import "time"

// snKnowledgeBase is one row of ServiceNow's kb_knowledge_base table, with
// only the fields this backfill needs. Every reference field is the raw
// sys_id string -- sysparm_display_value is never requested (see snclient.go).
type snKnowledgeBase struct {
	SysID      string    // sys_id
	Title      string    // title -> knowledge_bases.name
	Active     bool      // active -> knowledge_bases.is_active
	KBManagers []string  // kb_managers (glide_list) split into individual sys_user sys_ids
	CreatedOn  time.Time // sys_created_on
	UpdatedOn  time.Time // sys_updated_on
}

// snKnowledge is one row of ServiceNow's kb_knowledge table -- one version in
// an article's revision chain. Only the "latest=true" row per chain becomes
// a kb_articles row; every other row visited while walking base_version
// becomes a kb_article_history row (see chain.go).
type snKnowledge struct {
	SysID                  string    // sys_id -- this row's own origin identity, stored as source_sys_id
	Number                 string    // number (not persisted; kept for logging)
	ShortDescription       string    // short_description -> title
	Text                   string    // text (HTML body) -> body -- NOT the wiki field
	WorkflowState          string    // workflow_state -> collapsed via collapseWorkflowState
	Author                 string    // author (sys_user sys_id, reference raw value)
	RevisedBy              string    // revised_by (sys_user sys_id, reference raw value, may be empty)
	KnowledgeBase          string    // kb_knowledge_base (sys_id of the parent KB)
	CaseID                 string    // u_case_id (sn_customerservice_case sys_id, may be empty)
	RejectReason           string    // u_reject_reason
	GeneratedWithNowAssist bool      // generated_with_now_assist
	GeneratedBy            string    // u_generated_by (choice value, may be empty)
	HelpfulCount           int       // helpful_count
	Rating                 *float64  // rating (nullable numeric)
	UseCount               int       // use_count
	ViewCount              int       // sys_view_count
	BaseVersion            string    // base_version (immediate predecessor's sys_id, may be empty)
	Latest                 bool      // latest
	CreatedOn              time.Time // sys_created_on
	UpdatedOn              time.Time // sys_updated_on
}

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

// ============================================================================
// ServiceNow REST Message Configuration Guide
// ============================================================================
//
// PURPOSE
//   This file is NOT executable code. It is a step-by-step checklist for
//   creating the ServiceNow REST Message record that every MTTR component
//   (scheduled job, script include, widgets) uses to talk to the Choreo
//   MTTR API.
//
//   Open this file, work top-to-bottom, and create the corresponding
//   records in your ServiceNow instance. Phase 3 of `docs/setup-guide.md`
//   walks through the same steps in the broader deployment context.
//
// CONTENTS
//   Step 1 — Create the REST Message record
//   Step 2 — Configure the OAuth 2.0 profile
//   Step 3 — Create six HTTP Methods on the REST Message
//   Step 4 — Create the watermark system property
// ============================================================================

// ─── Step 1: Create the REST Message ────────────────────────────────────────
//
//   Navigation: System Web Services → Outbound → REST Message
//
//     Name:                 MTTR_Choreo_API
//     Endpoint:             https://<your-choreo-app-url>/api/v1
//     Authentication Type:  OAuth 2.0
//     OAuth Profile:        (created in Step 2 below)

// ─── Step 2: Create the OAuth 2.0 Profile ──────────────────────────────────
//
//   Navigation: System OAuth → Application Registry
//               (or: System Web Services → OAuth → OAuth Profile)
//
//     Name:          MTTR Choreo OAuth
//     Grant Type:    Client Credentials
//     Token URL:     https://sts.choreo.dev/oauth2/token
//                    (replace with your Choreo org's token endpoint)
//     Client ID:     <from the Choreo application credentials>
//     Client Secret: <from the Choreo application credentials>
//     Scope:         (leave empty unless Choreo requires a specific scope)

// ─── Step 3: Create the HTTP Methods ───────────────────────────────────────
//
//   On the REST Message record from Step 1, add SIX HTTP Methods. Their
//   names are referenced by code throughout this folder, so match them
//   exactly.
//
//   ── Method 1: POST_Batch ──────────────────────────────────────────────
//     HTTP Method:   POST
//     Endpoint:      https://<your-choreo-app-url>/api/v1/cases/batch
//     Content-Type:  application/json
//     Used by:       scheduled-job-mttr-push.js
//     Notes:         Body is set dynamically per request.
//
//   ── Method 2: POST_BulkImport ─────────────────────────────────────────
//     HTTP Method:   POST
//     Endpoint:      https://<your-choreo-app-url>/api/v1/cases/bulk-import
//     Content-Type:  application/json
//     Used by:       one-off historical backfills (manual operator use)
//
//   ── Method 3: GET_MTTR ────────────────────────────────────────────────
//     HTTP Method:   GET
//     Endpoint:      https://<your-choreo-app-url>/api/v1/mttr?type=${type}
//     Used by:       MttrDashboardAPI._fetchDimension()
//     Notes:         ${type} is substituted at runtime via setStringParameter (URL-encoded).
//
//   ── Method 4: GET_Health ──────────────────────────────────────────────
//     HTTP Method:   GET
//     Endpoint:      https://<your-choreo-app-url>/api/v1/health
//     Used by:       optional health-check widgets / connectivity tests
//
//   ── Method 5: POST_CacheReset ─────────────────────────────────────────
//     HTTP Method:   POST
//     Endpoint:      https://<your-choreo-app-url>/api/v1/admin/cache/reset
//     Used by:       operator-triggered "Reset Cache" UI buttons
//     Notes:         Requires the admin role in the OAuth client.
//
//   ── Method 6: GET_Summary ─────────────────────────────────────────────
//     HTTP Method:   GET
//     Endpoint:      https://<your-choreo-app-url>/api/v1/summary/historical?case_type=${case_type}&group_by=${group_by}
//     Used by:       MttrDashboardAPI._fetchSummary() for quarterly trends
//     Notes:         Both ${case_type} and ${group_by} are substituted at runtime.

// ─── Step 4: Create the watermark system properties ────────────────────────
//
//   The scheduled job tracks a COMPOUND cursor (sys_updated_on, sys_id) so
//   it can advance through groups of rows that share the same
//   sys_updated_on (otherwise setLimit(500) can leave rows in a tie stranded).
//   Create BOTH properties below.
//
//   Navigation: sys_properties.list
//
//   Property 1:
//     Name:   u_mttr_last_sync
//     Type:   string
//     Value:  (leave empty — the scheduled job sets it on first run)
//
//   Property 2:
//     Name:   u_mttr_last_sync_sys_id
//     Type:   string
//     Value:  (leave empty — the scheduled job sets it on first run)
//
//   Why system properties and not a table column?
//     A system property is the simplest SN-native key/value store and is
//     cheap to read/write inside a scheduled job. Two properties keep the
//     cursor typed and human-readable in the ServiceNow UI.
//
// ============================================================================

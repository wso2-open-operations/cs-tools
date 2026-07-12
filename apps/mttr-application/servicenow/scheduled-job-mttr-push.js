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
// ServiceNow Scheduled Job: MTTR — Push Resolved Cases to Choreo
// ============================================================================
//
// PURPOSE
//   Every 10 minutes this job pulls newly Closed/Resolved customer-service
//   cases from `sn_customerservice_case` and forwards them to the Choreo
//   MTTR API (/api/v1/cases/batch). The MTTR app stores them in
//   `case_events` and uses them to recompute the dashboards.
//
// AT-A-GLANCE
//   1. Read the watermark (`u_mttr_last_sync` system property)
//   2. Query up to 500 cases updated since the watermark
//   3. Validate each row & build the batch payload (skip invalid rows)
//   4. POST the payload to Choreo
//   5. ONLY on HTTP 200 → advance the watermark (at-least-once delivery)
//
// CONFIGURATION (in ServiceNow)
//   Name:     MTTR - Push Resolved Cases
//   Run:      Every 10 minutes
//   Active:   true
//
// PREREQUISITES
//   1. Two System Properties (auto-created on the first run):
//      • `u_mttr_last_sync`         — timestamp part of the cursor
//      • `u_mttr_last_sync_sys_id`  — sys_id tie-breaker for rows sharing
//                                      that timestamp (see Step 2 comment)
//   2. REST Message `MTTR_Choreo_API` with method `POST_Batch`:
//      • Endpoint:      https://<choreo-app>/api/v1/cases/batch
//      • Auth:          OAuth 2.0 (client_credentials)
//      • HTTP method:   POST
//      • Content-Type:  application/json
// ============================================================================

(function () {
    // ─── Tunables ────────────────────────────────────────────────────────
    // Keep BATCH_SIZE aligned with the Choreo /cases/batch endpoint cap.
    var MAX_CASES_PER_BATCH = 500;

    // Watermark = a COMPOUND CURSOR (sys_updated_on, sys_id).
    // We need both because sys_updated_on is not unique: many rows can share
    // the same value, and with setLimit(500) a group of ties can straddle the
    // batch boundary. Tracking sys_id as a tie-breaker gives us a strictly
    // monotonic cursor that always advances even when 501+ rows share a
    // timestamp.
    var WATERMARK_PROPERTY_NAME = 'u_mttr_last_sync';                // sys_updated_on
    var WATERMARK_SYS_ID_PROPERTY_NAME = 'u_mttr_last_sync_sys_id';  // sys_id tie-breaker

    // REST Message + HTTP Method records that talk to Choreo.
    var CHOREO_REST_MESSAGE = 'MTTR_Choreo_API';
    var CHOREO_HTTP_METHOD = 'POST_Batch';

    // sys_ids for the two `u_case_type` reference values we accept.
    // (Hard-coded because the lookup is on the hot path and these IDs
    // never change in production.)
    var INCIDENT_CASE_TYPE_SYS_ID = '8d4b87bd1b18f010cb6898aebd4bcb59';
    var QUERY_CASE_TYPE_SYS_ID = '0d5b8fbd1b18f010cb6898aebd4bcba5';

    // ─── Step 1: Read the compound watermark (sys_updated_on, sys_id) ────
    // No watermark = very first run → start from the historical cut-off
    // (early-2024, empty sys_id). The MTTR project intentionally only
    // tracks data from 2024 onwards.
    var lastSyncTimestamp = gs.getProperty(WATERMARK_PROPERTY_NAME);
    var lastSyncSysId = gs.getProperty(WATERMARK_SYS_ID_PROPERTY_NAME) || '';
    if (!lastSyncTimestamp) {
        lastSyncTimestamp = '2024-01-01 00:00:00';
        gs.info('MTTR Sync: No watermark found. Starting from ' + lastSyncTimestamp);
    }

    // ─── Step 2: Query candidate cases with a tie-broken cursor ──────────
    //   • state 3 = Closed, state 6 = Resolved / Cancelled
    //   • account / project / business_duration all required for valid MTTR
    //   • sys_created_on >= 2024-01-01 → only cases inside the project scope
    //   • u_case_type IN (Incident, Query)
    //
    // Watermark condition:
    //     (sys_updated_on > last_ts)
    //   OR
    //     (sys_updated_on = last_ts AND sys_id > last_sys_id)
    //
    // Why the OR-branch? sys_updated_on isn't unique. When >500 rows share
    // the same value they can straddle the batch boundary. A tie-broken
    // cursor advances even inside a group of ties, so the "left behind"
    // rows are picked up on the next tick instead of being skipped forever.
    // Choreo /cases/batch UPSERTs by case_sys_id, so any incidental
    // re-scan is harmless.
    //
    // Ordering must match the cursor: sys_updated_on ASC, sys_id ASC.
    var caseRecord = new GlideRecord('sn_customerservice_case');
    caseRecord.addQuery('state', 'IN', '3,6');
    caseRecord.addNotNullQuery('account');
    caseRecord.addNotNullQuery('project');
    caseRecord.addNotNullQuery('business_duration');
    caseRecord.addQuery('sys_created_on', '>=', '2024-01-01 00:00:00');
    caseRecord.addQuery('u_case_type', 'IN', INCIDENT_CASE_TYPE_SYS_ID + ',' + QUERY_CASE_TYPE_SYS_ID);
    // Compound watermark via encoded query. In SN encoded-query syntax
    // `^OR` starts an OR-alternative and `^` inside it is AND, so the
    // string below evaluates as:
    //   (sys_updated_on > last_ts) OR (sys_updated_on = last_ts AND sys_id > last_sys_id)
    caseRecord.addEncodedQuery(
        'sys_updated_on>' + lastSyncTimestamp +
        '^ORsys_updated_on=' + lastSyncTimestamp + '^sys_id>' + lastSyncSysId
    );
    caseRecord.orderBy('sys_updated_on');
    caseRecord.orderBy('sys_id');
    caseRecord.setLimit(MAX_CASES_PER_BATCH);
    caseRecord.query();

    // Nothing new — silent return.
    if (!caseRecord.hasNext()) {
        gs.debug('MTTR Sync: No new cases to push since (' + lastSyncTimestamp + ', ' + lastSyncSysId + ')');
        return;
    }

    // Accumulator for the batch payload, and the compound-cursor tracker.
    // Because the query is ORDER BY (sys_updated_on, sys_id) ASC, the last
    // row we iterate over IS the highest cursor value we've seen — no need
    // for a "max" reducer.
    var batchPayloadCases = [];
    var lastRowSysUpdatedOn = lastSyncTimestamp;
    var lastRowSysId = lastSyncSysId;

    // ─── Skip-reason counters (for end-of-run diagnostics) ───────────────
    var skipReasonStats = {
        wrongCaseType: 0,
        invalidDuration: 0,
        noProduct: 0,
        noTeam: 0
    };

    while (caseRecord.next()) {
        // ── Step 3a: Always advance the per-record cursor position ───────
        // Even rows we skip count towards "we've seen this", otherwise we'd
        // re-query the same invalid cases on every tick.
        lastRowSysUpdatedOn = caseRecord.getValue('sys_updated_on');
        lastRowSysId = caseRecord.sys_id.toString();

        // ── Step 3b: Derive is_patched from the fix-ETA field ────────────
        // u_fix_eta_shared is populated only after a fix has been targeted
        // for a release, so its mere presence is a reliable "patched" signal.
        var fixEtaValue = caseRecord.getValue('u_fix_eta_shared');
        var caseIsPatched = !!(fixEtaValue && fixEtaValue !== '');

        // ── Step 3c: Read & validate the case type ───────────────────────
        // The reference query above already restricts to Incident/Query
        // sys_ids, but we double-check via display value in case the
        // reference table changes.
        var caseTypeDisplay = caseRecord.getDisplayValue('u_case_type') || '';
        if (caseTypeDisplay !== 'Incident' && caseTypeDisplay !== 'Query') {
            skipReasonStats.wrongCaseType++;
            gs.info('MTTR Sync: Skipping case ' + caseRecord.number + ' (' + caseRecord.sys_id + ') - Wrong Case Type: "' + caseTypeDisplay + '"');
            continue;
        }

        // ── Step 3d: Normalise priority ──────────────────────────────────
        // Queries have no meaningful priority — leave it as an empty string.
        // Incidents: extract the first "P1"–"P4" substring; fall back to
        // the raw display value if no match (avoids losing odd data).
        var normalizedPriority = '';
        if (caseTypeDisplay !== 'Query') {
            var rawPriorityDisplay = caseRecord.getDisplayValue('priority') || '';
            var priorityRegexMatch = rawPriorityDisplay.match(/P[1-4]/);
            normalizedPriority = priorityRegexMatch ? priorityRegexMatch[0] : rawPriorityDisplay;
        }

        // ── Step 3e: Derive cs_team ──────────────────────────────────────
        // Team assignment priority order:
        //   1. Product-name overrides — Identity/Asgardeo → IAM, Choreo → Choreo.
        //      These always win because an Identity case raised against an
        //      account whose default team is "Cloud" still belongs in IAM's
        //      metrics, not Cloud's.
        //   2. Account.u_integration_cs_team (the account-level default).
        //   3. No team → skip the case entirely.
        var productNameDisplay = caseRecord.getDisplayValue('u_wso2_product') || '';
        if (!productNameDisplay) {
            skipReasonStats.noProduct++;
            gs.info('MTTR Sync: Skipping case ' + caseRecord.number + ' (' + caseRecord.sys_id + ') - No Product. Account: ' + caseRecord.getDisplayValue('account'));
            continue;
        }

        var productNameLower = productNameDisplay.toLowerCase();
        var resolvedCsTeam;
        if (productNameLower.indexOf('identity') >= 0 || productNameLower.indexOf('asgardeo') >= 0) {
            resolvedCsTeam = 'IAM';
        } else if (productNameLower.indexOf('choreo') >= 0) {
            resolvedCsTeam = 'Choreo';
        } else {
            resolvedCsTeam = caseRecord.getDisplayValue('account.u_integration_cs_team') || '';
        }

        if (!resolvedCsTeam) {
            skipReasonStats.noTeam++;
            gs.info('MTTR Sync: Skipping case ' + caseRecord.number + ' (' + caseRecord.sys_id + ') - No CS Team. Product: ' + productNameDisplay + ', Account: ' + caseRecord.getDisplayValue('account'));
            continue;
        }

        // ── Step 3f: Read business_duration as milliseconds ──────────────
        // business_duration is a GlideDuration; dateNumericValue() returns
        // its value in ms. This is "true work time" (excludes holidays,
        // hold periods, and outside-business-hours stretches) — the right
        // basis for MTTR.
        var businessDurationMs = 0;
        if (caseRecord.business_duration && caseRecord.business_duration.dateNumericValue) {
            businessDurationMs = parseInt(caseRecord.business_duration.dateNumericValue(), 10);
        }

        if (!businessDurationMs || businessDurationMs <= 0) {
            skipReasonStats.invalidDuration++;
            gs.info('MTTR Sync: Skipping case ' + caseRecord.number + ' (' + caseRecord.sys_id + ') - Invalid Duration: ' + businessDurationMs + ' ms');
            continue;
        }

        // ── Step 3g: Append the validated record to the batch ────────────
        //
        // closed_date MUST be the actual close/resolve time, NOT
        // sys_updated_on — every downstream MTTR aggregation buckets by
        // closed_date. If we used sys_updated_on, a case closed in Q1
        // that later gets a tag added or a work note logged in Q3 would
        // silently reappear in Q3's bucket and disappear from Q1's,
        // corrupting every rolling-window and quarterly figure.
        //
        // state 3 (Closed) populates closed_at; state 6 (Resolved/
        // "Solution Proposed") populates resolved_at. Coalesce in
        // preference order — both are stable once set, and the Choreo
        // ingestion side normalises '' → null (nullable column).
        var closedDate = caseRecord.getValue('closed_at')
            || caseRecord.getValue('resolved_at')
            || '';
        batchPayloadCases.push({
            case_sys_id: caseRecord.sys_id.toString(),
            product: productNameDisplay,                                       // reference → display value
            cs_team: resolvedCsTeam,                                           // resolved via override or account
            business_duration_ms: businessDurationMs,
            created_date: caseRecord.getValue('sys_created_on') || '',
            closed_date: closedDate,
            case_type: caseTypeDisplay,                                        // Incident | Query
            priority: normalizedPriority,                                      // P1–P4 or '' for Query
            is_patched: caseIsPatched,
            case_state: caseRecord.getDisplayValue('state') || ''              // "Closed" / "Solution Proposed" / etc.
        });
    }

    // ─── If every queried record was skipped: still advance the watermark
    // (skipped rows are structurally invalid — no point re-scanning them).
    if (batchPayloadCases.length === 0) {
        var totalSkippedNow = skipReasonStats.wrongCaseType + skipReasonStats.invalidDuration + skipReasonStats.noProduct + skipReasonStats.noTeam;
        gs.info('MTTR Sync: No eligible cases in this batch. Queried: ' + caseRecord.getRowCount() + ', Skipped: ' + totalSkippedNow);
        gs.info('  ├─ Wrong Case Type: ' + skipReasonStats.wrongCaseType);
        gs.info('  ├─ Invalid Duration: ' + skipReasonStats.invalidDuration);
        gs.info('  ├─ No Product: ' + skipReasonStats.noProduct);
        gs.info('  └─ No Team: ' + skipReasonStats.noTeam);
        gs.info('Advancing watermark to (' + lastRowSysUpdatedOn + ', ' + lastRowSysId + ')');
        gs.setProperty(WATERMARK_PROPERTY_NAME, lastRowSysUpdatedOn);
        gs.setProperty(WATERMARK_SYS_ID_PROPERTY_NAME, lastRowSysId);
        return;
    }

    // ─── Diagnostic batch summary ────────────────────────────────────────
    var totalRecordsProcessed = batchPayloadCases.length + skipReasonStats.wrongCaseType + skipReasonStats.invalidDuration + skipReasonStats.noProduct + skipReasonStats.noTeam;
    gs.info('MTTR Sync Batch Summary: Processed ' + totalRecordsProcessed + ' cases, Valid: ' + batchPayloadCases.length + ', Skipped: ' + (totalRecordsProcessed - batchPayloadCases.length));
    if (skipReasonStats.wrongCaseType + skipReasonStats.invalidDuration + skipReasonStats.noProduct + skipReasonStats.noTeam > 0) {
        gs.info('  ├─ Wrong Case Type: ' + skipReasonStats.wrongCaseType);
        gs.info('  ├─ Invalid Duration: ' + skipReasonStats.invalidDuration);
        gs.info('  ├─ No Product: ' + skipReasonStats.noProduct);
        gs.info('  └─ No Team: ' + skipReasonStats.noTeam);
    }

    // ─── Step 4: POST the batch to Choreo ────────────────────────────────
    // batch_id is unique per run; Choreo echoes it back in the ingestion_log.
    var batchId = 'batch_' + new GlideDateTime().getNumericValue();
    var requestBodyJson = JSON.stringify({
        batch_id: batchId,
        cases: batchPayloadCases
    });

    try {
        var restMessage = new sn_ws.RESTMessageV2(CHOREO_REST_MESSAGE, CHOREO_HTTP_METHOD);
        restMessage.setRequestBody(requestBodyJson);
        restMessage.setRequestHeader('Content-Type', 'application/json');
        // batch_id doubles as the correlation ID — Choreo echoes it in
        // every log line touched by this batch, so tracing SN <-> Choreo
        // failures back to a single ingestion tick becomes a grep.
        restMessage.setRequestHeader('X-Correlation-Id', batchId);
        restMessage.setHttpTimeout(60000); // 60s — Choreo can take this long during cold-starts.

        var apiResponse = restMessage.execute();
        var httpStatusCode = apiResponse.getStatusCode();
        var responseBodyText = apiResponse.getBody();

        // ─── Step 5: Watermark advance is gated on HTTP 200 ──────────────
        // This is what makes the delivery semantics "at least once": a
        // Choreo outage simply means the same batch ships next run.
        if (httpStatusCode == 200) {
            gs.setProperty(WATERMARK_PROPERTY_NAME, lastRowSysUpdatedOn);
            gs.setProperty(WATERMARK_SYS_ID_PROPERTY_NAME, lastRowSysId);
            gs.info('MTTR Sync: Pushed ' + batchPayloadCases.length + ' cases (batch: ' + batchId + '). New watermark: (' + lastRowSysUpdatedOn + ', ' + lastRowSysId + ')');

            // Choreo reports per-record rejections in the body — surface
            // them at WARN level so operators see them in the syslog.
            try {
                var parsedResponse = JSON.parse(responseBodyText);
                if (parsedResponse.rejected > 0) {
                    gs.warn('MTTR Sync: ' + parsedResponse.rejected + ' records rejected by Choreo. Details: ' + JSON.stringify(parsedResponse.rejected_details));
                }
            } catch (responseParseError) {
                // Push succeeded but the body wasn't JSON — non-fatal.
            }
        } else {
            // Any non-200 leaves the watermark untouched so we retry next run.
            gs.error('MTTR Sync: Push failed with HTTP ' + httpStatusCode + '. Body: ' + responseBodyText);
        }
    } catch (restCallException) {
        // Network / TLS / OAuth failure → same "don't advance" policy.
        gs.error('MTTR Sync: Exception during push — ' + restCallException.getMessage());
    }
})();

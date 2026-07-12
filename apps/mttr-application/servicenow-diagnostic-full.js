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

// ════════════════════════════════════════════════════════════════════════════
// MTTR Sync Diagnostic Script — Full Analysis
// ----------------------------------------------------------------------------
// Run inside ServiceNow: Scripts → Background.
//
// PURPOSE
//   When the scheduled job is shipping fewer cases than expected, this
//   script tells you WHY by walking every record in the same query the
//   scheduled job uses and counting each skip reason.
//
//   It also captures up to 3 example records per skip reason so you can
//   click straight into a failing case and inspect the underlying data.
//
// HOW IT MIRRORS THE SCHEDULED JOB
//   The GlideRecord query and skip checks below are an EXACT mirror of
//   scheduled-job-mttr-push.js. Keep them in sync when you change either
//   file — otherwise diagnostics will diverge from real-world behaviour.
//
//   Lookback window: 2024-01-01 onwards, matching the scheduled job's
//   project scope. The MTTR project intentionally only tracks data from
//   2024 onwards.
// ════════════════════════════════════════════════════════════════════════════

// sys_id references for the `u_case_type` reference field. Hard-coded to
// keep this script standalone — same values as the scheduled job.
var INCIDENT_CASE_TYPE_SYS_ID = '8d4b87bd1b18f010cb6898aebd4bcb59';
var QUERY_CASE_TYPE_SYS_ID = '0d5b8fbd1b18f010cb6898aebd4bcba5';

// ─── Build the same query the scheduled job uses ──────────────────────────
//
// state IN (3,6)              → Closed + Resolved/Cancelled
// account/project not null    → minimum data integrity for MTTR
// business_duration not null  → needed to compute MTTR
// sys_updated_on >= 2024-...  → matches the scheduled job's initial
//                                watermark (project starts 2024-01-01)
// sys_created_on >= 2024-...  → same project scope
// u_case_type IN (Incident,Q) → only the two types MTTR tracks

var caseRecord = new GlideRecord('sn_customerservice_case');
caseRecord.addQuery('state', 'IN', '3,6');
caseRecord.addQuery('sys_updated_on', '>=', '2024-01-01 00:00:00');
caseRecord.addNotNullQuery('account');
caseRecord.addNotNullQuery('project');
caseRecord.addNotNullQuery('business_duration');
caseRecord.addQuery('sys_created_on', '>=', '2024-01-01 00:00:00');
caseRecord.addQuery('u_case_type', 'IN', INCIDENT_CASE_TYPE_SYS_ID + ',' + QUERY_CASE_TYPE_SYS_ID);
caseRecord.orderBy('sys_updated_on');
caseRecord.setLimit(500);
caseRecord.query();

// ─── Counters and example collectors ──────────────────────────────────────
//
// We track per-reason counts AND keep up to 3 examples of each. The
// examples are what makes this script useful operationally — they give
// the operator something to click into and inspect.

var skipStatistics = {
    total: 0,
    valid: 0,
    skipped: {
        invalidDuration: 0,
        wrongCaseType: 0,
        noProduct: 0,
        noTeamAfterOverride: 0
    }
};

var skipExamples = {
    invalidDuration: [],
    wrongCaseType: [],
    noProduct: [],
    noTeamAfterOverride: []
};

// ─── Walk every record and apply the same checks as the scheduled job ─────

while (caseRecord.next()) {
    skipStatistics.total++;

    // ── Check 1: Case type must be Incident or Query ────────────────────
    var caseTypeDisplay = caseRecord.getDisplayValue('u_case_type') || '';
    if (caseTypeDisplay !== 'Incident' && caseTypeDisplay !== 'Query') {
        skipStatistics.skipped.wrongCaseType++;
        if (skipExamples.wrongCaseType.length < 3) {
            skipExamples.wrongCaseType.push({
                number: caseRecord.number + '',
                sys_id: caseRecord.sys_id + '',
                case_type: caseTypeDisplay,
                product: caseRecord.getDisplayValue('u_wso2_product')
            });
        }
        continue;
    }

    // ── Check 2: business_duration must be present and positive ─────────
    // dateNumericValue() returns milliseconds — 0/negative are useless for MTTR.
    var businessDurationMs = 0;
    if (caseRecord.business_duration && caseRecord.business_duration.dateNumericValue) {
        businessDurationMs = parseInt(caseRecord.business_duration.dateNumericValue(), 10);
    }

    if (!businessDurationMs || businessDurationMs <= 0) {
        skipStatistics.skipped.invalidDuration++;
        if (skipExamples.invalidDuration.length < 3) {
            skipExamples.invalidDuration.push({
                number: caseRecord.number + '',
                sys_id: caseRecord.sys_id + '',
                duration_raw: caseRecord.business_duration + '',
                product: caseRecord.getDisplayValue('u_wso2_product')
            });
        }
        continue;
    }

    // ── Check 3: Product must be populated ──────────────────────────────
    // Without a product we can't apply the IAM/Choreo overrides AND we
    // can't fall back to the account's default team.
    var productNameDisplay = caseRecord.getDisplayValue('u_wso2_product') || '';
    if (!productNameDisplay) {
        skipStatistics.skipped.noProduct++;
        if (skipExamples.noProduct.length < 3) {
            skipExamples.noProduct.push({
                number: caseRecord.number + '',
                sys_id: caseRecord.sys_id + '',
                account: caseRecord.getDisplayValue('account'),
                account_cs_team: caseRecord.getDisplayValue('account.u_integration_cs_team')
            });
        }
        continue;
    }

    // ── Check 4: Resolve cs_team via override-then-account chain ────────
    // Identity/Asgardeo products → IAM team (always).
    // Choreo product            → Choreo team (always).
    // Anything else             → account.u_integration_cs_team.
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
        skipStatistics.skipped.noTeamAfterOverride++;
        if (skipExamples.noTeamAfterOverride.length < 3) {
            skipExamples.noTeamAfterOverride.push({
                number: caseRecord.number + '',
                sys_id: caseRecord.sys_id + '',
                product: productNameDisplay,
                account: caseRecord.getDisplayValue('account'),
                account_cs_team: caseRecord.getDisplayValue('account.u_integration_cs_team')
            });
        }
        continue;
    }

    // ── Passed every check: this case WOULD be synced by the job ────────
    skipStatistics.valid++;
}

// ════════════════════════════════════════════════════════════════════════════
// Pretty-print the results to the system log (gs.info shows in syslog)
// ════════════════════════════════════════════════════════════════════════════

gs.info('═══════════════════════════════════════════════════════');
gs.info('MTTR SYNC DIAGNOSTIC - FULL ANALYSIS');
gs.info('═══════════════════════════════════════════════════════');
gs.info('');
gs.info('Query: state IN (3,6) AND sys_updated_on >= 2024-01-01 AND sys_created_on >= 2024-01-01');
gs.info('       + account NOT NULL + project NOT NULL + business_duration NOT NULL');
gs.info('       + u_case_type IN (Incident, Query)');
gs.info('');
gs.info('SUMMARY:');
gs.info('  Total cases matched:        ' + skipStatistics.total);
gs.info('  Valid (would sync):         ' + skipStatistics.valid);
gs.info('  Total skipped:              ' + (skipStatistics.total - skipStatistics.valid));
gs.info('');
gs.info('SKIP REASONS:');
gs.info('  ├─ Wrong Case Type:         ' + skipStatistics.skipped.wrongCaseType);
gs.info('  ├─ Invalid Duration:        ' + skipStatistics.skipped.invalidDuration);
gs.info('  ├─ No Product:              ' + skipStatistics.skipped.noProduct);
gs.info('  └─ No Team (after override): ' + skipStatistics.skipped.noTeamAfterOverride);
gs.info('');

// ─── Per-reason example dumps (helps operators jump straight to a case) ───

if (skipStatistics.skipped.wrongCaseType > 0) {
    gs.info('───────────────────────────────────────────────────────');
    gs.info('SKIPPED - WRONG CASE TYPE (' + skipStatistics.skipped.wrongCaseType + ' total):');
    for (var i = 0; i < skipExamples.wrongCaseType.length; i++) {
        gs.info('  • ' + skipExamples.wrongCaseType[i].number + ' | Type: "' + skipExamples.wrongCaseType[i].case_type + '" | Product: ' + skipExamples.wrongCaseType[i].product);
    }
}

if (skipStatistics.skipped.invalidDuration > 0) {
    gs.info('───────────────────────────────────────────────────────');
    gs.info('SKIPPED - INVALID DURATION (' + skipStatistics.skipped.invalidDuration + ' total):');
    for (var i = 0; i < skipExamples.invalidDuration.length; i++) {
        gs.info('  • ' + skipExamples.invalidDuration[i].number + ' | Duration: ' + skipExamples.invalidDuration[i].duration_raw + ' | Product: ' + skipExamples.invalidDuration[i].product);
    }
}

if (skipStatistics.skipped.noProduct > 0) {
    gs.info('───────────────────────────────────────────────────────');
    gs.info('SKIPPED - NO PRODUCT (' + skipStatistics.skipped.noProduct + ' total):');
    for (var i = 0; i < skipExamples.noProduct.length; i++) {
        gs.info('  • ' + skipExamples.noProduct[i].number + ' | Account: ' + skipExamples.noProduct[i].account + ' | Account CS Team: ' + skipExamples.noProduct[i].account_cs_team);
    }
}

if (skipStatistics.skipped.noTeamAfterOverride > 0) {
    gs.info('───────────────────────────────────────────────────────');
    gs.info('SKIPPED - NO TEAM AFTER OVERRIDE (' + skipStatistics.skipped.noTeamAfterOverride + ' total):');
    gs.info('(Product does NOT contain identity/asgardeo/choreo AND Account CS Team is empty)');
    gs.info('');
    for (var i = 0; i < skipExamples.noTeamAfterOverride.length; i++) {
        gs.info('  • ' + skipExamples.noTeamAfterOverride[i].number);
        gs.info('    Product: ' + skipExamples.noTeamAfterOverride[i].product);
        gs.info('    Account: ' + skipExamples.noTeamAfterOverride[i].account);
        gs.info('    Account CS Team: ' + skipExamples.noTeamAfterOverride[i].account_cs_team);
    }
}

gs.info('═══════════════════════════════════════════════════════');
gs.info('');
gs.info('EXPECTED SYNC RESULT:');
gs.info('  • Valid cases should be inserted into backend');
gs.info('  • "No Product" cases will be rejected with reason "product is missing"');
gs.info('  • All other skipped cases will not be sent to backend');
gs.info('');
gs.info('═══════════════════════════════════════════════════════');

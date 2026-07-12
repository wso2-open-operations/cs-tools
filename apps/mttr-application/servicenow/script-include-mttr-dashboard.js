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
// ServiceNow Script Include: MttrDashboardAPI
// ============================================================================
//
// PURPOSE
//   Server-side adapter between the Choreo MTTR API and the ServiceNow
//   homepage widgets that visualise the data with Google Charts.
//
//   Each widget's <g:evaluate> block instantiates this class and calls one
//   of the get…() methods below; the method returns plain JS objects ready
//   to be JSON-stringified into the Jelly template so the client side can
//   consume them.
//
// WHY A SCRIPT INCLUDE?
//   • Centralises REST authentication + retry semantics.
//   • Reshapes API responses into the row/column structures Google Charts
//     expects — keeps the Jelly template free of data-massaging code.
//   • Allows reuse across widgets without duplicating fetch logic.
//
// PREREQUISITES
//   REST Message `MTTR_Choreo_API` with two HTTP methods:
//     • GET_MTTR    – .../mttr?type=${type}
//     • GET_Summary – .../summary/historical?case_type=${case_type}&group_by=${group_by}
//   Both methods use the same OAuth 2.0 client-credentials profile.
//
// USAGE INSIDE A <g:evaluate> BLOCK:
//   var api = new MttrDashboardAPI();
//   JSON.stringify(api.getOverallByType());
// ============================================================================

var MttrDashboardAPI = Class.create();
MttrDashboardAPI.prototype = {
    initialize: function () {
        // REST Message + HTTP Method names — keep in sync with the SN
        // configuration described in servicenow/rest-message-setup.js.
        this.REST_MESSAGE_NAME = 'MTTR_Choreo_API';
        this.HTTP_METHOD_MTTR = 'GET_MTTR';
        this.HTTP_METHOD_SUMMARY = 'GET_Summary';
    },

    // ─── Core: fetch one dimension from /mttr ──────────────────────────────
    //
    // Returns the parsed JSON envelope from the API, or null on any
    // network / HTTP failure. Failures are logged but never throw —
    // the caller treats null as "no data" so one broken widget never
    // blows up the whole dashboard render.
    _fetchDimension: function (dimensionTypeName) {
        try {
            gs.info('MttrDashboardAPI: Fetching dimension=' + dimensionTypeName);
            var restMessage = new sn_ws.RESTMessageV2(this.REST_MESSAGE_NAME, this.HTTP_METHOD_MTTR);
            // setStringParameter (URL-encodes) rather than setStringParameterNoEscape:
            // every current caller passes a hardcoded dimension name, so the value
            // is safe either way — but URL-encoding hardens the call site against
            // a future refactor accidentally routing user input here.
            restMessage.setStringParameter('type', dimensionTypeName);
            restMessage.setHttpTimeout(30000);

            var apiResponse = restMessage.execute();
            var httpStatusCode = apiResponse.getStatusCode();
            var responseBodyText = apiResponse.getBody();

            gs.info('MttrDashboardAPI: HTTP ' + httpStatusCode + ' for ' + dimensionTypeName + ' — body length=' + (responseBodyText ? responseBodyText.length : 0));

            if (httpStatusCode == 200) {
                var parsedEnvelope = JSON.parse(responseBodyText);
                gs.info('MttrDashboardAPI: ' + dimensionTypeName + ' returned ' + (parsedEnvelope.data ? parsedEnvelope.data.length : 0) + ' rows');
                return parsedEnvelope;
            }
            gs.error('MttrDashboardAPI: HTTP ' + httpStatusCode + ' for type ' + dimensionTypeName + ' — body: ' + (responseBodyText || '').substring(0, 500));
            return null;
        } catch (restException) {
            gs.error('MttrDashboardAPI: Exception fetching ' + dimensionTypeName + ' — ' + restException.getMessage());
            return null;
        }
    },

    // ─── Helper: hours → "X Days Y Hours Z Minutes" display string ────────
    //
    // Used for KPI cards and tooltips. Rounded to whole minutes so the
    // display stays compact even for very long-running cases.
    _hoursToDisplay: function (hoursValue) {
        if (hoursValue === null || hoursValue === undefined) return '0 Days 0 Hours 0 Minutes';
        var totalMinutes = Math.round(hoursValue * 60);
        var displayDays = Math.floor(totalMinutes / (60 * 24));
        var displayHours = Math.floor((totalMinutes % (60 * 24)) / 60);
        var displayMinutes = totalMinutes % 60;
        return displayDays + ' Days ' + displayHours + ' Hours ' + displayMinutes + ' Minutes';
    },

    // ─── Widget 1-3: Overall MTTR (combined + Incident + Query) ───────────
    //
    // Pulls `overall_by_type`, then computes:
    //   • result.incident / .query   – per-case-type MTTR (display string)
    //   • result.incident_hours      – raw numeric value (for sorting/maths)
    //   • result.overall             – weighted average across both
    //                                  (weight = total_cases — preserves the
    //                                  correct overall MTTR even when one
    //                                  case type dominates the volume).
    getOverallByType: function () {
        var apiEnvelope = this._fetchDimension('overall_by_type');
        if (!apiEnvelope || !apiEnvelope.data) return null;

        var result = { overall: null, incident: null, query: null };
        var weightedHoursSum = 0;
        var weightedCaseCount = 0;

        for (var rowIndex = 0; rowIndex < apiEnvelope.data.length; rowIndex++) {
            var dataRow = apiEnvelope.data[rowIndex];
            var caseTypeKey = (dataRow.labels.case_type || '').toLowerCase();
            var displayString = this._hoursToDisplay(dataRow.mttr_hours);

            if (caseTypeKey === 'incident') {
                result.incident = displayString;
                result.incident_hours = dataRow.mttr_hours;
            } else if (caseTypeKey === 'query') {
                result.query = displayString;
                result.query_hours = dataRow.mttr_hours;
            }

            // Weighted average for the combined MTTR figure.
            // Include the row when we have a real numeric mttr_hours (0 is
            // legitimate — a small-sample or genuinely-fast case type) and
            // at least one case backing it. The prior `dataRow.mttr_hours &&
            // dataRow.total_cases` truthy check silently dropped 0-hour rows.
            if (dataRow.total_cases > 0 && dataRow.mttr_hours != null) {
                weightedHoursSum += dataRow.mttr_hours * dataRow.total_cases;
                weightedCaseCount += dataRow.total_cases;
            }
        }

        if (weightedCaseCount > 0) {
            result.overall = this._hoursToDisplay(weightedHoursSum / weightedCaseCount);
        }

        return result;
    },

    // ─── Widget 4-5: Patched / Non-Patched MTTR by case type ──────────────
    //
    // Fetches two dimensions (`patched_incidents` + `patched_queries`) and
    // restructures them into:
    //   {
    //     patched:     { Incident: "Xd Yh Zm", Query: "Xd Yh Zm", incident_hours, query_hours },
    //     non_patched: { Incident: "…",        Query: "…",        incident_hours, query_hours }
    //   }
    // This shape lines up with the two pie widgets ("Patched" and "Non-Patched").
    getPatchedMTTR: function () {
        var patchedIncidentsResponse = this._fetchDimension('patched_incidents');
        var patchedQueriesResponse = this._fetchDimension('patched_queries');

        var result = {
            patched:     { Incident: null, Query: null, incident_hours: 0, query_hours: 0 },
            non_patched: { Incident: null, Query: null, incident_hours: 0, query_hours: 0 }
        };

        if (patchedIncidentsResponse && patchedIncidentsResponse.data) {
            for (var incRowIndex = 0; incRowIndex < patchedIncidentsResponse.data.length; incRowIndex++) {
                var incidentRow = patchedIncidentsResponse.data[incRowIndex];
                // is_patched arrives as boolean or string — normalise both.
                var incidentIsPatched = (incidentRow.labels.is_patched === true || incidentRow.labels.is_patched === 'true');
                var incidentBucket = incidentIsPatched ? 'patched' : 'non_patched';
                result[incidentBucket].Incident = this._hoursToDisplay(incidentRow.mttr_hours);
                result[incidentBucket].incident_hours = incidentRow.mttr_hours || 0;
            }
        }

        if (patchedQueriesResponse && patchedQueriesResponse.data) {
            for (var qryRowIndex = 0; qryRowIndex < patchedQueriesResponse.data.length; qryRowIndex++) {
                var queryRow = patchedQueriesResponse.data[qryRowIndex];
                var queryIsPatched = (queryRow.labels.is_patched === true || queryRow.labels.is_patched === 'true');
                var queryBucket = queryIsPatched ? 'patched' : 'non_patched';
                result[queryBucket].Query = this._hoursToDisplay(queryRow.mttr_hours);
                result[queryBucket].query_hours = queryRow.mttr_hours || 0;
            }
        }

        return result;
    },

    // ─── Widget 6: Monthly priority trend (Incidents) ─────────────────────
    //
    // Returns rows sorted chronologically by month:
    //   [
    //     { month: "2025-04", P1: hours, P1_display: "…", P2: …, P2_display: "…", … },
    //     { month: "2025-05", … },
    //   ]
    // The *_display fields are pre-formatted strings for tooltips.
    getMonthlyTrendPriority: function () {
        var apiEnvelope = this._fetchDimension('monthly_trend_priority');
        if (!apiEnvelope || !apiEnvelope.data) return [];

        // Build month → priority → hours map first; pivot afterwards.
        var hoursByMonthAndPriority = {};
        for (var rowIndex = 0; rowIndex < apiEnvelope.data.length; rowIndex++) {
            var dataRow = apiEnvelope.data[rowIndex];
            var monthLabel = dataRow.labels.month || '';
            var priorityLabel = dataRow.labels.priority || '';

            if (!hoursByMonthAndPriority[monthLabel]) hoursByMonthAndPriority[monthLabel] = {};
            hoursByMonthAndPriority[monthLabel][priorityLabel] = dataRow.mttr_hours || 0;
        }

        var sortedMonthLabels = Object.keys(hoursByMonthAndPriority).sort();
        var rowOutput = [];
        for (var monthIndex = 0; monthIndex < sortedMonthLabels.length; monthIndex++) {
            var monthEntry = { month: sortedMonthLabels[monthIndex] };
            var hoursByPriority = hoursByMonthAndPriority[sortedMonthLabels[monthIndex]];
            for (var priorityKey in hoursByPriority) {
                monthEntry[priorityKey] = hoursByPriority[priorityKey];
                monthEntry[priorityKey + '_display'] = this._hoursToDisplay(hoursByPriority[priorityKey]);
            }
            rowOutput.push(monthEntry);
        }
        return rowOutput;
    },

    // ─── Widget 7: Team × Case Type bar chart ─────────────────────────────
    //
    // Merges team_incidents and team_queries into one row per team:
    //   [ { team: "Polaris", Incident: hrs, Query: hrs, Incident_display, Query_display }, … ]
    // Sorted by Incident MTTR descending so the chart highlights the
    // slowest teams first.
    getTeamCaseTypeMTTR: function () {
        var teamIncidentsResponse = this._fetchDimension('team_incidents');
        var teamQueriesResponse = this._fetchDimension('team_queries');

        var rowsByTeam = {};

        if (teamIncidentsResponse && teamIncidentsResponse.data) {
            for (var incRowIndex = 0; incRowIndex < teamIncidentsResponse.data.length; incRowIndex++) {
                var incidentRow = teamIncidentsResponse.data[incRowIndex];
                var teamName = incidentRow.labels.cs_team || '';
                if (!rowsByTeam[teamName]) rowsByTeam[teamName] = { team: teamName, Incident: 0, Query: 0, Incident_display: '', Query_display: '' };
                rowsByTeam[teamName].Incident = incidentRow.mttr_hours || 0;
                rowsByTeam[teamName].Incident_display = this._hoursToDisplay(incidentRow.mttr_hours);
            }
        }

        if (teamQueriesResponse && teamQueriesResponse.data) {
            for (var qryRowIndex = 0; qryRowIndex < teamQueriesResponse.data.length; qryRowIndex++) {
                var queryRow = teamQueriesResponse.data[qryRowIndex];
                var queryTeamName = queryRow.labels.cs_team || '';
                if (!rowsByTeam[queryTeamName]) rowsByTeam[queryTeamName] = { team: queryTeamName, Incident: 0, Query: 0, Incident_display: '', Query_display: '' };
                rowsByTeam[queryTeamName].Query = queryRow.mttr_hours || 0;
                rowsByTeam[queryTeamName].Query_display = this._hoursToDisplay(queryRow.mttr_hours);
            }
        }

        // Sort by Incident MTTR descending — slowest team first.
        var teamNamesSortedByIncident = Object.keys(rowsByTeam);
        teamNamesSortedByIncident.sort(function (a, b) {
            return (rowsByTeam[b].Incident || 0) - (rowsByTeam[a].Incident || 0);
        });

        var output = [];
        for (var sortedIndex = 0; sortedIndex < teamNamesSortedByIncident.length; sortedIndex++) {
            output.push(rowsByTeam[teamNamesSortedByIncident[sortedIndex]]);
        }
        return output;
    },

    // ─── Widget 8: Monthly per-team line chart ────────────────────────────
    //
    // Returns:
    //   {
    //     months: ["2025-04","2025-05",…],
    //     teams:  { "Polaris":[h,h,…], "IAM":[h,h,…] },
    //     teams_display: { "Polaris":["Xd Yh Zm",…], … }
    //   }
    // Pivot is done here (server side) so the client just feeds it into
    // Google Charts column-by-column.
    getMonthlyTrendTeam: function () {
        var apiEnvelope = this._fetchDimension('monthly_trend_team');
        if (!apiEnvelope || !apiEnvelope.data) return { months: [], teams: {} };

        var monthsSeen = {};
        var hoursByTeamThenMonth = {};

        for (var rowIndex = 0; rowIndex < apiEnvelope.data.length; rowIndex++) {
            var dataRow = apiEnvelope.data[rowIndex];
            var monthLabel = dataRow.labels.month || '';
            var teamName = dataRow.labels.cs_team || '';

            monthsSeen[monthLabel] = true;
            if (!hoursByTeamThenMonth[teamName]) hoursByTeamThenMonth[teamName] = {};
            hoursByTeamThenMonth[teamName][monthLabel] = dataRow.mttr_hours || 0;
        }

        var sortedMonths = Object.keys(monthsSeen).sort();
        var teamSeriesNumeric = {};
        var teamSeriesDisplay = {};
        for (var teamKey in hoursByTeamThenMonth) {
            teamSeriesNumeric[teamKey] = [];
            teamSeriesDisplay[teamKey] = [];
            for (var monthIndex = 0; monthIndex < sortedMonths.length; monthIndex++) {
                var mttrHoursValue = hoursByTeamThenMonth[teamKey][sortedMonths[monthIndex]] || 0;
                teamSeriesNumeric[teamKey].push(mttrHoursValue);
                teamSeriesDisplay[teamKey].push(this._hoursToDisplay(mttrHoursValue));
            }
        }

        return { months: sortedMonths, teams: teamSeriesNumeric, teams_display: teamSeriesDisplay };
    },

    // ─── Historical (quarterly) summaries via /summary/historical ─────────
    //
    // Used by the four "Historical …" widgets. Returns the API envelope
    // verbatim — the Jelly templates consume the {periods, series} shape
    // directly.
    _fetchSummary: function (caseTypeName, groupByDimension) {
        try {
            gs.info('MttrDashboardAPI: Fetching historical summary case_type=' + caseTypeName + ' group_by=' + groupByDimension);
            var restMessage = new sn_ws.RESTMessageV2(this.REST_MESSAGE_NAME, this.HTTP_METHOD_SUMMARY);
            // URL-encoded per _fetchDimension comment above — defensive against
            // future callers that might pass unvalidated user input.
            restMessage.setStringParameter('case_type', caseTypeName);
            restMessage.setStringParameter('group_by', groupByDimension);
            restMessage.setHttpTimeout(30000);

            var apiResponse = restMessage.execute();
            var httpStatusCode = apiResponse.getStatusCode();
            var responseBodyText = apiResponse.getBody();

            gs.info('MttrDashboardAPI: HTTP ' + httpStatusCode + ' for historical summary — body length=' + (responseBodyText ? responseBodyText.length : 0));

            if (httpStatusCode == 200) {
                return JSON.parse(responseBodyText);
            }
            gs.error('MttrDashboardAPI: HTTP ' + httpStatusCode + ' for historical summary — body: ' + (responseBodyText || '').substring(0, 500));
            return null;
        } catch (restException) {
            gs.error('MttrDashboardAPI: Exception fetching historical summary — ' + restException.getMessage());
            return null;
        }
    },

    // ─── Historical: Incidents by team ────────────────────────────────────
    getHistoricalIncidentsByTeam: function () {
        return this._fetchSummary('Incident', 'team');
    },

    // ─── Historical: Incidents by priority ────────────────────────────────
    getHistoricalIncidentsByPriority: function () {
        return this._fetchSummary('Incident', 'priority');
    },

    // ─── Historical: Queries by team ──────────────────────────────────────
    getHistoricalQueriesByTeam: function () {
        return this._fetchSummary('Query', 'team');
    },

    // ─── Historical: overall (Incidents vs Queries) ──────────────────────
    //
    // The API doesn't expose a single "overall historical" endpoint, so we
    // pull the team-level data for both case types and compute a per-period
    // weighted average (weight = total_cases) here. Returns:
    //   {
    //     periods: ["2025-Q1","2025-Q2",…],
    //     series:  { "Incident": { "2025-Q1": { mttr_hours, total_cases } },
    //                "Query":    { … } }
    //   }
    getHistoricalOverall: function () {
        var incidentSummaryResponse = this._fetchSummary('Incident', 'team');
        var querySummaryResponse = this._fetchSummary('Query', 'team');

        var periodsSeen = {};
        var seriesByCaseType = { Incident: {}, Query: {} };

        // Inner helper: collapse one (per-team-per-period) envelope into
        // weighted-average per period for the given case-type bucket.
        var aggregateIntoSeries = function (summaryEnvelope, caseTypeBucket) {
            if (!summaryEnvelope || !summaryEnvelope.periods) return;
            for (var periodIndex = 0; periodIndex < summaryEnvelope.periods.length; periodIndex++) {
                var periodLabel = summaryEnvelope.periods[periodIndex];
                periodsSeen[periodLabel] = true;
                var weightedHoursSum = 0;
                var weightedCaseCount = 0;
                var teamSeries = summaryEnvelope.series || {};
                for (var teamKey in teamSeries) {
                    var periodEntry = teamSeries[teamKey][periodLabel];
                    if (periodEntry) {
                        var perTeamHours = periodEntry.mttr_hours || 0;
                        var perTeamCases = periodEntry.total_cases || 0;
                        weightedHoursSum += perTeamHours * perTeamCases;
                        weightedCaseCount += perTeamCases;
                    }
                }
                seriesByCaseType[caseTypeBucket][periodLabel] = {
                    // Round to 1 decimal place to match the /mttr endpoint's convention.
                    mttr_hours: weightedCaseCount > 0 ? Math.round((weightedHoursSum / weightedCaseCount) * 10) / 10 : 0,
                    total_cases: weightedCaseCount
                };
            }
        };

        aggregateIntoSeries(incidentSummaryResponse, 'Incident');
        aggregateIntoSeries(querySummaryResponse, 'Query');

        var sortedPeriods = Object.keys(periodsSeen).sort();
        return { periods: sortedPeriods, series: seriesByCaseType };
    },

    type: 'MttrDashboardAPI'
};

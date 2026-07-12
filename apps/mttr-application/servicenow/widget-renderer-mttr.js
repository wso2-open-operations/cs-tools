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
// ServiceNow Homepage Widget Renderer — MTTR Widgets
// ============================================================================
//
// PURPOSE
//   ServiceNow's "Homepage Widget Renderer" is the bridge between the
//   homepage layout system and the 12 standalone UI Pages that render the
//   MTTR dashboard sections.
//
//   When an admin drops a "MTTR" widget onto a homepage they pick one of
//   the entries in sections() below; that label is mapped to the matching
//   UI Page name and rendered via GlideForm.
//
// SETUP
//   Create a "Renderer" record (Homepage → Widget Renderer) and paste
//   this code in. The three exported functions (sections, render,
//   getEditLink) are what the platform expects.
//
// SECTION → UI PAGE MAPPING
//   Each entry in sections() corresponds to a UI Page record stored under
//   `sys_ui_page` with the given `name`. Create one UI Page per entry,
//   each containing a Jelly template that calls into MttrDashboardAPI
//   (see script-include-mttr-dashboard.js) and renders the chart with
//   Google Visualization API.
// ============================================================================

/**
 * Return the dropdown of widget sections an admin can pick from when
 * placing this renderer on a homepage. Map key = human label shown in
 * the picker; value.name = the UI Page (sys_ui_page) that renders it.
 */
function sections() {
    return {
        'CS Overall MTTR':                  { 'name': 'mttr_widget_overall' },
        'Overall Incident MTTR':            { 'name': 'mttr_widget_incidents' },
        'Overall Query MTTR':               { 'name': 'mttr_widget_queries' },
        'Non Patched Issues MTTR':          { 'name': 'mttr_widget_non_patched' },
        'Patched Issues MTTR':              { 'name': 'mttr_widget_patched' },
        'Priority Level MTTR Trend':        { 'name': 'mttr_widget_priority_trend' },
        'Case Type Wise MTTR for ABTs':     { 'name': 'mttr_widget_team_casetype' },
        'ABT Monthly MTTR Trend':           { 'name': 'mttr_widget_team_trend' },
        'Historical Overall Trend':         { 'name': 'mttr_widget_hist_overall' },
        'Historical Incidents by Team':     { 'name': 'mttr_widget_hist_inc_team' },
        'Historical Incidents by Priority': { 'name': 'mttr_widget_hist_inc_priority' },
        'Historical Queries by Team':       { 'name': 'mttr_widget_hist_qry_team' }
    };
}

/**
 * Render the UI Page chosen by the admin. The platform-provided `renderer`
 * global gives us the preference (which section), and GlideForm renders
 * the corresponding UI Page in standalone mode.
 */
function render() {
    var selectedPageName = renderer.getPreferences().get("name");
    var glideForm = new GlideForm(renderer.getGC(), selectedPageName, 0);
    glideForm.setDirect(true);
    glideForm.setRenderProperties(renderer.getRenderProperties());
    return glideForm.getRenderedPage();
}

/**
 * Return a deep-link to edit the underlying UI Page record — but only for
 * admins, so non-admin homepage viewers don't see configuration links.
 */
function getEditLink() {
    if (!gs.hasRole('admin')) {
        return '';
    }
    return "sys_ui_page.do?sysparm_query=name=" + renderer.getPreferences().get("name");
}

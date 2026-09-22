/* ============================================================================
   42 — THE STATUS-UPDATE EMAIL FLOW, AND WHO IT IS ALLOWED TO REACH.
        Run in: System Definition > Scripts - Background  (/sys.scripts.do)
        READ-ONLY. No writes, no updates, no deletes.

   Scope: SendEmailsOnEngagementStatusUpdateFlow — trigger "Created" on
   u_customer_engagement_status_update, calling
   SendEmailsOnEngagmentStatusUpdateAction (to / subject / ccList / content).
   Ported to entity-service (POST /engagement-status-updates, which writes the
   update and publishes engagement.status_update_created) and
   csm-notification-service (which consumes that event and sends the mail).

   ── THE ONE THAT DECIDES THE DESIGN ─────────────────────────────────────
   PASS 2. The action's script step loops the cc list and rejects anything
   not ending "@wso2.com" — but it throws inside its own try/catch, the catch
   says "Do nothing", and nothing downstream reads its error output, so the
   email sends anyway. Two readings, and they need different ports:

     (a) the check was INTENDED and is simply broken -> the port should
         enforce @wso2.com and reject external addresses outright, or

     (b) external addresses are LEGITIMATE for customer-visible updates
         (u_visibility = 1 "Customer") and the check was quietly defanged on
         purpose -> enforcing it would break real sends.

   PASS 2 settles it with evidence: it reports how many existing updates
   carry a non-WSO2 address in u_mailing_list, and cross-tabs that against
   u_visibility and u_scope. If external addresses only ever appear on
   Customer-visibility rows, that is reading (b) and the port must gate the
   filter on visibility rather than apply it flatly.

   ── RUN IT IN PASSES ────────────────────────────────────────────────────
     PASS 1  the flow + its action: trigger condition, steps, input bindings
     PASS 2  recipient reality: delimiters, external addresses, visibility
     PASS 3  who else writes to this table (other flows, business rules)
             — the port assumes the create endpoint is the only trigger path
     PASS 4  a recent row end to end
             *** PASS 4 PRINTS REAL EMAIL ADDRESSES — SCRUB BEFORE SHARING ***
   ======================================================================== */

var PASS = 1;

var FLOW_NAME   = 'SendEmailsOnEngagementStatusUpdateFlow';
var ACTION_NAME = 'SendEmailsOnEngagmentStatusUpdateAction';  /* sic: "Engagment" */
var TABLE       = 'u_customer_engagement_status_update';

var BUDGET = 60000, used = 0, truncated = false;
function out(s) {
  s = String(s);
  if (truncated) return;
  if (used + s.length > BUDGET) { gs.info('*** BUDGET HIT — TRUNCATED ***'); truncated = true; return; }
  used += s.length; gs.info(s);
}
function rule(t) { out(''); out('================================================================'); out('  ' + t); out('================================================================'); }

/* -------------------------------------------------------------------------
   PASS 1 — the flow's own definition, including the trigger condition and
            the action input bindings that a screenshot shows only partly.
   ------------------------------------------------------------------------- */
if (PASS == 1) {
  rule('FLOW: ' + FLOW_NAME);

  var f = new GlideRecord('sys_hub_flow');
  f.addQuery('name', FLOW_NAME);
  f.query();
  if (!f.next()) {
    out('!! flow not found by exact name — check spelling in FLOW_NAME');
  } else {
    out('  sys_id      : ' + f.getUniqueValue());
    out('  active      : ' + f.getValue('active'));
    out('  status      : ' + f.getValue('status'));
    out('  updated     : ' + f.getValue('sys_updated_on') + ' by ' + f.getValue('sys_updated_by'));
    out('  description : ' + (f.getValue('description') || '-'));

    /* The trigger's condition lives on the trigger instance, not the flow. */
    var tr = new GlideRecord('sys_hub_trigger_instance');
    tr.addQuery('flow', f.getUniqueValue());
    tr.query();
    while (tr.next()) {
      out('');
      out('  TRIGGER');
      out('    table      : ' + tr.getValue('table'));
      out('    type       : ' + tr.getDisplayValue('trigger_type'));
      /* THE QUESTION: is there a u_state=published filter? Without one, a
         DRAFT status update emails itself out the moment it is created. */
      out('    condition  : ' + (tr.getValue('condition') || '(NONE — every insert fires this flow)'));
      out('    filter     : ' + (tr.getValue('filter_condition') || '-'));
    }

    /* Action instances carry the input bindings — the data pills that decide
       what "to" and "ccList" actually are. */
    rule('ACTION INPUT BINDINGS (what feeds to / subject / ccList / content)');
    var ai = new GlideRecord('sys_hub_action_instance');
    ai.addQuery('flow', f.getUniqueValue());
    ai.orderBy('order');
    ai.query();
    var found = false;
    while (ai.next()) {
      found = true;
      out('');
      out('  step ' + ai.getValue('order') + ': ' + ai.getDisplayValue('action_type'));
      var vals = ai.getValue('values');
      out('    values: ' + (vals || '-'));
    }
    if (!found) {
      out('  (no sys_hub_action_instance rows — this flow is snapshot-only.');
      out('   Its definition lives in sys_update_xml.payload; PASS 1B dumps it.)');
      rule('PASS 1B — snapshot XML, grepped for the input bindings');
      var x = new GlideRecord('sys_update_xml');
      x.addQuery('name', 'CONTAINS', FLOW_NAME);
      x.orderByDesc('sys_updated_on');
      x.setLimit(3);
      x.query();
      while (x.next()) {
        var payload = String(x.getValue('payload') || '');
        out('  update_xml: ' + x.getValue('name') + '  (' + payload.length + ' chars)');
        /* Print only the neighbourhoods that mention the four input names,
           rather than the whole payload, which will not fit. */
        var names = ['ccList', 'cclist', '"to"', 'subject', 'content'];
        for (var n = 0; n < names.length; n++) {
          var idx = payload.indexOf(names[n]);
          if (idx > -1) out('    ...' + payload.substr(Math.max(0, idx - 200), 400) + '...');
        }
      }
    }
  }

  rule('THE ACTION ITSELF: ' + ACTION_NAME);
  var a = new GlideRecord('sys_hub_action_type_definition');
  a.addQuery('name', ACTION_NAME);
  a.query();
  while (a.next()) {
    out('  sys_id: ' + a.getUniqueValue() + '   updated: ' + a.getValue('sys_updated_on'));
    var st = new GlideRecord('sys_hub_step');
    st.addQuery('action_type', a.getUniqueValue());
    st.orderBy('order');
    st.query();
    while (st.next()) {
      out('    step ' + st.getValue('order') + ': ' + st.getDisplayValue('step_type'));
      out('      values: ' + (st.getValue('values') || '-'));
    }
  }
}

/* -------------------------------------------------------------------------
   PASS 2 — recipient reality. This is the pass that decides the port.
   ------------------------------------------------------------------------- */
if (PASS == 2) {
  rule('RECIPIENT REALITY on ' + TABLE);

  var total = 0, withList = 0, external = 0;
  var delimiters = {}, externalRows = [];
  var byVisibility = {}, byScope = {};

  var gr = new GlideRecord(TABLE);
  gr.query();
  while (gr.next()) {
    total++;
    var list = String(gr.getValue('u_mailing_list') || '').trim();
    var vis  = gr.getDisplayValue('u_visibility') || '(none)';
    var scope = gr.getDisplayValue('u_scope') || '(none)';
    byVisibility[vis] = (byVisibility[vis] || 0) + 1;
    byScope[scope] = (byScope[scope] || 0) + 1;

    if (!list) continue;
    withList++;

    /* What actually separates the addresses? The port splits on this. */
    if (list.indexOf(';') > -1) delimiters[';'] = (delimiters[';'] || 0) + 1;
    if (list.indexOf(',') > -1) delimiters[','] = (delimiters[','] || 0) + 1;
    if (/\s/.test(list) && list.indexOf(',') == -1 && list.indexOf(';') == -1) {
      delimiters['whitespace only'] = (delimiters['whitespace only'] || 0) + 1;
    }

    var parts = list.split(/[\s,;]+/);
    var hasExternal = false;
    for (var i = 0; i < parts.length; i++) {
      var e = parts[i].trim().toLowerCase();
      if (e && e.indexOf('@') > -1 && e.indexOf('@wso2.com') == -1) hasExternal = true;
    }
    if (hasExternal) {
      external++;
      externalRows.push({ vis: vis, scope: scope, state: gr.getDisplayValue('u_state') });
    }
  }

  out('  total status updates      : ' + total);
  out('  with a mailing list       : ' + withList);
  out('  containing a NON-WSO2 addr: ' + external);
  out('');
  out('  delimiters seen in u_mailing_list:');
  for (var d in delimiters) out('     "' + d + '" on ' + delimiters[d] + ' rows');
  out('');
  out('  by visibility:');
  for (var v in byVisibility) out('     ' + v + ': ' + byVisibility[v]);
  out('  by scope:');
  for (var sc in byScope) out('     ' + sc + ': ' + byScope[sc]);

  out('');
  out('  rows carrying an external address:');
  if (!externalRows.length) {
    out('     (none)');
    out('');
    out('  >>> READING (a): the @wso2.com check was intended and simply broken.');
    out('  >>> The port should enforce it. No existing send would have been blocked.');
  } else {
    for (var r = 0; r < externalRows.length; r++) {
      out('     visibility=' + externalRows[r].vis + '  scope=' + externalRows[r].scope +
          '  state=' + externalRows[r].state);
    }
    out('');
    out('  >>> External addresses EXIST. If they cluster on visibility=Customer,');
    out('  >>> that is READING (b) and a flat @wso2.com filter would break real');
    out('  >>> sends — the port must gate the filter on visibility instead.');
  }
}

/* -------------------------------------------------------------------------
   PASS 3 — anything else that writes to this table. The port assumes the
            create endpoint is the only path that can trigger a send.
   ------------------------------------------------------------------------- */
if (PASS == 3) {
  rule('OTHER WRITERS to ' + TABLE);

  var br = new GlideRecord('sys_script');
  br.addQuery('collection', TABLE);
  br.query();
  out('  business rules: ' + br.getRowCount());
  while (br.next()) {
    out('    - ' + br.getValue('name') + '  [' + (br.getValue('active') == '1' ? 'active' : 'inactive') + ']' +
        '  insert=' + br.getValue('action_insert') + ' update=' + br.getValue('action_update'));
  }

  /* sys_hub_trigger_instance does NOT necessarily expose the trigger's table
     as a field called "table". addQuery on a field that does not exist is
     silently IGNORED by GlideRecord — the query then matches every row, which
     is why a first version of this pass reported "3633 flows triggered by this
     table", i.e. every flow on the instance. Detect the real field first and
     refuse to guess. */
  var trg = new GlideRecord('sys_hub_trigger_instance');
  var tableField = '';
  var candidates = ['table', 'table_name', 'collection', 'trigger_table'];
  for (var c = 0; c < candidates.length; c++) {
    if (trg.isValidField(candidates[c])) { tableField = candidates[c]; break; }
  }
  out('');
  if (!tableField) {
    out('  !! sys_hub_trigger_instance has none of: ' + candidates.join(', '));
    out('  !! Cannot filter by table. Listing the field names instead so the');
    out('  !! right one can be added to the candidates list above:');
    var els = new GlideRecord('sys_dictionary');
    els.addQuery('name', 'sys_hub_trigger_instance');
    els.addQuery('element', '!=', '');
    els.orderBy('element');
    els.query();
    var names = [];
    while (els.next()) names.push(els.getValue('element'));
    out('     ' + names.join(', '));
  } else {
    trg.addQuery(tableField, TABLE);
    trg.query();
    out('  flows triggered by this table (via "' + tableField + '"): ' + trg.getRowCount());
    /* A count in the thousands means the filter was ignored again — say so
       rather than printing every flow on the instance. */
    if (trg.getRowCount() > 50) {
      out('  !! That count is implausible — the filter was not applied.');
      out('  !! Do not trust this section; report the number and stop.');
    } else {
      while (trg.next()) {
        out('    - ' + trg.getDisplayValue('flow') + '  (' + trg.getDisplayValue('trigger_type') + ')');
      }
    }
  }

  var si = new GlideRecord('sys_script_include');
  si.addQuery('script', 'CONTAINS', TABLE);
  si.query();
  out('');
  out('  script includes referencing the table: ' + si.getRowCount());
  while (si.next()) out('    - ' + si.getValue('name'));
}

/* -------------------------------------------------------------------------
   PASS 4 — one recent row, end to end.
   ------------------------------------------------------------------------- */
if (PASS == 4) {
  rule('MOST RECENT STATUS UPDATES');
  out('*** REAL EMAIL ADDRESSES BELOW — SCRUB BEFORE SHARING ***');
  var gr = new GlideRecord(TABLE);
  gr.orderByDesc('sys_created_on');
  gr.setLimit(3);
  gr.query();
  while (gr.next()) {
    out('');
    out('  sys_id      : ' + gr.getUniqueValue());
    out('  created     : ' + gr.getValue('sys_created_on') + ' by ' + gr.getValue('sys_created_by'));
    out('  engagement  : ' + gr.getDisplayValue('u_engagement'));
    out('  allocation  : ' + (gr.getDisplayValue('u_allocation') || '(empty)'));
    out('  author      : ' + gr.getDisplayValue('u_author'));
    out('  subject     : ' + gr.getValue('u_subject'));
    out('  state       : ' + gr.getDisplayValue('u_state'));
    out('  frequency   : ' + gr.getDisplayValue('u_frequency'));
    out('  scope       : ' + gr.getDisplayValue('u_scope'));
    out('  visibility  : ' + gr.getDisplayValue('u_visibility'));
    out('  cycle start : ' + gr.getValue('u_cycle_start_date'));
    out('  published   : ' + gr.getValue('u_published_date'));
    out('  mailing list: ' + (gr.getValue('u_mailing_list') || '(empty)'));
    var content = String(gr.getValue('u_content') || '');
    out('  content     : ' + content.length + ' chars, starts: ' + content.substr(0, 160));
  }
}

out('');
out('=== PASS ' + PASS + ' complete. Used ' + used + '/' + BUDGET + ' chars. ===');

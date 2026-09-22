/* ============================================================================
   41 — THE CUSTOMER ENGAGEMENT / ALLOCATION / STATUS UPDATE TABLES.
        Run in: System Definition > Scripts - Background  (/sys.scripts.do)
        READ-ONLY. No writes, no updates, no deletes.

   Scope: the three u_ tables behind WeeklyAllocationStatusUpdateReminderEmailFlow,
   ported to this component's allocation_status_update_reminder sub-cron. None
   of them were mapped in csm-sync-service, so all three needed a mapping YAML
   plus a migration before the port had anything to read.

     u_customer_engagement                       the engagement (u_state gates it)
     u_customer_engagement_allocation_resource   who is allocated, and when
     u_customer_engagement_status_update         the weekly update itself

   ── WHY THIS SCRIPT ─────────────────────────────────────────────────────
   A csm-sync-service mapping is field-for-field: every column of the source
   table gets a target column and a type. The flow's own script only touches
   six fields, so porting from the script alone would produce a mapping that
   silently drops the rest of the table and goes stale the moment anything
   else wants to read it. PASS 1 is the whole dictionary, which is what the
   mapping actually has to be written from.

   PASS 2 matters just as much: the flow filters on u_engagement.u_state == "1"
   and nothing anywhere records what "1" MEANS. If that choice list turns out
   to have more than one active-ish value, the port's WHERE clause is wrong.

   ── RUN IT IN PASSES ────────────────────────────────────────────────────
   Set PASS below, run, paste the output, then bump PASS and run again.

     PASS 1  dictionary: every column, type, length, reference target  <- start
     PASS 2  choice lists for every choice field (u_state above all)
     PASS 3  row counts, date ranges, reference density (how many nulls)
     PASS 4  replay the flow's own query — how many people WOULD be emailed
             *** PASS 4 PRINTS REAL NAMES AND EMAILS — SCRUB BEFORE SHARING ***
   ======================================================================== */

var PASS = 1;

var TABLES = [
  'u_customer_engagement',
  'u_customer_engagement_allocation_resource',
  'u_customer_engagement_status_update'
];

/* Output budget. Scripts - Background truncates silently; this makes the
   truncation visible instead. Scripts - Background truncates silently. */
var BUDGET = 60000;
var used = 0;
var truncated = false;

function out(s) {
  s = String(s);
  if (truncated) return;
  if (used + s.length > BUDGET) {
    gs.info('*** BUDGET HIT — OUTPUT TRUNCATED. Narrow TABLES and rerun. ***');
    truncated = true;
    return;
  }
  used += s.length;
  gs.info(s);
}

function rule(title) {
  out('');
  out('================================================================');
  out('  ' + title);
  out('================================================================');
}

function tableExists(name) {
  var gr = new GlideRecord('sys_db_object');
  gr.addQuery('name', name);
  gr.query();
  return gr.next() ? gr.getValue('label') : null;
}

/* -------------------------------------------------------------------------
   PASS 1 — the dictionary. This is the input to the mapping YAML.
   ------------------------------------------------------------------------- */
if (PASS == 1) {
  for (var i = 0; i < TABLES.length; i++) {
    var t = TABLES[i];
    var label = tableExists(t);
    rule('DICTIONARY: ' + t + (label ? '  ("' + label + '")' : '  *** TABLE NOT FOUND ***'));
    if (!label) continue;

    /* Include inherited columns: a u_ table extending task or similar carries
       fields the mapping still has to account for. name IN (table, parents). */
    var gr = new GlideRecord('sys_dictionary');
    gr.addQuery('name', t);
    gr.addQuery('element', '!=', '');
    gr.orderBy('element');
    gr.query();

    out('element | type | len | reference | mandatory | default | label');
    out('--------+------+-----+-----------+-----------+---------+------');
    while (gr.next()) {
      out([
        gr.getValue('element'),
        gr.getValue('internal_type'),
        gr.getValue('max_length'),
        gr.getValue('reference') || '-',
        gr.getValue('mandatory') == '1' ? 'Y' : '-',
        gr.getValue('default_value') || '-',
        gr.getDisplayValue('column_label')
      ].join(' | '));
    }
  }
  out('');
  out('NOTE: every "reference" above becomes a FK in the migration ONLY if its');
  out('target table is itself synced. sys_user is (mapping sys_user.yaml); an');
  out('unsynced target has to stay a plain sys_id string or the loader drops');
  out('every row whose parent has not been seen yet.');
}

/* -------------------------------------------------------------------------
   PASS 2 — choice lists. u_state == "1" is the flow's whole activity filter.
   ------------------------------------------------------------------------- */
if (PASS == 2) {
  for (var j = 0; j < TABLES.length; j++) {
    var tbl = TABLES[j];
    rule('CHOICES: ' + tbl);

    var ch = new GlideRecord('sys_choice');
    ch.addQuery('name', tbl);
    ch.orderBy('element');
    ch.orderBy('sequence');
    ch.query();

    var seen = '';
    var any = false;
    while (ch.next()) {
      any = true;
      var el = ch.getValue('element');
      if (el != seen) {
        out('');
        out('  ' + el + ':');
        seen = el;
      }
      out('     value="' + ch.getValue('value') + '"  label="' +
          ch.getValue('label') + '"  inactive=' + (ch.getValue('inactive') == '1' ? 'Y' : '-'));
    }
    if (!any) out('  (no sys_choice rows — the field may be a plain integer/string)');
  }
  out('');
  out('*** THE ONE THAT MATTERS: u_customer_engagement.u_state value "1".    ***');
  out('*** The flow treats ONLY "1" as active. If the list has a second      ***');
  out('*** not-yet-closed value, the port must decide whether to include it. ***');
}

/* -------------------------------------------------------------------------
   PASS 3 — shape of the data. Sizing the sync, and finding null-heavy refs.
   ------------------------------------------------------------------------- */
if (PASS == 3) {
  for (var k = 0; k < TABLES.length; k++) {
    var tn = TABLES[k];
    rule('SHAPE: ' + tn);

    var agg = new GlideAggregate(tn);
    agg.addAggregate('COUNT');
    agg.query();
    var total = agg.next() ? agg.getAggregate('COUNT') : '0';
    out('  total rows: ' + total);

    /* Oldest and newest, to size the initial extract window. */
    var oldest = new GlideRecord(tn);
    oldest.orderBy('sys_created_on');
    oldest.setLimit(1);
    oldest.query();
    if (oldest.next()) out('  oldest sys_created_on: ' + oldest.getValue('sys_created_on'));

    var newest = new GlideRecord(tn);
    newest.orderByDesc('sys_updated_on');
    newest.setLimit(1);
    newest.query();
    if (newest.next()) out('  newest sys_updated_on: ' + newest.getValue('sys_updated_on'));
  }

  rule('REFERENCE DENSITY (empty refs = rows a FK would reject)');
  var checks = [
    ['u_customer_engagement_allocation_resource', 'u_engagement'],
    ['u_customer_engagement_allocation_resource', 'u_resource'],
    ['u_customer_engagement_allocation_resource', 'u_start_date'],
    ['u_customer_engagement_allocation_resource', 'u_end_date'],
    ['u_customer_engagement_status_update',       'u_engagement'],
    ['u_customer_engagement_status_update',       'u_author'],
    ['u_customer_engagement_status_update',       'u_cycle_start_date']
  ];
  for (var c = 0; c < checks.length; c++) {
    var g = new GlideAggregate(checks[c][0]);
    g.addQuery(checks[c][1], '');
    g.addAggregate('COUNT');
    g.query();
    var empties = g.next() ? g.getAggregate('COUNT') : '0';
    out('  ' + checks[c][0] + '.' + checks[c][1] + ' empty on ' + empties + ' rows');
  }
  out('');
  out('An empty u_resource or u_engagement means the reminder query silently');
  out('skips that allocation today, and would break a NOT NULL column tomorrow.');
}

/* -------------------------------------------------------------------------
   PASS 4 — replay the flow's query. Establishes the expected blast radius
            BEFORE the Go port sends anything.
   ------------------------------------------------------------------------- */
if (PASS == 4) {
  rule('REPLAY: WeeklyAllocationStatusUpdateReminderEmailFlow');
  out('*** REAL NAMES AND EMAIL ADDRESSES BELOW — SCRUB BEFORE SHARING ***');
  out('');

  var lastMondayDate = new GlideDate();
  lastMondayDate.addDays(-7);
  out('script "last week" date: ' + lastMondayDate.getValue() +
      '   (today: ' + new GlideDate().getValue() + ')');
  out('NOTE: the real flow runs Monday 00:00, so this only reproduces the live');
  out('result if you run it on a Monday. Any other day shifts the window.');
  out('');

  var allocations = 0;
  var faithful = [];   /* per-ENGAGEMENT check — what the flow does today */
  var fixed = [];      /* per-RESOURCE check — what the script intended  */

  var gr = new GlideRecord('u_customer_engagement_allocation_resource');
  gr.addQuery('u_engagement.u_state', '=', '1');
  gr.addQuery('u_start_date', '<=', lastMondayDate);
  gr.addQuery('u_end_date', '>=', lastMondayDate);
  gr.query();

  while (gr.next()) {
    allocations++;
    var engagementSysId = gr.u_engagement.sys_id;
    var resourceSysId = gr.u_resource.sys_id;
    var email = String(gr.u_resource.email || '');

    /* (a) exactly what the flow does: engagement-wide, no author filter,
       because its u_author addQuery lands on the wrong GlideRecord. */
    var anyUpdate = new GlideRecord('u_customer_engagement_status_update');
    anyUpdate.addQuery('u_engagement.sys_id', '=', engagementSysId);
    anyUpdate.addQuery('u_cycle_start_date', '>=', lastMondayDate);
    anyUpdate.query();
    if (anyUpdate.getRowCount() == 0 && email && faithful.indexOf(email) < 0) {
      faithful.push(email);
    }

    /* (b) what it was meant to do: this person's own update. */
    var mine = new GlideRecord('u_customer_engagement_status_update');
    mine.addQuery('u_engagement.sys_id', '=', engagementSysId);
    mine.addQuery('u_author.sys_id', '=', resourceSysId);
    mine.addQuery('u_cycle_start_date', '>=', lastMondayDate);
    mine.query();
    if (mine.getRowCount() == 0 && email && fixed.indexOf(email) < 0) {
      fixed.push(email);
    }
  }

  out('active allocations covering last Monday: ' + allocations);
  out('');
  out('recipients, FAITHFUL (per-engagement, today\'s behaviour): ' + faithful.length);
  out('recipients, FIXED    (per-resource, intended behaviour):  ' + fixed.length);
  out('');
  out('The gap between those two numbers IS the bug: people who owe an update');
  out('but are covered by a colleague having posted one on the same engagement.');
  out('');
  out('-- faithful --');
  out(faithful.join('\n'));
  out('');
  out('-- fixed only (would newly start receiving the reminder) --');
  var newly = [];
  for (var f = 0; f < fixed.length; f++) {
    if (faithful.indexOf(fixed[f]) < 0) newly.push(fixed[f]);
  }
  out(newly.length ? newly.join('\n') : '(none — the two sets agree this week)');
}

out('');
out('=== PASS ' + PASS + ' complete. Output used ' + used + '/' + BUDGET + ' chars. ===');

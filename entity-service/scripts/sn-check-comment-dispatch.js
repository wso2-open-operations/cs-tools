// Does ServiceNow's outbound flow dispatch SYSTEM-generated case comments to
// GitHub, or only ones a person wrote?
//
// HOW TO RUN: paste into System Definition > Scripts - Background on the
// instance that runs the integration (prod for a real answer). READ ONLY --
// no insert, update or delete.
//
// WHY THIS IS BEING ASKED. The native port mirrors case comments to the linked
// GitHub issue. On staging, 7,915 case comments are authored by 'system' --
// auto-closure reminders addressed to "Hi team" -- and 278 are the "Change
// request (CHGxxxxxxx) is created." notices that the port now generates
// itself. Whether those should reach a customer's issue depends on whether
// ServiceNow already sent them. Reproducing the old behaviour and quietly
// changing it are both defensible; doing so without knowing which is not.

(function () {
  var out = [];
  function say(s) { out.push(s == null ? '' : String(s)); }
  function rule(t) { say(''); say('--- ' + t + ' ---'); }

  say('========== ServiceNow: is system comment dispatch real? ==========');
  say('instance : ' + gs.getProperty('instance_name'));
  say('run      : ' + new GlideDateTime().getDisplayValue());

  // ------------------------------------------------------------------ 1
  // Find the flow(s). Named by hand in the console, so match loosely rather
  // than assuming the exact string.
  rule('1. candidate flows');
  var flowIds = [];
  var f = new GlideRecord('sys_hub_flow');
  f.addEncodedQuery('nameLIKEgithub^ORnameLIKEgit^ORnameLIKEcomment');
  f.orderBy('name');
  f.query();
  while (f.next()) {
    say('  ' + (f.getValue('active') === '1' ? '[ACTIVE]  ' : '[inactive]') +
        ' ' + f.getValue('name') + '   sys_id=' + f.getUniqueValue());
    flowIds.push({ id: f.getUniqueValue(), name: f.getValue('name'), active: f.getValue('active') });
  }
  if (!flowIds.length) say('  none found -- widen the query before concluding anything.');

  // ------------------------------------------------------------------ 2
  // The trigger decides what fires it. A condition naming sys_created_by or
  // a user field is the thing that would exclude system writes.
  rule('2. trigger configuration (what fires each flow)');
  flowIds.forEach(function (fl) {
    var t = new GlideRecord('sys_hub_trigger_instance');
    t.addQuery('flow', fl.id);
    t.query();
    while (t.next()) {
      say('  flow: ' + fl.name);
      say('    trigger type : ' + t.getValue('trigger_type'));
      say('    table        : ' + t.getValue('table_name'));
      var cond = t.getValue('condition');
      say('    condition    : ' + (cond ? cond : '(none -- fires on every matching change)'));
      // Trigger inputs live as name/value pairs against the instance.
      var v = new GlideRecord('sys_variable_value');
      v.addQuery('document_key', t.getUniqueValue());
      v.query();
      while (v.next()) {
        var val = v.getValue('value');
        if (val) say('    input        : ' + v.variable.element + ' = ' + val);
      }
      if (cond && /sys_created_by|sys_updated_by|user|author/i.test(cond)) {
        say('    >> the condition REFERENCES AN AUTHOR -- read it closely, this is');
        say('       the filter that would exclude system-generated entries.');
      } else {
        say('    >> no author filter visible on the trigger.');
      }
    }
  });

  // ------------------------------------------------------------------ 3
  // The journal itself. In ServiceNow a case comment is a sys_journal_field
  // row with element='comments' (customer visible) or 'work_notes' (internal).
  // If system entries land in 'comments', a comments-changed trigger sees them.
  rule('3. where system-generated entries actually land');
  var agg = new GlideAggregate('sys_journal_field');
  agg.addQuery('name', 'sn_customerservice_case');
  agg.addEncodedQuery('sys_created_onRELATIVEGT@month@ago@12');
  agg.groupBy('element');
  agg.groupBy('sys_created_by');
  agg.addAggregate('COUNT');
  agg.orderByAggregate('COUNT');
  agg.query();
  var rows = 0, sysInComments = 0;
  while (agg.next() && rows < 40) {
    rows++;
    var el = agg.getValue('element'), by = agg.getValue('sys_created_by'),
        n  = agg.getAggregate('COUNT');
    say('  ' + padTo(el, 14) + ' ' + padTo(by, 26) + ' x' + n);
    if (el === 'comments' && isSystemish(by)) sysInComments += parseInt(n, 10);
  }
  say('');
  say('  system-ish authors writing to customer-visible "comments": ' + sysInComments);
  if (sysInComments > 0) {
    say('  >> ServiceNow DOES put system-generated text in the customer-visible');
    say('     journal. A "comments changes" trigger therefore SEES them, and');
    say('     unless the trigger condition above excludes them, ServiceNow has');
    say('     been dispatching this text to GitHub all along.');
  } else {
    say('  >> No system-generated entries in the customer-visible journal, so');
    say('     ServiceNow never dispatched any. Filtering them in the port');
    say('     matches the old behaviour rather than changing it.');
  }

  // ------------------------------------------------------------------ 4
  // The decisive evidence: a case that is linked to an issue AND has a
  // system-authored comment. If the flow dispatched it, the text is on the
  // issue. Prints what to go and look at.
  rule('4. cases to verify by eye on GitHub');
  var checked = 0;
  var cr = new GlideRecord('change_request');
  cr.addNotNullQuery('u_git_reference');
  cr.addNotNullQuery('parent');
  cr.orderByDesc('sys_updated_on');
  cr.setLimit(200);
  cr.query();
  while (cr.next() && checked < 5) {
    var parentId = cr.getValue('parent');
    var j = new GlideRecord('sys_journal_field');
    j.addQuery('element_id', parentId);
    j.addQuery('element', 'comments');
    j.addQuery('sys_created_by', 'system');
    j.orderByDesc('sys_created_on');
    j.setLimit(1);
    j.query();
    if (j.next()) {
      checked++;
      say('  issue   : ' + cr.getValue('u_git_reference'));
      say('  case    : ' + parentId);
      say('  comment : "' + String(j.getValue('value')).substring(0, 90).replace(/\s+/g, ' ') + '"');
      say('  written : ' + j.getValue('sys_created_on'));
      say('  -> open that issue. If this text is on it, ServiceNow dispatched it.');
      say('');
    }
  }
  if (!checked) {
    say('  none found: no linked case in the sample has a system-authored');
    say('  customer-visible comment. That itself suggests the situation does');
    say('  not arise in practice on linked cases.');
  }

  say('');
  say('========== end ==========');

  function padTo(s, n) { s = String(s == null ? '' : s); while (s.length < n) s += ' '; return s; }
  function isSystemish(u) {
    return /^(system|guest|admin|.*_integration|.*_pipeline|.*integration.*)$/i.test(String(u || ''));
  }

  gs.print(out.join('\n'));
})();

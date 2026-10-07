/* ===========================================================================
   72 — What ServiceNow's CSM API does when it creates a problem for an
        incident. Read-only. Run in Scripts - Background, paste the output
        back.

   In dual-write, the workaround problem (post-resolution flow) has to be
   created in ServiceNow first, so it gets a real PRB number and can be moved
   through its states. entity-service sends POST /problems with subject,
   description, category, subcategory, originCaseId and primaryIncidentId only.
   The flow also sets service, impact, urgency and assignment group, and links
   the incident back. This prints:

     1  ProblemUtils: the createProblem function and every payload key it
        reads (does it take businessService / impact / urgency /
        assignmentGroup / priority?), and what it does with primaryIncidentId
     2  the scripted REST operations behind POST /problems
     3  business rules on problem insert that write incident.problem_id or
        read u_incident
     4  the "[WSO2 Cloud Ops] Post resolution tasks" flow: active? published?
        (if it runs in ServiceNow, a mirrored resolve creates a problem there)
     5  evidence: recent problems with u_incident set, created through the
        API, and whether their incident's problem_id points back to them
   ======================================================================== */

var BUDGET = 40000, used = 0, truncated = false;
function out(s) {
  s = String(s);
  if (truncated) return;
  if (used + s.length > BUDGET) { gs.info('*** BUDGET HIT -- TRUNCATED ***'); truncated = true; return; }
  used += s.length; gs.info(s);
}
function rule(t) { out(''); out('==== ' + t + ' ===='); }
function safe(label, fn) { try { fn(); } catch (e) { out('!! ' + label + ' failed: ' + e); } }
function cut(s, n) { s = '' + (s || ''); return s.length > n ? s.substring(0, n) + '\n  ...[' + s.length + ' chars]' : s; }

// The body of `name: function` (or `name = function`) in a script, up to the
// next top-level member.
function fnBody(script, name) {
  var re = new RegExp('(^|\\n)\\s*' + name + '\\s*[:=]\\s*function');
  var m = re.exec(script);
  if (!m) return null;
  var start = m.index, depth = 0, i = script.indexOf('{', start);
  for (; i < script.length; i++) {
    if (script[i] === '{') depth++;
    else if (script[i] === '}' && --depth === 0) return script.substring(start, i + 1);
  }
  return script.substring(start);
}

out('instance: ' + gs.getProperty('instance_name'));

rule('1. ProblemUtils.createProblem');
safe('script include', function () {
  var si = new GlideRecord('sys_script_include');
  si.addQuery('name', 'ProblemUtils');
  si.query();
  if (!si.next()) { out('(no ProblemUtils script include)'); return; }
  var script = si.getValue('script') || '';
  out('ProblemUtils (' + script.length + ' chars), api_name ' + si.getValue('api_name'));
  var body = fnBody(script, 'createProblem');
  out(body ? cut(body, 6000) : '(no createProblem member found)');
  // Helpers createProblem calls, by name.
  if (body) {
    var calls = {}, re = /this\.(_?[A-Za-z0-9]+)\(/g, m;
    while ((m = re.exec(body))) calls[m[1]] = true;
    for (var c in calls) {
      var b = fnBody(script, c);
      if (b) { out(''); out('-- helper ' + c + ':'); out(cut(b, 2500)); }
    }
  }
  // Every payload key ProblemUtils reads anywhere, for the field list.
  var keys = {}, kre = /payload\.([A-Za-z0-9_]+)|payload\[['"]([A-Za-z0-9_]+)['"]\]|key:\s*['"]([A-Za-z0-9_]+)['"]/g, k;
  while ((k = kre.exec(script))) keys[k[1] || k[2] || k[3]] = true;
  out('');
  out('payload keys read anywhere in ProblemUtils: ' + Object.keys(keys).sort().join(', '));
});

rule('2. Scripted REST operations for POST problems');
safe('rest', function () {
  var op = new GlideRecord('sys_ws_operation');
  op.addQuery('http_method', 'POST');
  op.addQuery('relative_path', 'CONTAINS', 'problem');
  op.addActiveQuery();
  op.query();
  while (op.next()) {
    out('* ' + op.web_service_definition.getDisplayValue() + ' ' + op.getValue('http_method') + ' ' + op.getValue('relative_path'));
    out(cut(op.getValue('operation_script'), 2500));
  }
});

rule('3. Business rules on problem that touch incident.problem_id / u_incident');
safe('business rules', function () {
  var br = new GlideRecord('sys_script');
  br.addQuery('collection', 'IN', 'problem,incident');
  br.addActiveQuery();
  br.query();
  while (br.next()) {
    var body = br.getValue('script') || '';
    if (!/problem_id|u_incident/.test(body + (br.getValue('filter_condition') || ''))) continue;
    out('* [' + br.getValue('collection') + '] "' + br.getValue('name') + '" | ' + br.getValue('when') +
        ' | insert ' + br.getValue('action_insert') + ' update ' + br.getValue('action_update') +
        ' | filter: ' + (br.getValue('filter_condition') || '-'));
    out(cut(body, 1200));
  }
});

rule('4. The post-resolution flow in ServiceNow');
safe('flow', function () {
  var f = new GlideRecord('sys_hub_flow');
  f.addQuery('name', 'CONTAINS', 'Post resolution');
  f.query();
  while (f.next()) out('* "' + f.getValue('name') + '" | active ' + f.getValue('active') + ' | status ' + f.getValue('status') +
                       ' | type ' + f.getValue('type') + ' | updated ' + f.getValue('sys_updated_on'));
  var ctx = new GlideAggregate('sys_flow_context');
  ctx.addQuery('name', 'CONTAINS', 'Post resolution');
  ctx.addQuery('sys_created_on', '>=', gs.daysAgoStart(30));
  ctx.addAggregate('COUNT');
  ctx.query();
  if (ctx.next()) out('runs in the last 30 days: ' + ctx.getAggregate('COUNT'));
});

rule('5. Recent problems with a primary incident: does the incident point back?');
safe('evidence', function () {
  var p = new GlideRecord('problem');
  p.addNotNullQuery('u_incident');
  p.orderByDesc('sys_created_on');
  p.setLimit(8);
  p.query();
  while (p.next()) {
    var inc = p.u_incident.getRefRecord();
    out(p.getValue('number') + ' | created ' + p.getValue('sys_created_on') + ' by ' + p.getValue('sys_created_by') +
        ' | u_incident ' + inc.getValue('number') + ' (problem_id ' + (inc.problem_id.getDisplayValue() || '-') + ')' +
        ' | service "' + (p.getDisplayValue('business_service') || '-') + '" impact ' + p.getValue('impact') +
        ' urgency ' + p.getValue('urgency') + ' priority ' + p.getValue('priority') +
        ' | group "' + (p.getDisplayValue('assignment_group') || '-') + '"');
  }
});

out('');
out('done (' + used + ' chars)');

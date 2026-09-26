// How does a GitHub issue get created for a case, and what writes its number
// back onto the case?
//
// This is the one mechanism the port has no equivalent for. Our GitHub client
// has five operations -- comment, list comments, set labels, remove label, set
// state -- and no create-issue; and nothing in our code or migrations ever
// writes case.github_issue_number. So the outbound sync can only fire for a
// case somebody linked by hand.
//
// The only lead is CaseGithubIssuesCreateAPI, named once in the porting spec.
// This finds it, prints it, and finds everything else that touches the case's
// issue field.
//
// READ ONLY. Paste into System Definition > Scripts - Background.

(function () {
  var out = [];
  function say(s) { out.push(s == null ? '' : String(s)); }
  function rule(t) { say(''); say('--- ' + t + ' ---'); }
  function pad(s, n) { s = String(s == null ? '' : s); while (s.length < n) s += ' '; return s; }

  say('===== how does issue creation happen? =====');
  say('instance: ' + gs.getProperty('instance_name'));

  // ---------------------------------------------------------------- 1
  // Which field on the case holds the issue number. Discovered, not guessed:
  // a wrong column name and an empty result look identical.
  rule('1. the case field that holds the issue number');
  var found = [];
  ['git', 'issue', 'github', 'repo'].forEach(function (needle) {
    var d = new GlideRecord('sys_dictionary');
    d.addQuery('name', 'sn_customerservice_case');
    d.addQuery('element', 'CONTAINS', needle);
    d.query();
    while (d.next()) {
      var el = d.getValue('element');
      if (found.indexOf(el) !== -1) continue;
      found.push(el);
      say('  ' + pad(el, 28) + pad(d.getValue('internal_type'), 14) + '"' + d.getValue('column_label') + '"');
    }
  });
  if (!found.length) say('  NONE. The link may live on a separate table.');

  // How many cases actually carry a value, and how recently.
  found.forEach(function (el) {
    try {
      var a = new GlideAggregate('sn_customerservice_case');
      a.addNotNullQuery(el);
      a.addAggregate('COUNT');
      a.query();
      var n = a.next() ? a.getAggregate('COUNT') : 0;
      say('  ' + pad(el, 28) + 'populated on ' + n + ' cases');
    } catch (e) { /* not queryable */ }
  });

  // ---------------------------------------------------------------- 2
  rule('2. CaseGithubIssuesCreateAPI');
  var si = new GlideRecord('sys_script_include');
  si.addQuery('name', 'CONTAINS', 'CaseGithubIssues');
  si.query();
  var gotInclude = false;
  while (si.next()) {
    gotInclude = true;
    say('  script include: ' + si.getValue('name') + '   active=' + si.getValue('active'));
    say('  ---8<--- script ---8<---');
    say(si.getValue('script'));
    say('  ---8<--- end ---8<---');
  }
  if (!gotInclude) say('  no script include by that name.');

  // Scripted REST: the definition, then each operation with its script.
  var wsd = new GlideRecord('sys_ws_definition');
  wsd.addEncodedQuery('nameLIKEgithub^ORnameLIKEissue^ORnameLIKEcase');
  wsd.query();
  while (wsd.next()) {
    say('');
    say('  REST API: ' + wsd.getValue('name') + '   base=' + wsd.getValue('service_id'));
    var op = new GlideRecord('sys_ws_operation');
    op.addQuery('web_service_definition', wsd.getUniqueValue());
    op.query();
    while (op.next()) {
      say('    operation: ' + op.getValue('http_method') + ' ' + op.getValue('relative_path') +
          '   (' + op.getValue('name') + ')');
      var s = op.getValue('operation_script') || '';
      if (/issue/i.test(s) && /POST|create/i.test(s)) {
        say('    ---8<--- operation script ---8<---');
        say(s.length > 6000 ? s.substring(0, 6000) + '\n    …[' + s.length + ' chars]' : s);
        say('    ---8<--- end ---8<---');
      }
    }
  }

  // ---------------------------------------------------------------- 3
  // Anything that WRITES the field, whatever created the issue.
  rule('3. what writes the case issue field');
  if (found.length) {
    found.forEach(function (el) {
      ['sys_script', 'sys_script_include', 'sys_ws_operation'].forEach(function (table) {
        var g = new GlideRecord(table);
        g.addQuery('script', 'CONTAINS', el);
        g.query();
        while (g.next()) {
          say('  ' + pad(table, 20) + g.getValue('name') +
              (g.isValidField('active') ? '   active=' + g.getValue('active') : ''));
        }
      });
    });
  } else {
    say('  (no field identified in section 1, nothing to search for)');
  }

  // ---------------------------------------------------------------- 4
  // The two inactive flows whose names suggest they did this.
  rule('4. the flows named for issue creation');
  ['Create Case from GitHub Issue', 'Github - Create Issue'].forEach(function (nm) {
    var f = new GlideRecord('sys_hub_flow');
    f.addQuery('name', 'CONTAINS', nm);
    f.query();
    while (f.next()) {
      say('  ' + pad(f.getValue('name'), 46) +
          (f.getValue('active') === '1' ? 'ACTIVE' : 'inactive') +
          '   updated ' + f.getValue('sys_updated_on'));
    }
  });

  say('');
  say('===== end =====');
  gs.print(out.join('\n'));
})();

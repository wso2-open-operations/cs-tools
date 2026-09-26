// Extract the GitHub integration's routing table from ServiceNow.
//
// WHAT github.dispatch.config IS. A system property holding one JSON object,
// keyed by ACCOUNT NAME. Each entry is the routing for that customer:
//
//   { "Acme Corp": { "owner": "wso2", "repo": "choreo",
//                    "credential_sys_id": "a1b2..." }, ... }
//
// The three dispatch actions all read it the same way: look up
// config[account_name], decrypt the PAT named by credential_sys_id, then
// POST to https://api.github.com/repos/{owner}/{repo}/dispatches.
//
// This is the table account_github_repo replaces, so this script prints it and
// emits the SQL to seed it. Field names are taken from the live action scripts,
// not guessed.
//
// READ ONLY -- no insert, update or delete. The PAT is never printed; only the
// NAME of the credential record, which is what account_github_repo stores.

(function () {
  var out = [];
  function say(s) { out.push(s == null ? '' : String(s)); }
  function pad(s, n) { s = String(s == null ? '' : s); while (s.length < n) s += ' '; return s; }
  function sq(s) { return String(s == null ? '' : s).replace(/'/g, "''"); }

  say('========== github.dispatch.config ==========');
  say('instance: ' + gs.getProperty('instance_name'));
  say('');

  var raw = gs.getProperty('github.dispatch.config');
  if (!raw) {
    say('NOT SET on this instance. Run this on the instance that actually');
    say('dispatches (prod) before concluding there is nothing to migrate.');
    gs.print(out.join('\n'));
    return;
  }
  say('raw length: ' + raw.length + ' chars');

  var config;
  try {
    config = JSON.parse(raw);
  } catch (e) {
    say('UNPARSEABLE: ' + e);
    say('Inspect by hand -- do not guess the shape.');
    gs.print(out.join('\n'));
    return;
  }

  // ---------------------------------------------------------------- 1
  say('');
  say('--- 1. routing, one row per account ---');
  var rows = [];
  Object.keys(config).forEach(function (accountName) {
    var e = config[accountName] || {};
    var credName = '', credFound = false;
    if (e.credential_sys_id) {
      var c = new GlideRecord('api_key_credentials');
      if (c.get(e.credential_sys_id)) {
        credFound = true;
        // The NAME, never the key. account_github_repo stores a reference.
        credName = c.getValue('name') || '(unnamed)';
      }
    }
    rows.push({ account: accountName, owner: e.owner, repo: e.repo,
                credId: e.credential_sys_id, credName: credName, credFound: credFound });
    say('  ' + pad(accountName, 34) + pad((e.owner || '?') + '/' + (e.repo || '?'), 40) +
        (e.credential_sys_id
          ? (credFound ? 'cred="' + credName + '"' : 'CREDENTIAL MISSING: ' + e.credential_sys_id)
          : 'NO CREDENTIAL'));
  });
  say('');
  say('  ' + rows.length + ' account(s) mapped.');

  // Does any repository serve more than one account? This decides whether the
  // account_github_repo UNIQUE (owner, repository) constraint can hold.
  var byRepo = {};
  rows.forEach(function (r) {
    var k = (r.owner || '?') + '/' + (r.repo || '?');
    (byRepo[k] = byRepo[k] || []).push(r.account);
  });
  var shared = Object.keys(byRepo).filter(function (k) { return byRepo[k].length > 1; });
  say('');
  if (shared.length) {
    say('  *** SHARED REPOSITORIES FOUND ***');
    shared.forEach(function (k) { say('    ' + k + '  <- ' + byRepo[k].join(', ')); });
    say('  The native schema has UNIQUE (owner, repository): one repo, one');
    say('  account. Seeding WILL FAIL on the second account for each repo above,');
    say('  and inbound cannot tell which account an event belongs to. The');
    say('  schema needs changing before go-live.');
  } else {
    say('  No repository is shared between accounts, so UNIQUE (owner,');
    say('  repository) holds and the native schema matches this data.');
  }

  // ---------------------------------------------------------------- 2
  // The REST Message the actions use, and its methods. Confirms the endpoint
  // and shows any header or auth set at definition level.
  say('');
  say('--- 2. the "GitHub Integration" REST message ---');
  var m = new GlideRecord('sys_rest_message');
  m.addQuery('name', 'GitHub Integration');
  m.query();
  if (!m.next()) {
    say('  not found by that name.');
  } else {
    say('  endpoint: ' + m.getValue('rest_endpoint'));
    var fn = new GlideRecord('sys_rest_message_fn');
    fn.addQuery('rest_message', m.getUniqueValue());
    fn.query();
    while (fn.next()) {
      say('    method "' + fn.getValue('function_name') + '"  ' +
          fn.getValue('http_method') + '  ' + fn.getValue('rest_endpoint'));
    }
  }

  // ---------------------------------------------------------------- 3
  // Which of these accounts exist by that exact name, so the seed is checkable
  // before it runs rather than after it half-fails.
  say('');
  say('--- 3. do these account names exist in ServiceNow? ---');
  rows.forEach(function (r) {
    var a = new GlideRecord('customer_account');
    a.addQuery('name', r.account);
    a.query();
    say('  ' + pad(r.account, 34) + (a.next() ? 'found  sys_id=' + a.getUniqueValue() : 'NOT FOUND by that exact name'));
  });

  // ---------------------------------------------------------------- 4
  say('');
  say('--- 4. SQL to seed account_github_repo ---');
  say('-- Review the account names first: the match is by name, and a name that');
  say('-- differs in Postgres inserts nothing. The final SELECT lists any misses.');
  say('BEGIN;');
  rows.forEach(function (r) {
    if (!r.owner || !r.repo) {
      say("-- SKIPPED " + r.account + ": entry has no owner/repo");
      return;
    }
    say("INSERT INTO account_github_repo (id, created_by, updated_by, account_id, owner, repository, credential_ref)");
    say("SELECT gen_random_uuid(), 'sn-cutover', 'sn-cutover', a.id, '" +
        sq(r.owner) + "', '" + sq(r.repo) + "', " +
        (r.credName ? "'" + sq(r.credName) + "'" : 'NULL') +
        " FROM account a WHERE a.name = '" + sq(r.account) + "'");
    say("ON CONFLICT (account_id) DO UPDATE SET owner = EXCLUDED.owner, repository = EXCLUDED.repository;");
  });
  say('-- Any account name that matched nothing:');
  say("SELECT v.name FROM (VALUES");
  say(rows.map(function (r) { return "  ('" + sq(r.account) + "')"; }).join(',\n'));
  say(") AS v(name) LEFT JOIN account a ON a.name = v.name WHERE a.id IS NULL;");
  say('COMMIT;');

  say('');
  say('========== end ==========');
  gs.print(out.join('\n'));
})();

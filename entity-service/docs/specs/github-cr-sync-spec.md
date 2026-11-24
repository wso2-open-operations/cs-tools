# GitHub ↔ Change Request sync — porting spec

Reconstructed from the live ServiceNow instance (2026-09-18), not from
documentation. Sources: `GitHubIssueContentProcessor` (40,431 chars, Global
scope), the `git_i` scripted REST API, and the `Authenticator` script include.

The ServiceNow Flow Designer flows named `[Change Request] *` are thin — the
behaviour below all lives in `GitHubIssueContentProcessor`.

---

## 1. Shape

**Inbound only.** GitHub sends a webhook; ServiceNow mutates a change request
and sometimes writes back to the issue. There is no "CR changed → push to
GitHub" path in this processor.

```
GitHub webhook ──▶ git_i  POST /event ──▶ Authenticator (HMAC-SHA1)
                                     └──▶ GitHubIssueContentProcessor.process()
                                            ├─ mutate change_request
                                            └─ write back to the issue (5 calls)
```

The change request is found by `u_git_reference = issue.html_url`. That column
is the join key for the whole integration.

## 2. The validity gate

`validate()` drops the event unless **all** hold:

| Requirement | Detail |
|---|---|
| `git.integration.user-id` is set | else error and drop |
| sender ≠ that user | **loop prevention** — our own writes come back as webhooks |
| `Type/ChangeRequest` label present | |
| exactly one of `CRType/Normal` / `Standard` / `Emergency` | sets `type` |
| exactly one of `CRScope/Application` / `Infrastructure` | sets `u_crscope` |

Derived from labels, defaulting to 3: `Impact 1|2` → `impact`,
`Likelihood 1|2` → `u_likelihood`.

## 3. Trigger matrix

Events: `issues`, `issue_comment`. Everything else is rejected.

| Event | Action | CR exists? | Behaviour |
|---|---|---|---|
| `issue_comment` | created | yes | comment matching `[CMD::…]` → `handleCommand` (**dead**, see §6); otherwise append to `comments` as an HTML journal entry |
| `issues` | opened | no | **commented out** — creation on open is disabled |
| `issues` | labeled | no | **create the change request** |
| `issues` | edited / labeled / unlabeled | yes | overwrite `short_description`, `description`, `impact`, `u_likelihood`, `assignment_group` |
| `issues` | labeled | yes | state / scope / label handling — see §4 |
| `issues` | closed | yes | only if state = `0` (Review): close with `close_code=successful`. Otherwise **reopen the issue** and comment why |
| `issues` | unlabeled | yes | state and `CRType/*` labels are re-added (protected); others removed from `u_labels` |

Creation is triggered by **labelling**, not by opening. That is an explicit
opt-in and worth preserving.

## 4. Label-driven state

| Label | state |
|---|---|
| Assessed | -3 |
| Authorized | -2 |
| Scheduled | -1 |
| Implemented | 0 |
| Reviewed | 3 |
| Closed | 3 |
| Canceled | 4 |

Guards, all of which write back to GitHub:

- **Closed CR (state 3)**: label adds and removes are reverted on the issue and
  answered with a comment explaining why.
- **`CRType/*` added**: removed again immediately — type is fixed at creation.
- **`CRScope/*` added**: replaces any existing scope label; updates `u_crscope`.
- **Closing an issue** below Review state: issue is reopened and commented.

## 5. Field mapping on create

| change_request | source |
|---|---|
| `short_description` | issue title, with `[…NCR…]`, `[…L…]`, `[…I…]` stripped |
| `description` | HTML card: rendered issue body + creator avatar and link |
| `u_git_reference` | `issue.html_url` |
| `impact`, `u_likelihood` | from labels (§2) |
| `u_labels` | resolved label set, comma-joined |
| `u_crscope` | `Application` / `Infrastructure` |
| `type` | `normal` / `standard` / `emergency` |
| `assignment_group` | `getAssigment(repository.name)` — see §7 |

`processContent()` converts a subset of Markdown to HTML: headings (4 levels),
blockquote, bold, italic, images, links, newlines, and `- [x]` / `- [ ]` to
green ✓ / red ✗.

## 6. The five outbound calls

The entire GitHub API surface this integration uses. Owner, repo and number are
parsed from the issue URL with `/([^\/]+)\/([^\/]+)\/issues\/(\d+)$/`.

| Action | Inputs | GitHub equivalent |
|---|---|---|
| `global.create_comment_on_issue` | owner, repository, issue_number, body | `POST /repos/{o}/{r}/issues/{n}/comments` |
| `global.get_comments_on_issue` | owner, repository, issue_number | `GET  …/comments` |
| `global.delete_a_label_on_issue` | owner, repository, issue_number, label | `DELETE …/labels/{label}` |
| `global.update_state_of_issue` | owner, repository, issue_number, state | `PATCH …/issues/{n}` |
| `global.update_issue_label` | owner, **repository2**, issue_number, labels | `PUT  …/labels` |

Five plain REST calls. A Go client for this is small.

## 7. Things that are broken today

Decide per item whether to reproduce or fix. None of these are guesses — they
are visible in the source.

| Where | Problem | Effect |
|---|---|---|
| `validate()` | `.spit(",")` instead of `.split(",")` | If `git.valid.org.list` is **set**, this throws, the outer catch swallows it, and the whole integration silently no-ops. It only works while that property is empty. |
| `handleCommand()` | entire body commented out | `[CMD::…]` comments are neither executed nor synced — silently swallowed |
| comment regex | `CMD::…Reviwed` | the correctly spelled `[CMD::Reviewed]` never matches |
| `forEach` backfill | no `thisArg`; `this` is the global object | `fillCommentInformation_d` is undefined → throws |
| `fillCommentInformation_d` | writes `data.comment.*` on a raw comment object | would throw even if reached |
| after `cr_new.insert()` | `JSON.strigify` | CR is created, then the branch throws |
| `getAssigment` default | returns the string `"NULL"` | written into `assignment_group` as an invalid sys_id |
| `u_crscope` casing | `validate` writes lowercase, the labeled branch writes capitalised | inconsistent values in the same column |
| title regexes | `[.*L.*]`, `[.*I.*]` | strip any bracketed text containing L or I |

## 8. Repo → assignment group

Hardcoded sys_ids, five entries, three of them test repos:

| repo | group |
|---|---|
| `choreo` | Choreo DevOps |
| `asgardeo-product` | **WSO2 Cloud DevOps** (not Asgardeo — probably a bug) |
| `sn-test-repo` | Choreo DevOps |
| `cr-test` | Asgardeo DevOps |
| `internal-project-test` | Bijira DevOps |

`Devant_DevOps` is declared and never used. This needs to become a real table.

## 9. Open questions

1. **Is it running?** `git.valid.org.list` set → integration is dead (§7). Needs
   checking before any of this is worth porting.
2. **How much is it used?** `SELECT count(*) FROM change_request WHERE
   u_git_reference IS NOT EMPTY`, and how recently.
3. **Do we want `[CMD::…]` back?** It has been dead long enough that nobody may
   miss it.
4. **Which repos really?** Two non-test entries today.

**RESOLVED (SRE, 2026-09-18): the mapping is product-level.** All Choreo change
requests go to a single Choreo repo; a customer does not get its own. So the
mapping keys off `product_id` and needs no `project_id`, and the inbound lookup
(repository name -> product -> team) is unambiguous:

```sql
CREATE TABLE product_github_repo (
    id         UUID PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    owner      VARCHAR(100) NOT NULL,
    repository VARCHAR(200) NOT NULL,
    team_id    UUID REFERENCES team(id),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    CONSTRAINT uq_product_github_repo UNIQUE (owner, repository)
);
```

This replaces all three of ServiceNow's mechanisms at once: `getAssigment`'s
hardcoded sys_ids, the `scripted-wum.*-repo-name` properties, and whatever
`CaseGithubIssuesCreateAPI` derives internally.
5. **Is GitHub still the source of truth for `description`?** Today every issue
   edit overwrites it, discarding anything typed in ServiceNow.

## 10. Notes for the port

- **Idempotency is mandatory here**, unlike the CR notification flows. A replayed
  webhook must not create a second change request. GitHub delivery IDs
  (`X-GitHub-Delivery`) give a natural dedupe key.
- **Loop prevention by identity**, not by string-matching the body: drop events
  whose sender is our integration account. The case webhook's approach (looking
  for `"ServiceNow Case:"` in the text) is strictly worse.
- **The state guards are the valuable part** — refusing to close below Review,
  reverting labels on a closed CR, and explaining why in a comment. That is real
  domain logic and should survive the port.
- **HMAC verification must be constant-time and must not echo the expected
  signature.** See the `Authenticator` finding raised separately.

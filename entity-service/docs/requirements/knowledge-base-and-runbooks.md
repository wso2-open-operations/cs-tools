# Knowledge base and runbooks — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 8.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

---

## 1. Purpose

Two collections of guidance that CS engineers keep and use:

- The **knowledge base**: articles written, reviewed and published for answering customers,
  grouped into named knowledge bases.
- **Runbooks**: a catalogue that links a monitoring alert's name to the page that says what to do
  when it fires.

The service stores both, keeps an article's life cycle in order, and tells other services when
an article is published. The portals draw them.

## 2. Who is involved

| Who | What they do |
|---|---|
| Article author | Writes an article and sends it for review. |
| Reviewer | Publishes the article, or sends it back with a comment. |
| Knowledge-base manager | A person or group named as looking after a knowledge base (section 5). |
| CS engineer | Reads and searches articles; maintains runbooks. |
| The notification service | Receives the "article published" event to make the article searchable by meaning. |

## 3. Articles

### Life cycle

An article is always in one of four states: **draft**, **pending review**, **published** or
**retired**.

**CUS-501** A new article shall always start as a **draft**, whatever the request says. It needs a
knowledge base, a title, a body and an author; the author is recorded as its creator.
*Check:* create an article; it is a draft, and a missing title, body, knowledge base or author is
refused.

**CUS-502** The only moves allowed are:

| From | To |
|---|---|
| Draft | Pending review |
| Pending review | Draft (sent back), or Published |
| Published | Draft (reopened), or Retired |

Retired is final. Any other move is refused with the two states named. A request for the state
the article is already in is not refused.

**CUS-503** A rejection comment is kept **only** when a pending-review article is sent back to
draft. In every other move the comment is dropped.

**CUS-504** Two people changing the state at the same moment shall not both succeed: if the
article's state changed since it was read, the second change is refused as a conflict.

**CUS-505** Publishing shall record when the article was published; retiring shall record when it
was retired.

**CUS-506** Every change of state shall add an entry to the article's **history**: the title, body
and state at that moment and who changed it. The history is returned newest first.
*Check:* send an article for review, then publish it; the history has two entries, newest first.

### Editing and removing

**CUS-510** The service shall let a person change an article's title and body **only while it is a
draft**. The title is required. For an article in any other state, or one that does not exist,
the answer is "not found, or not in draft state".

**CUS-511** The service shall let a person delete an article **only while it is a draft or pending
review**. A published or retired article cannot be deleted.

### Finding articles

**CUS-520** The service shall let a person open one article by id; an unknown id is "not found".

**CUS-521** The service shall let a person search articles by knowledge base, by state, by author,
and by text, in pages, newest first, returning the total and whether there are more. Only the
**latest version** of each article is searched. The text matches the **title** only, ignoring
letter case, and wildcard characters in it are taken literally. The text may be at most 200
characters.

**CUS-522** The page size follows the service-wide rule: a limit above the maximum is refused.

### Publishing event

**CUS-530** When an article moves **into** published from any other state, the service shall
publish an "article published" event carrying the article's id, so the notification service can
index it for search by meaning. An article that is published again without having left that state
publishes nothing. A failure to publish is logged and never fails or undoes the change.

## 4. Knowledge bases

**CUS-540** The service shall list all knowledge bases, alphabetical by title.

**CUS-541** The service shall let a person create a knowledge base with a name; it starts **active**.
A blank name is refused.

**CUS-542** The service shall let a person rename a knowledge base (the name is required) and switch
it active or inactive.

**CUS-543** Switching a knowledge base inactive does **not** stop articles being added to it or
found in search. (**Confirm**, see 8.2)

## 5. Knowledge-base managers

**CUS-550** The service shall let a person name users, and groups, as **managers** of a knowledge
base, remove them, and search who the managers are (by knowledge base, or by user or group).
Results are oldest first.

**CUS-551** Naming someone needs the knowledge base and the user (or group). An unknown knowledge
base is refused, and so is naming the same person or group twice for the same knowledge base.

**CUS-552** The manager lists are **records only**: the service does not check them when anyone
creates, edits, publishes or deletes an article. (**Confirm**, see 8.1)

## 6. Runbooks

A runbook is a numbered catalogue entry: a name, an optional link, and the **alert names** it
covers. The CSM portal uses the catalogue to show the right guidance for an alert.

**CUS-560** Runbooks are for **internal** callers only. The service shall list all runbooks, and
create, change and delete one.

**CUS-561** Each runbook shall get the next number in its own series automatically (RB plus seven
digits).

**CUS-562** A runbook needs a name (at most 1,000 characters) and at least one alert name (each at
most 1,000 characters, at most 500 alerts). Blank and repeated alert names (ignoring letter case)
are dropped. The link, if given, must be a full web address (http or https) of at most 2,000
characters.

**CUS-563** An alert name belongs to **one** runbook only, ignoring letter case and surrounding
blanks. Saving a runbook with an alert that another runbook already holds is refused as a
conflict naming that runbook. Two runbooks may not share a name, ignoring letter case and
surrounding blanks.

**CUS-564** Changing or deleting a runbook that does not exist is "not found".

### Importing

**CUS-570** The service shall let a person import up to **5,000** runbooks at once. An import with
none is refused.

**CUS-571** Within one file, a repeated runbook name is refused, and where the same alert appears
under several entries the **later** entry takes it. An entry left with no alerts at all is refused
with its position in the file. A name is optional in an import.

**CUS-572** For each entry, the runbook that already holds its **first listed alert** is updated
(its link and, if given, its name); an entry none of whose alerts is known creates a new runbook,
named after its first alert if no name was given. The entry's alerts then become that runbook's
alerts, wherever they were listed before.

**CUS-573** A runbook whose alerts have **all** been moved to other entries is **removed** by the
import. The answer says how many were created and updated and lists each removed runbook by number,
name and link, so nothing disappears unnoticed.

**CUS-574** An import is all or nothing, and two imports or an import and an edit at the same time
are handled one after the other.

## 7. What this covers

Which article or runbook an incident is shown with, and the handoff to a specialist team when no
runbook fits, belong to incidents (see the incident requirements).

## 8. Open questions

1. **The knowledge base has no access control in this service.** Creating, editing, publishing,
   retiring and deleting articles, and managing knowledge bases and their managers, are open to
   any caller who reaches the service. Who may do what is not checked here, and the manager lists
   are never consulted. Is this meant to be enforced in the portal alone?
2. **An inactive knowledge base is still writable and searchable.** An earlier database rule that
   refused new articles under an inactive knowledge base is no longer in place. Should it return?
3. **Author and editor identity is partly taken on trust.** The author of a new article and the
   person recorded for a state change are supplied by the caller, and knowledge bases and manager
   records are saved as made by "system". The signed-in person is not used.
4. **A search filter for teams is accepted and ignored.** The article search takes a list of team
   keys but does not apply it.
5. **Content edits are not in the history.** Only state changes are recorded; a draft edited ten
   times has no trail of the edits, and article creation itself is not an entry.
6. **Runbook links are not checked** beyond being a web address, and a runbook with no link is
   allowed.

## 9. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Article life cycle, search | `internal/service/kb_article_service.go`, `internal/repository/kb_article_repo.go` |
| Knowledge bases | `internal/service/knowledge_base_service.go`, `internal/repository/knowledge_base_repo.go` |
| Managers | `internal/service/kb_manager_service.go`, `internal/repository/kb_manager_repo.go` |
| Published event | `internal/events/events.go` (`kb.article_published`) |
| Runbooks | `internal/service/runbook_service.go`, `internal/repository/runbook_repo.go`, `internal/domain/runbook.go` |
| Routes | `/kb-articles` (create, get, search, patch, state, delete, history), `/knowledge-bases`, `/kb-manager-users`, `/kb-manager-groups`, `/runbooks` (list, create, import, update, delete) |
| Tables | migrations `0044`, `0203` (knowledge); `0241`, `0243`, `0244` (runbooks) |
| Tests | `runbook_service_test.go` |

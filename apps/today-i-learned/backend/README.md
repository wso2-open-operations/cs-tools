# til-backend

The backend for "Today I Learned": a company-wide feed of learnings from
customers, partners and internal work. The One WSO2 webapp is its only
client for sharing and editing entries; any signed-in employee can read them.

## Run locally

Needs Go 1.26 and a MySQL 8 server.

```bash
# Once per database: create it, then its tables.
mysql -h 127.0.0.1 -u root -p -e "CREATE DATABASE IF NOT EXISTS til CHARACTER SET utf8mb4"
mysql -h 127.0.0.1 -u root -p til < sql/create_til_tables.sql

cp .env.example .env   # fill in the database, auth and webapp settings
set -a; . ./.env; set +a
PORT=8078 go run ./cmd/server
```

The service reads the caller's token only from `x-jwt-assertion`, which the
API gateway sets in a deployment. Locally, put the repo's gateway shim in
front of it on the port the webapp calls. The shim turns the webapp's
`Authorization: Bearer` header into `x-jwt-assertion`:

```bash
cd ../../../scripts/csm-compose/gateway-shim
PORT=8077 UPSTREAM_URL=http://localhost:8078 go run .
```

The service checks its settings and the database at startup, and stops with
a message naming what's wrong: a missing setting, an unreachable database,
or a missing `til_entry` table or column.

## Test

```bash
make test
```

This runs `go vet` and every test with the race detector. The store tests
run against MySQL only when `TIL_TEST_DATABASE_DSN` is set, e.g.
`root:root@tcp(127.0.0.1:3306)/`. They create and use a `til_test` database
there and never touch any other.

## Configuration

Every setting is an environment variable; `.env.example` lists and explains
them all.

| Setting | Required | |
|---|---|---|
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` | yes (port defaults to 3306) | The MySQL database. |
| `DB_TLS` | no | For a database that requires secure transport. `true` encrypts the connection and checks the server's certificate, which must come from a CA the system trusts and name `DB_HOST`. `skip-verify` encrypts it without checking the certificate: use it only when the certificate can't be verified. Default `false`. |
| `AUTH_JWKS_ENDPOINT`, `AUTH_ISSUER` | yes | Where the token's signing keys are published, and its issuer. |
| `AUTH_AUDIENCE` | no | Client ids a token must be issued to. |
| `AUTH_MODERATOR_GROUPS` | no | Groups whose members may edit and delete anyone's entries. |
| `ONE_WSO2_WEBAPP_CLIENT_ID` | no | The webapp's client id. Only a token issued to it may share or edit; unset means no one can. |
| `ENTRY_TEXT_MAX_LENGTH` | no | The most characters of text a learning may have, markup not counted: 1–5000, default 500. The webapp's form reads it from `GET /settings` and stops there; the backend accepts 20 more, so a tiny counting difference never rejects what the form allowed. |
| `ONE_WSO2_BASE_URL` | no | The webapp's address, for the "Read more" link in Google Chat and the TIL logo it serves at `/til-logo.png` (shown only over https). |
| `CORS_ALLOWED_ORIGINS` | no | Browser origins allowed to call the API. |
| `PORT`, `REST_READ_TIMEOUT`, `REST_WRITE_TIMEOUT` | no | Defaults 9090, 15s, 45s. |
| `NOVERA_*` | no | Announcing new entries in Google Chat through Novera's `POST /internal/notifications`: all four or none. |

## API

`openapi.yaml` describes every operation. Errors are `{"message": "..."}`,
written to be shown to people as they are.

| Method | Path | |
|---|---|---|
| GET | `/health` | Liveness. No token needed. |
| GET | `/user-info` | `{email, displayName, canModerate}` |
| GET | `/settings` | `{whatMaxLength, notificationsEnabled}`: the rules the webapp's form shows, and whether entries are announced. |
| GET | `/submissions` | `{items, total, limit, offset}`, newest first. Query: `q` (search), `createdFrom` (inclusive) / `createdBefore` (exclusive) as RFC 3339 times, `author` (the author's name: every word starts a word in it), `source` (Customer, Partner, Internal or Other), `mine=true`, `limit` (1–100, default 10), `offset`. |
| POST | `/submissions` | `{title, where, what, whereDetail?}`. Webapp only. Announced in Google Chat. A `who` field is accepted and ignored. |
| GET | `/submissions/{id}` | One entry. |
| PUT | `/submissions/{id}` | `{title, where, what, whereDetail?}`. Webapp only; the author or a moderator. |
| DELETE | `/submissions/{id}` | The author or a moderator. |
| POST | `/submissions/{id}/notify` | Sends an entry whose announcement failed. Webapp only; moderators only; once (409 after). |

## How it works

- **Identity.** Each token's signature, issuer and expiry are checked against
  the published keys. `submittedByEmail` always comes from the token, never
  the request, and so does a new entry's byline, so no entry is anonymous and
  no one can post as someone else.
- **Webapp only.** Sharing and editing need a token issued to the webapp
  (`ONE_WSO2_WEBAPP_CLIENT_ID`). An employee's token from another
  application can read and delete, but not post.
- **Authors and moderators.** Only an entry's author (by verified email) or
  a moderator can edit or delete it. An edit never changes the byline, and a
  save that changes nothing doesn't mark the entry as edited.
- **Audit log.** Every share, edit, delete and resend logs one line: `entry
  created`, `entry updated`, `entry deleted` or `entry sent to Google Chat`,
  with the entry's `entryID` and the caller's `userID` (their token's `sub`).
  An edit or delete also logs `byAuthor`, which is false when a moderator
  changed someone else's entry. Nothing else about the entry is logged. A
  delete can't be undone, so its line is the only record of it.
- **Sanitising.** The learning is HTML from a rich-text editor. It is
  sanitised on every write to the editor's own formatting (paragraphs, line
  breaks, bold, italic, underline, lists, links), whatever the client sent.
  Entries are text only.
- **Search.** Each entry's plain text is stored beside its HTML. A
  full-text index over the title, that text, the author and the customer or
  partner name serves search, so markup never matches. Every word has to
  match, and a word matches the start of a word. Words the index can't hold
  (under three characters, or common words such as "the") are matched as
  text.
- **Announcements.** A new entry is sent to Novera, which posts it to every
  employee who has a chat open with it. A failure never fails the request:
  it's logged, and the entry's `notifiedAt` stays null. A moderator can then
  send it from the webapp (`POST /submissions/{id}/notify`). An entry is
  claimed before it's sent and released if Novera doesn't accept it, so no
  one gets the same card twice, even when two moderators press send at once.
  Only if the service is killed mid-send, or the database fails as the claim
  is released (logged as `release notification failed` with the entry's id),
  does an unsent entry stay marked as sent. To let a moderator send it:
  `UPDATE til_entry SET notified_at = NULL WHERE id = '<entry id>';`

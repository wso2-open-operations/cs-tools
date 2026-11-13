# TIL Backend

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../../LICENSE)
[![GitHub last commit](https://img.shields.io/github/last-commit/wso2-open-operations/cs-tools/main?path=apps%2Ftoday-i-learned)](https://github.com/wso2-open-operations/cs-tools/commits/main/?path=apps/today-i-learned)
[![GitHub issues](https://img.shields.io/github/issues/wso2-open-operations/cs-tools.svg)](https://github.com/wso2-open-operations/cs-tools/issues)

TIL Backend is the Go service behind "Today I Learned" — a company-wide
feed of learnings sourced from customers, partners, and internal work. The
One WSO2 webapp's Knowledge feed is where entries are shared, read and
edited.

## Why TIL?

Knowledge shared one-on-one in a customer call, a partner conversation, or a
hallway chat tends to stay with the person who learned it. TIL gives it a
place to land — a short, moderated, searchable feed anyone in the company can
post to and read, without turning it into a wiki page or a Slack thread no
one can find again.

## Features

- **Lightweight submissions** — a title, a source and what was learned,
  sanitized server-side, with no anonymous entries (the author is always the
  verified, signed-in user).
- **Search** — across titles, learnings, authors and customer names, with
  pages and a total count.
- **Moderation** — authors edit and delete their own entries; members of a
  configured Asgardeo group can edit and delete any.
- **Chat notifications** — each new entry is announced through Novera, the
  Google Chat assistant, to every employee who has a chat open with it.
- **Verified tokens** — every request's token is checked against its
  issuer's published signing keys.

## Project Structure

```bash
.
└── backend                  # Go service — entries, search, moderation
    ├── cmd/server/           # Entry point: configuration, routes, server
    ├── internal/             # Handlers, middleware, storage, entry rules, outbound clients
    ├── sql/                  # Creates the tables; run once before the service starts
    └── README.md             # Detailed backend documentation
```

There is no separate `webapp/` here — the UI is the One WSO2 webapp's
Knowledge feed, which lives in the One WSO2 webapp's own repository.

## Setup

See the [backend README](./backend/README.md) for local run instructions,
the full API surface, and the environment variables required for a real
deployment.

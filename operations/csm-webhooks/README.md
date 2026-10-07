# CSM Webhooks

Inbound webhooks from third-party systems, **one folder per source**. Each
folder is its own Choreo component (its own `.choreo/component.yaml`, Go
module, public endpoint and secret):

| Folder | Source | Choreo component directory |
|---|---|---|
| [`github/`](github/) | GitHub issue and issue-comment webhooks | `operations/csm-webhooks/github` |

## Adding a source

Create a sibling folder, `operations/csm-webhooks/<source>/`, on the same terms
as `github/`:

- **Verify and forward, nothing else.** Check the source's signature over the
  raw body, then forward to entity-service as an internal client. No database
  credentials, no decisions about what a payload means -- that belongs behind
  the Organization boundary in entity-service.
- **Public on purpose.** These exist because entity-service is
  Organization-visible and must stay that way.
- **One directory under `cmd/`.** A second `package main` breaks the Choreo Go
  buildpack.
- Its client id goes in entity-service's `M2M_CLIENT_IDS`.

See [`github/CLAUDE.md`](github/CLAUDE.md) for the reference implementation.

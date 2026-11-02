# azblobstore

The small slice of Azure Blob Storage the portal backends use for attachments.
Files are written as numbered blocks and become visible in one commit, and are
read back whole or by byte range.

Used by `apps/customer-portal/backend-v2` and `apps/csm-portal/backend`.
Entity-service does not use it: it decides who may touch which key, and never
talks to Azure.

## How an upload uses it

```go
store, _ := azblobstore.New(azblobstore.Config{
    AccountName: "...", AccountKey: "...", Container: "attachments",
})

// One call per chunk the browser sends (index 0, 1, 2, ...).
store.PutBlock(ctx, key, index, chunkBody, chunkSize)

// After a break: which chunks are already stored?
blocks, _ := store.UncommittedBlocks(ctx, key)

// After the last chunk: the blob appears now, and not before.
store.Commit(ctx, key, blockCount, "application/pdf")
```

Blocks are invisible until `Commit`, and Azure deletes uncommitted blocks after
seven days, so an upload that is never finished leaves nothing in the container.
`Commit` fails with `ErrMissingBlocks` if a block was never put.

## Reading and deleting

```go
obj, err := store.Open(ctx, key, nil)                         // whole blob
obj, err := store.Open(ctx, key, &azblobstore.Range{Offset: 0, Length: 16 << 20})
defer obj.Body.Close()                                         // obj.Partial, obj.ContentRange for a range

info, err := store.Stat(ctx, key)                              // ErrNotFound if absent
err = store.Delete(ctx, key)                                   // already gone is not an error
```

Keys are validated: empty, absolute, `.`/`..` folders and backslashes are
refused with `ErrInvalidKey`. That is a second line of defence. The access
check belongs to entity-service.

## Limits to know

- `PutBlock` reads the whole chunk into memory before sending it, so memory use
  is one chunk per request. `MaxBlockSize` is 100 MiB; the portals send 16 MiB.
- A blob has at most `MaxBlocks` (50,000) blocks.
- Every block id in a blob must be the same length; the package handles this.

## Tests

Unit tests run anywhere. The integration tests need a blob service and are
skipped unless `AZBLOBSTORE_TEST_ENDPOINT` is set. With the Azurite emulator:

```bash
docker run --rm -p 10000:10000 mcr.microsoft.com/azure-storage/azurite \
    azurite-blob --blobHost 0.0.0.0 --skipApiVersionCheck
AZBLOBSTORE_TEST_ENDPOINT=http://127.0.0.1:10000/devstoreaccount1 go test ./...
```

`AZBLOBSTORE_TEST_ACCOUNT` and `AZBLOBSTORE_TEST_KEY` override Azurite's
well-known development account to run the same tests against a real one.

## Using it from a backend

The module is imported by version, not by path, so Choreo can build a backend
from its own folder:

```
require github.com/wso2-open-operations/cs-tools/libs/azblobstore v0.1.0
```

A maintainer publishes a version by tagging a commit `libs/azblobstore/vX.Y.Z`.
For local work across the repo, use an uncommitted `go.work` that lists the
backend and this module.

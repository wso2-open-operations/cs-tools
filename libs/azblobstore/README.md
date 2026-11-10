# azblobstore

The small slice of Azure Blob Storage the portal backends use for attachments.
Files are written as numbered blocks and become visible in one commit, and are
read back whole or by byte range.

Used by `apps/customer-portal/backend-v2` and `apps/csm-portal/backend`.
Entity-service does not use it: it decides who may touch which key, and never
talks to Azure.

## How an upload uses it

```go
// Recommended: sign in as an Entra app that holds "Storage Blob Data Contributor"
// on the container only (and turn off shared key access on the account).
store, _ := azblobstore.New(azblobstore.Config{
    AccountName: "...", Container: "attachments",
    TenantID: "...", ClientID: "...", ClientSecret: "...",
})
// Or, for development: the account key (AccountName + AccountKey) or a
// connection string (Azurite).

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

The whole blob:

```go
obj, err := store.Open(ctx, key, nil)
if err != nil { /* ErrNotFound, ... */ }
defer obj.Body.Close()
```

Or one range of it, as a separate read (Length 0 reads to the end). A range read
sets `obj.Partial` and `obj.ContentRange`; a range past the end of the blob is
`ErrRangeNotSatisfiable`:

```go
obj, err := store.Open(ctx, key, &azblobstore.Range{Offset: 0, Length: 8 << 20})
if err != nil { /* ErrNotFound, ErrRangeNotSatisfiable, ... */ }
defer obj.Body.Close()
```

Size and type without reading the body, and delete:

```go
info, err := store.Stat(ctx, key)   // ErrNotFound if absent
err = store.Delete(ctx, key)        // already gone is not an error
```

A missing container is an error, not `ErrNotFound`: only a missing blob is.

Keys are validated: empty, absolute, `.`/`..` folders and backslashes are
refused with `ErrInvalidKey`. That is a second line of defence. The access
check belongs to entity-service.

## Direct links for large files

A store that signs in with an account key (or a connection string that carries
one) can make short-lived links, so a client sends or reads the bytes itself
instead of passing them through the service:

```go
if store.CanSignLinks() {
    // One PUT URL per block index; the client sends block i to links[i].
    links, expires, err := store.UploadLinks(key, blocks, 30*time.Minute)
    // ... then, after the client has sent every block:
    err = store.Commit(ctx, key, blocks, contentType)

    // A GET URL that answers as a download of fileName.
    link, expires, err := store.DownloadLink(key, fileName, contentType, 10*time.Minute)
}
```

A link is signed for one blob, for writing only (upload) or reading only
(download). Azure checks it when a request starts, so a request that started
before the expiry finishes after it. A link cannot limit how much is sent, so
check the real size with `Stat` after an upload. A store that signs in as an
Entra app cannot sign a link and returns `ErrLinksUnavailable`; the storage
account also needs CORS for the browsers that use the links.

## Limits to know

- `PutBlock` reads the whole chunk into memory before sending it, so memory use
  is one chunk per request. `MaxBlockSize` is 100 MiB; the portals send 8 MiB.
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

For local work across the repo, use an uncommitted `go.work` that lists the
backend and this module. Never commit `go.work`.

## Releasing a new version

A service gets the library from a version tag, not from the folder, so a change
here reaches it only after the steps below. Entity-service is the only user
of this module and must be moved to the new version.

1. **Change and test the library** under `libs/azblobstore` (`go test ./...`;
   the Azurite integration tests need Azurite running).
2. **Merge the change to `main`.** Tag only a commit that is on `main`.
3. **Tag it** with the next version and push the tag to the shared repo:
   ```
   git tag libs/azblobstore/vX.Y.Z <commit-on-main>
   git push upstream libs/azblobstore/vX.Y.Z
   ```
   The tag name must start with `libs/azblobstore/`, because the module lives in
   a subfolder. Use a patch version for a fix, a minor version for something new
   that keeps old callers working, and a major version for a breaking change
   (a major version of 2 or more also changes the module path to
   `.../libs/azblobstore/v2`).
4. **Move entity-service to it.** In `entity-service`:
   ```
   go get github.com/wso2-open-operations/cs-tools/libs/azblobstore@vX.Y.Z
   go mod tidy
   ```
   Commit its `go.mod` and `go.sum`, and check that it builds and its tests pass.
5. **Never move or delete a published tag.** If a version is wrong, publish the
   next patch version. The first build after a new tag may be slower, while the
   Go module proxy fetches it.

A change to entity-service that needs new library code cannot be merged before the tag
exists, or its build fails. Merge and tag the library first, then entity-service.

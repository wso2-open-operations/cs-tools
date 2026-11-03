// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Package azblobstore is the small slice of Azure Blob Storage the portal
// backends use for attachments: files are written as numbered blocks and made
// visible in one commit, and read back whole or by byte range.
//
// A blob's blocks are invisible until Commit, and Azure drops uncommitted
// blocks after seven days, so an upload that is never finished leaves nothing
// behind in the container.
package azblobstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

// Config says which container to use and how to sign in to it. Set one of:
//
//   - TenantID, ClientID and ClientSecret with AccountName: sign in as a
//     Microsoft Entra app. This is the recommended way: give the app the
//     "Storage Blob Data Contributor" role on the container only, and turn off
//     shared key access on the account.
//   - AccountName and AccountKey: the account's shared key. Full control of the
//     whole account, so keep it for development and a dedicated account.
//   - ConnectionString: what the Azurite emulator hands out.
//
// If more than one is set, the connection string wins, then Entra, then the key.
type Config struct {
	// ConnectionString is an Azure storage connection string. It is what the
	// Azurite emulator hands out for local development.
	ConnectionString string
	// AccountName is the storage account. With AccountKey it signs requests with
	// the account's shared key; with the three Entra settings below it only
	// names the account.
	AccountName string
	AccountKey  string
	// TenantID, ClientID and ClientSecret sign in as a Microsoft Entra app
	// (client-credential flow). The secret is never logged or returned in an error.
	TenantID     string
	ClientID     string
	ClientSecret string
	// Endpoint overrides the blob service URL, for example
	// http://127.0.0.1:10000/devstoreaccount1 for Azurite. It is only used with
	// AccountName; empty means https://<AccountName>.blob.core.windows.net.
	Endpoint string
	// Container is the container every key lives in.
	Container string
}

// Store reads and writes blobs in one container. It is safe for concurrent use.
type Store struct {
	container *container.Client
}

// New builds a Store. It does not contact Azure; use EnsureContainer or the
// first call to find out whether the settings work.
func New(cfg Config) (*Store, error) {
	if strings.TrimSpace(cfg.Container) == "" {
		return nil, errors.New("azblobstore: container is required")
	}

	var (
		c   *container.Client
		err error
	)
	entra := cfg.TenantID != "" || cfg.ClientID != "" || cfg.ClientSecret != ""
	switch {
	case cfg.ConnectionString != "":
		c, err = container.NewClientFromConnectionString(cfg.ConnectionString, cfg.Container, nil)
	case entra:
		if cfg.TenantID == "" || cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.AccountName == "" {
			return nil, errors.New("azblobstore: signing in as an Entra app needs a tenant id, client id, client secret and account name")
		}
		cred, credErr := azidentity.NewClientSecretCredential(cfg.TenantID, cfg.ClientID, cfg.ClientSecret, nil)
		if credErr != nil {
			// The library's error can echo the inputs; say only what is wrong.
			return nil, errors.New("azblobstore: invalid Entra app settings (check the tenant id, client id and client secret)")
		}
		c, err = container.NewClient(containerURL(cfg), cred, nil)
	case cfg.AccountName != "" && cfg.AccountKey != "":
		cred, credErr := blob.NewSharedKeyCredential(cfg.AccountName, cfg.AccountKey)
		if credErr != nil {
			return nil, fmt.Errorf("azblobstore: invalid account key: %w", credErr)
		}
		c, err = container.NewClientWithSharedKeyCredential(containerURL(cfg), cred, nil)
	default:
		return nil, errors.New("azblobstore: set a connection string, an account name and key, or an Entra app (tenant id, client id, client secret) and account name")
	}
	if err != nil {
		return nil, fmt.Errorf("azblobstore: create client: %w", err)
	}
	return &Store{container: c}, nil
}

// containerURL is the container's address: the endpoint override or the
// account's standard blob URL, then the container name.
func containerURL(cfg Config) string {
	base := strings.TrimRight(cfg.Endpoint, "/")
	if base == "" {
		base = fmt.Sprintf("https://%s.blob.core.windows.net", cfg.AccountName)
	}
	return base + "/" + cfg.Container
}

// CheckContainer fails if the container cannot be reached with these settings.
// Unlike EnsureContainer it only reads, so it works with a role scoped to the
// container (an app with "Storage Blob Data Contributor" on one container cannot
// create containers). Use it at startup in production.
func (s *Store) CheckContainer(ctx context.Context) error {
	if _, err := s.container.GetProperties(ctx, nil); err != nil {
		return fmt.Errorf("azblobstore: container is not reachable: %w", err)
	}
	return nil
}

// EnsureContainer creates the container if it does not exist. Production
// containers are created by infrastructure; this is for Azurite and tests.
func (s *Store) EnsureContainer(ctx context.Context) error {
	_, err := s.container.Create(ctx, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return fmt.Errorf("azblobstore: create container: %w", err)
	}
	return nil
}

// PutBlock stores one block of the blob at key. Blocks are numbered from 0 and
// are committed in number order. It reads the block from body into memory, never
// more than size+1 bytes, and stages it only if body held exactly size bytes
// (size is the chunk's declared length, at most MaxBlockSize); any other length
// gives ErrBlockSize. An error from reading body, such as http.MaxBytesError from
// a size-limited request body, is returned wrapped so callers can match it with
// errors.As. Sending the same index again replaces that block, which makes a
// retry safe.
func (s *Store) PutBlock(ctx context.Context, key string, index int, body io.Reader, size int64) error {
	if err := validateKey(key); err != nil {
		return err
	}
	id, err := blockID(index)
	if err != nil {
		return err
	}
	if size <= 0 || size > MaxBlockSize {
		return fmt.Errorf("%w: %d bytes", ErrBlockSize, size)
	}

	// Read one byte past size so a longer body is noticed, and let the buffer
	// grow with what actually arrives: allocating size up front would let a
	// caller that declares a large chunk and sends little hold that memory.
	buf, err := io.ReadAll(io.LimitReader(body, size+1))
	if err != nil {
		return fmt.Errorf("azblobstore: read block %d of %s: %w", index, key, err)
	}
	if int64(len(buf)) != size {
		return fmt.Errorf("%w: body is %d bytes, declared %d", ErrBlockSize, len(buf), size)
	}

	_, err = s.container.NewBlockBlobClient(key).StageBlock(ctx, id, streaming.NopCloser(bytes.NewReader(buf)), nil)
	if err != nil {
		return fmt.Errorf("azblobstore: put block %d of %s: %w", index, key, err)
	}
	return nil
}

// Block is one staged block that has not been committed yet.
type Block struct {
	Index int
	Size  int64
}

// UncommittedBlocks lists the blocks staged for key, in index order. A blob with
// nothing staged gives an empty list, not an error. An upload resumes from the
// first index that is missing.
func (s *Store) UncommittedBlocks(ctx context.Context, key string) ([]Block, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	resp, err := s.container.NewBlockBlobClient(key).GetBlockList(ctx, blockblob.BlockListTypeUncommitted, nil)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("azblobstore: list blocks of %s: %w", key, err)
	}

	blocks := make([]Block, 0, len(resp.UncommittedBlocks))
	for _, b := range resp.UncommittedBlocks {
		if b == nil || b.Name == nil {
			continue
		}
		index, err := parseBlockID(*b.Name)
		if err != nil {
			// Not one of ours; it cannot be part of a commit we make.
			continue
		}
		var size int64
		if b.Size != nil {
			size = *b.Size
		}
		blocks = append(blocks, Block{Index: index, Size: size})
	}
	sortBlocks(blocks)
	return blocks, nil
}

// Commit makes blocks 0 to blockCount-1 the content of the blob at key. The blob
// appears, with contentType, only when this returns nil. A block that was never
// put makes it fail with ErrMissingBlocks and the blob does not appear.
func (s *Store) Commit(ctx context.Context, key string, blockCount int, contentType string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if blockCount <= 0 || blockCount > MaxBlocks {
		return fmt.Errorf("%w: %d blocks", ErrBlockCount, blockCount)
	}
	ids := make([]string, blockCount)
	for i := range ids {
		id, err := blockID(i)
		if err != nil {
			return err
		}
		ids[i] = id
	}

	opts := &blockblob.CommitBlockListOptions{}
	if contentType != "" {
		opts.HTTPHeaders = &blob.HTTPHeaders{BlobContentType: &contentType}
	}
	if _, err := s.container.NewBlockBlobClient(key).CommitBlockList(ctx, ids, opts); err != nil {
		if bloberror.HasCode(err, bloberror.InvalidBlobOrBlock, bloberror.InvalidBlockList) {
			return fmt.Errorf("%w: %s", ErrMissingBlocks, key)
		}
		return fmt.Errorf("azblobstore: commit %s: %w", key, err)
	}
	return nil
}

// Info describes a stored blob.
type Info struct {
	Size        int64
	ContentType string
}

// Stat returns the size and type of the committed blob at key, or ErrNotFound.
func (s *Store) Stat(ctx context.Context, key string) (Info, error) {
	if err := validateKey(key); err != nil {
		return Info{}, err
	}
	resp, err := s.container.NewBlobClient(key).GetProperties(ctx, nil)
	if err != nil {
		if isNotFound(err) {
			return Info{}, ErrNotFound
		}
		return Info{}, fmt.Errorf("azblobstore: stat %s: %w", key, err)
	}
	info := Info{}
	if resp.ContentLength != nil {
		info.Size = *resp.ContentLength
	}
	if resp.ContentType != nil {
		info.ContentType = *resp.ContentType
	}
	return info, nil
}

// Range selects part of a blob. Length 0 means to the end.
type Range struct {
	Offset int64
	Length int64
}

// Object is a blob being read. The caller closes Body.
type Object struct {
	Body io.ReadCloser
	// Info is the whole blob's size and type, even for a partial read.
	Info Info
	// ContentLength is how many bytes Body will deliver.
	ContentLength int64
	// Partial is true when Body holds only the requested Range.
	Partial bool
	// ContentRange is the response's Content-Range value for a partial read.
	ContentRange string
}

// Open reads the blob at key, all of it when rng is nil. A range that starts
// past the end of the blob gives ErrRangeNotSatisfiable.
func (s *Store) Open(ctx context.Context, key string, rng *Range) (*Object, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	opts := &blob.DownloadStreamOptions{}
	if rng != nil {
		if rng.Offset < 0 || rng.Length < 0 {
			return nil, fmt.Errorf("%w: offset %d, length %d", ErrRangeNotSatisfiable, rng.Offset, rng.Length)
		}
		opts.Range = blob.HTTPRange{Offset: rng.Offset, Count: rng.Length}
	}

	resp, err := s.container.NewBlobClient(key).DownloadStream(ctx, opts)
	if err != nil {
		switch {
		case isNotFound(err):
			return nil, ErrNotFound
		case isStatus(err, http.StatusRequestedRangeNotSatisfiable):
			return nil, ErrRangeNotSatisfiable
		}
		return nil, fmt.Errorf("azblobstore: open %s: %w", key, err)
	}

	obj := &Object{Body: resp.Body}
	if resp.ContentLength != nil {
		obj.ContentLength = *resp.ContentLength
	}
	if resp.ContentType != nil {
		obj.Info.ContentType = *resp.ContentType
	}
	obj.Info.Size = obj.ContentLength
	if rng != nil && resp.ContentRange != nil {
		obj.Partial = true
		obj.ContentRange = *resp.ContentRange
		if total, ok := totalFromContentRange(obj.ContentRange); ok {
			obj.Info.Size = total
		}
	}
	return obj, nil
}

// Delete removes the blob at key. A blob that is already gone is not an error.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	_, err := s.container.NewBlobClient(key).Delete(ctx, nil)
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("azblobstore: delete %s: %w", key, err)
	}
	return nil
}

// isNotFound reports a missing blob. A missing container is not that: it means
// the store is misconfigured, and treating it as an absent blob would let Delete
// report success and UncommittedBlocks an empty list for a container that is not
// there.
func isNotFound(err error) bool {
	if bloberror.HasCode(err, bloberror.ContainerNotFound) {
		return false
	}
	return bloberror.HasCode(err, bloberror.BlobNotFound, bloberror.ResourceNotFound) ||
		isStatus(err, http.StatusNotFound)
}

func isStatus(err error, status int) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == status
}

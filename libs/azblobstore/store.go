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
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

// Config says which container to use and how to sign in to it. Set either
// ConnectionString, or AccountName and AccountKey.
type Config struct {
	// ConnectionString is an Azure storage connection string. It is what the
	// Azurite emulator hands out for local development.
	ConnectionString string
	// AccountName and AccountKey sign requests with the account's shared key.
	AccountName string
	AccountKey  string
	// Endpoint overrides the blob service URL, for example
	// http://127.0.0.1:10000/devstoreaccount1 for Azurite. It is only used with
	// AccountName and AccountKey; empty means https://<AccountName>.blob.core.windows.net.
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
	switch {
	case cfg.ConnectionString != "":
		c, err = container.NewClientFromConnectionString(cfg.ConnectionString, cfg.Container, nil)
	case cfg.AccountName != "" && cfg.AccountKey != "":
		cred, credErr := blob.NewSharedKeyCredential(cfg.AccountName, cfg.AccountKey)
		if credErr != nil {
			return nil, fmt.Errorf("azblobstore: invalid account key: %w", credErr)
		}
		base := strings.TrimRight(cfg.Endpoint, "/")
		if base == "" {
			base = fmt.Sprintf("https://%s.blob.core.windows.net", cfg.AccountName)
		}
		c, err = container.NewClientWithSharedKeyCredential(base+"/"+cfg.Container, cred, nil)
	default:
		return nil, errors.New("azblobstore: set a connection string, or an account name and key")
	}
	if err != nil {
		return nil, fmt.Errorf("azblobstore: create client: %w", err)
	}
	return &Store{container: c}, nil
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
// are committed in number order. It reads exactly size bytes from body (at most
// MaxBlockSize) into memory first, so a caller streaming a request should pass
// an http.MaxBytesReader-limited body, and size must be the chunk's length.
// Sending the same index again replaces that block, which makes a retry safe.
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

	buf := make([]byte, size)
	if _, err := io.ReadFull(body, buf); err != nil {
		return fmt.Errorf("%w: %v", ErrBlockSize, err)
	}
	// Reading one more byte must hit the end; a longer body is not the chunk
	// the caller declared.
	var extra [1]byte
	if n, _ := body.Read(extra[:]); n > 0 {
		return fmt.Errorf("%w: body is longer than %d bytes", ErrBlockSize, size)
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

func isNotFound(err error) bool {
	return bloberror.HasCode(err, bloberror.BlobNotFound, bloberror.ResourceNotFound) ||
		isStatus(err, http.StatusNotFound)
}

func isStatus(err error, status int) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == status
}

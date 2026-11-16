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

package azblobstore

import (
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Errors a Store returns. Compare with errors.Is.
var (
	// ErrNotFound: no committed blob exists at the key.
	ErrNotFound = errors.New("azblobstore: blob not found")
	// ErrInvalidKey: the key is empty, too long, or could escape its folder.
	ErrInvalidKey = errors.New("azblobstore: invalid key")
	// ErrBlockSize: a block is empty, larger than MaxBlockSize, or its body did
	// not match the size the caller declared.
	ErrBlockSize = errors.New("azblobstore: invalid block size")
	// ErrBlockIndex: a block index is outside 0 to MaxBlocks-1.
	ErrBlockIndex = errors.New("azblobstore: invalid block index")
	// ErrBlockCount: a commit names no blocks or more than MaxBlocks.
	ErrBlockCount = errors.New("azblobstore: invalid block count")
	// ErrMissingBlocks: a commit names a block that was never put.
	ErrMissingBlocks = errors.New("azblobstore: blocks are missing")
	// ErrRangeNotSatisfiable: the requested byte range is outside the blob.
	ErrRangeNotSatisfiable = errors.New("azblobstore: range not satisfiable")
)

const (
	// MaxBlocks is Azure's limit on blocks in one blob.
	MaxBlocks = 50000
	// MaxBlockSize is the largest block PutBlock accepts. Azure allows more, but
	// PutBlock holds the whole block in memory, so this keeps a request bounded.
	MaxBlockSize int64 = 100 << 20

	// maxKeyLength is Azure's limit on a blob name.
	maxKeyLength = 1024
	// blockIDWidth makes every block id the same length, which Azure requires
	// within one blob.
	blockIDWidth = 8
)

// blockID is the base64 id Azure wants for block index. Every id in a blob is
// the same length, so the index is zero-padded before encoding.
func blockID(index int) (string, error) {
	if index < 0 || index >= MaxBlocks {
		return "", fmt.Errorf("%w: %d", ErrBlockIndex, index)
	}
	raw := fmt.Sprintf("%0*d", blockIDWidth, index)
	return base64.StdEncoding.EncodeToString([]byte(raw)), nil
}

// parseBlockID is the inverse of blockID.
func parseBlockID(id string) (int, error) {
	raw, err := base64.StdEncoding.DecodeString(id)
	if err != nil || len(raw) != blockIDWidth {
		return 0, fmt.Errorf("%w: %q", ErrBlockIndex, id)
	}
	index, err := strconv.Atoi(string(raw))
	if err != nil || index < 0 || index >= MaxBlocks {
		return 0, fmt.Errorf("%w: %q", ErrBlockIndex, id)
	}
	return index, nil
}

func sortBlocks(blocks []Block) {
	slices.SortFunc(blocks, func(a, b Block) int { return a.Index - b.Index })
}

// validateKey refuses keys the callers should never build: empty, over Azure's
// length limit, absolute, or with a "." or ".." folder. Entity-service builds
// every key; this is a second line of defence, not the access check.
func validateKey(key string) error {
	if key == "" || len(key) > maxKeyLength || strings.HasPrefix(key, "/") || strings.ContainsRune(key, '\\') {
		return fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("%w: %q", ErrInvalidKey, key)
		}
	}
	return nil
}

// totalFromContentRange reads the blob's full size from "bytes 0-9/100".
func totalFromContentRange(v string) (int64, bool) {
	_, total, ok := strings.Cut(v, "/")
	if !ok || total == "*" {
		return 0, false
	}
	n, err := strconv.ParseInt(total, 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

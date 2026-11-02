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
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// These tests run against a real blob service, normally the Azurite emulator:
//
//	docker run --rm -p 10000:10000 mcr.microsoft.com/azure-storage/azurite \
//	    azurite-blob --blobHost 0.0.0.0 --skipApiVersionCheck
//	AZBLOBSTORE_TEST_ENDPOINT=http://127.0.0.1:10000/devstoreaccount1 go test ./...
//
// Without AZBLOBSTORE_TEST_ENDPOINT they are skipped. The account is Azurite's
// well-known development account unless AZBLOBSTORE_TEST_ACCOUNT and
// AZBLOBSTORE_TEST_KEY say otherwise.
const (
	azuriteAccount = "devstoreaccount1"
	azuriteKey     = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
)

func integrationStore(t *testing.T) *Store {
	t.Helper()
	endpoint := os.Getenv("AZBLOBSTORE_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("AZBLOBSTORE_TEST_ENDPOINT is not set; skipping tests that need a blob service")
	}
	account, key := azuriteAccount, azuriteKey
	if v := os.Getenv("AZBLOBSTORE_TEST_ACCOUNT"); v != "" {
		account = v
	}
	if v := os.Getenv("AZBLOBSTORE_TEST_KEY"); v != "" {
		key = v
	}

	s, err := New(Config{
		AccountName: account,
		AccountKey:  key,
		Endpoint:    endpoint,
		Container:   fmt.Sprintf("azblobstore-test-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.EnsureContainer(ctx); err != nil {
		t.Fatalf("EnsureContainer: %v", err)
	}
	// Creating it a second time must not fail.
	if err := s.EnsureContainer(ctx); err != nil {
		t.Fatalf("EnsureContainer again: %v", err)
	}
	t.Cleanup(func() {
		_, _ = s.container.Delete(context.Background(), nil)
	})
	return s
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStore_UploadInBlocksThenReadBack(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	key := "csm/projects/P1/workitems/42/attachments/7_report.bin"

	// Three blocks, the last one shorter, as a chunked upload would send them.
	data := randomBytes(t, 2*1024*1024+123)
	const blockSize = 1024 * 1024
	var blocks int
	for off := 0; off < len(data); off += blockSize {
		end := min(off+blockSize, len(data))
		if err := s.PutBlock(ctx, key, blocks, bytes.NewReader(data[off:end]), int64(end-off)); err != nil {
			t.Fatalf("PutBlock %d: %v", blocks, err)
		}
		blocks++
	}

	// Nothing is visible before the commit.
	if _, err := s.Stat(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat before commit = %v, want ErrNotFound", err)
	}
	if _, err := s.Open(ctx, key, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open before commit = %v, want ErrNotFound", err)
	}

	// A resuming client can see what is already staged.
	staged, err := s.UncommittedBlocks(ctx, key)
	if err != nil {
		t.Fatalf("UncommittedBlocks: %v", err)
	}
	if len(staged) != 3 || staged[0].Index != 0 || staged[2].Index != 2 || staged[2].Size != int64(len(data)-2*blockSize) {
		t.Fatalf("UncommittedBlocks = %+v", staged)
	}

	if err := s.Commit(ctx, key, blocks, "application/octet-stream"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	info, err := s.Stat(ctx, key)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size != int64(len(data)) || info.ContentType != "application/octet-stream" {
		t.Fatalf("Stat = %+v, want size %d", info, len(data))
	}

	obj, err := s.Open(ctx, key, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer obj.Body.Close()
	got, err := io.ReadAll(obj.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("downloaded bytes differ from the uploaded bytes")
	}
	if obj.Partial || obj.Info.Size != int64(len(data)) || obj.ContentLength != int64(len(data)) {
		t.Fatalf("full Open metadata = %+v", obj)
	}

	// After the commit nothing is left staged.
	if staged, _ := s.UncommittedBlocks(ctx, key); len(staged) != 0 {
		t.Fatalf("blocks still staged after commit: %+v", staged)
	}
}

func TestStore_ResumeSkipsStagedBlocks(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	key := "resume/file.bin"

	data := randomBytes(t, 3*100)
	// Upload blocks 0 and 1, "lose the connection", then resume at block 2.
	for i := 0; i < 2; i++ {
		if err := s.PutBlock(ctx, key, i, bytes.NewReader(data[i*100:(i+1)*100]), 100); err != nil {
			t.Fatalf("PutBlock %d: %v", i, err)
		}
	}
	staged, err := s.UncommittedBlocks(ctx, key)
	if err != nil || len(staged) != 2 {
		t.Fatalf("UncommittedBlocks = %+v, %v; want blocks 0 and 1", staged, err)
	}
	// Sending block 1 again is harmless.
	if err := s.PutBlock(ctx, key, 1, bytes.NewReader(data[100:200]), 100); err != nil {
		t.Fatalf("repeat PutBlock: %v", err)
	}
	if err := s.PutBlock(ctx, key, 2, bytes.NewReader(data[200:300]), 100); err != nil {
		t.Fatalf("PutBlock 2: %v", err)
	}
	if err := s.Commit(ctx, key, 3, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	obj, err := s.Open(ctx, key, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer obj.Body.Close()
	got, _ := io.ReadAll(obj.Body)
	if !bytes.Equal(got, data) {
		t.Fatal("resumed upload does not match the original bytes")
	}
}

func TestStore_CommitWithMissingBlockFails(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	key := "missing/file.bin"

	if err := s.PutBlock(ctx, key, 0, strings.NewReader("abc"), 3); err != nil {
		t.Fatalf("PutBlock: %v", err)
	}
	// Block 1 was never put.
	if err := s.Commit(ctx, key, 2, ""); !errors.Is(err, ErrMissingBlocks) {
		t.Fatalf("Commit = %v, want ErrMissingBlocks", err)
	}
	if _, err := s.Stat(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("blob exists after a failed commit: %v", err)
	}
}

func TestStore_RangeRead(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	key := "range/file.txt"

	data := []byte("0123456789abcdefghij")
	if err := s.PutBlock(ctx, key, 0, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ctx, key, 1, "text/plain"); err != nil {
		t.Fatal(err)
	}

	read := func(rng Range) (*Object, string) {
		t.Helper()
		obj, err := s.Open(ctx, key, &rng)
		if err != nil {
			t.Fatalf("Open %+v: %v", rng, err)
		}
		defer obj.Body.Close()
		b, _ := io.ReadAll(obj.Body)
		return obj, string(b)
	}

	obj, got := read(Range{Offset: 5, Length: 5})
	if got != "56789" || !obj.Partial || obj.ContentLength != 5 || obj.Info.Size != int64(len(data)) {
		t.Fatalf("middle range = %q %+v", got, obj)
	}
	if obj.ContentRange != "bytes 5-9/20" {
		t.Fatalf("Content-Range = %q", obj.ContentRange)
	}

	// Length 0 reads to the end.
	_, got = read(Range{Offset: 15})
	if got != "fghij" {
		t.Fatalf("open-ended range = %q", got)
	}

	if _, err := s.Open(ctx, key, &Range{Offset: 100, Length: 10}); !errors.Is(err, ErrRangeNotSatisfiable) {
		t.Fatalf("range past the end: %v, want ErrRangeNotSatisfiable", err)
	}
}

func TestStore_DeleteIsIdempotent(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	key := "delete/file.txt"

	if err := s.PutBlock(ctx, key, 0, strings.NewReader("x"), 1); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ctx, key, 1, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Stat(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat after delete = %v, want ErrNotFound", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("second Delete: %v", err)
	}
	if err := s.Delete(ctx, "never/existed.txt"); err != nil {
		t.Fatalf("Delete of a missing blob: %v", err)
	}
}

func TestStore_UncommittedBlocksOfUnknownBlobIsEmpty(t *testing.T) {
	s := integrationStore(t)
	blocks, err := s.UncommittedBlocks(context.Background(), "nothing/here.bin")
	if err != nil || len(blocks) != 0 {
		t.Fatalf("UncommittedBlocks = %+v, %v; want an empty list", blocks, err)
	}
}

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
	"errors"
	"strings"
	"testing"
)

func TestBlockID_RoundTripAndFixedLength(t *testing.T) {
	var length int
	for _, index := range []int{0, 1, 9, 10, 255, 49999} {
		id, err := blockID(index)
		if err != nil {
			t.Fatalf("blockID(%d): %v", index, err)
		}
		if length == 0 {
			length = len(id)
		}
		if len(id) != length {
			t.Fatalf("blockID(%d) has length %d, want %d: Azure needs equal-length ids", index, len(id), length)
		}
		got, err := parseBlockID(id)
		if err != nil || got != index {
			t.Fatalf("parseBlockID(blockID(%d)) = %d, %v", index, got, err)
		}
	}
}

func TestBlockID_RejectsOutOfRange(t *testing.T) {
	for _, index := range []int{-1, MaxBlocks, MaxBlocks + 1} {
		if _, err := blockID(index); !errors.Is(err, ErrBlockIndex) {
			t.Errorf("blockID(%d) error = %v, want ErrBlockIndex", index, err)
		}
	}
}

func TestParseBlockID_IgnoresForeignIDs(t *testing.T) {
	for _, id := range []string{"", "not base64!", "YQ==", "YWJjZGVmZ2hpag=="} {
		if _, err := parseBlockID(id); err == nil {
			t.Errorf("parseBlockID(%q) succeeded, want an error", id)
		}
	}
}

func TestValidateKey(t *testing.T) {
	good := []string{
		"csm/projects/P1/workitems/42/attachments/7_report.pdf",
		"a",
		"folder/file name with spaces.txt",
	}
	for _, key := range good {
		if err := validateKey(key); err != nil {
			t.Errorf("validateKey(%q) = %v, want nil", key, err)
		}
	}

	bad := []string{
		"",
		"/absolute/path",
		"a/../b",
		"../b",
		"a/./b",
		"a//b",
		"trailing/",
		`back\slash`,
		strings.Repeat("x", maxKeyLength+1),
	}
	for _, key := range bad {
		if err := validateKey(key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("validateKey(%q) = %v, want ErrInvalidKey", key, err)
		}
	}
}

func TestTotalFromContentRange(t *testing.T) {
	tests := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"bytes 0-9/100", 100, true},
		{"bytes 10-19/5368709120", 5368709120, true},
		{"bytes 0-9/*", 0, false},
		{"bytes 0-9", 0, false},
		{"bytes 0-9/abc", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, ok := totalFromContentRange(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("totalFromContentRange(%q) = %d, %v; want %d, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestNew_RequiresContainerAndCredentials(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"no container", Config{ConnectionString: "UseDevelopmentStorage=true"}},
		{"no credentials", Config{Container: "c"}},
		{"name without key", Config{Container: "c", AccountName: "acct"}},
		{"key without name", Config{Container: "c", AccountKey: "a2V5"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.cfg); err == nil {
				t.Fatal("New succeeded, want an error")
			}
		})
	}
}

func TestNew_AcceptsSharedKey(t *testing.T) {
	// The key only has to be valid base64 for the client to be built; nothing
	// contacts Azure here.
	_, err := New(Config{AccountName: "acct", AccountKey: "a2V5", Container: "c", Endpoint: "http://127.0.0.1:10000/acct/"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
}

// The argument checks run before any network call, so a Store that points
// nowhere is enough to test them.
func offlineStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(Config{AccountName: "acct", AccountKey: "a2V5", Container: "c", Endpoint: "http://127.0.0.1:1/acct"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestPutBlock_RejectsBadArguments(t *testing.T) {
	s := offlineStore(t)
	ctx := context.Background()

	if err := s.PutBlock(ctx, "a/b", -1, strings.NewReader("x"), 1); !errors.Is(err, ErrBlockIndex) {
		t.Errorf("negative index: %v, want ErrBlockIndex", err)
	}
	if err := s.PutBlock(ctx, "a/b", 0, strings.NewReader(""), 0); !errors.Is(err, ErrBlockSize) {
		t.Errorf("zero size: %v, want ErrBlockSize", err)
	}
	if err := s.PutBlock(ctx, "a/b", 0, strings.NewReader("x"), MaxBlockSize+1); !errors.Is(err, ErrBlockSize) {
		t.Errorf("oversize: %v, want ErrBlockSize", err)
	}
	if err := s.PutBlock(ctx, "a/../b", 0, strings.NewReader("x"), 1); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("bad key: %v, want ErrInvalidKey", err)
	}
	if err := s.PutBlock(ctx, "a/b", 0, strings.NewReader("short"), 10); !errors.Is(err, ErrBlockSize) {
		t.Errorf("short body: %v, want ErrBlockSize", err)
	}
	if err := s.PutBlock(ctx, "a/b", 0, bytes.NewReader([]byte("too long")), 3); !errors.Is(err, ErrBlockSize) {
		t.Errorf("long body: %v, want ErrBlockSize", err)
	}
}

func TestCommit_RejectsBadBlockCount(t *testing.T) {
	s := offlineStore(t)
	for _, n := range []int{0, -1, MaxBlocks + 1} {
		if err := s.Commit(context.Background(), "a/b", n, "text/plain"); !errors.Is(err, ErrBlockCount) {
			t.Errorf("Commit with %d blocks: %v, want ErrBlockCount", n, err)
		}
	}
}

func TestOpen_RejectsNegativeRange(t *testing.T) {
	s := offlineStore(t)
	if _, err := s.Open(context.Background(), "a/b", &Range{Offset: -1}); !errors.Is(err, ErrRangeNotSatisfiable) {
		t.Errorf("negative offset: %v, want ErrRangeNotSatisfiable", err)
	}
}

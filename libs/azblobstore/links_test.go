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
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

// A made-up key: the signature only needs it to be valid base64.
const testKey = "c2VjcmV0LWtleS1mb3ItdGVzdHM="

// keyedStore is a store that signs in with an account key, so it can sign links.
func keyedStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(Config{Container: "files", AccountName: "acct", AccountKey: testKey})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

// TestUploadLinks: one write-only link per block, for one blob, over https, with the block id in it.
func TestUploadLinks(t *testing.T) {
	s := keyedStore(t)
	links, expires, err := s.UploadLinks("projects/p1/a.pdf", 3, 30*time.Minute)
	if err != nil || len(links) != 3 {
		t.Fatalf("UploadLinks = %d links, %v", len(links), err)
	}
	if d := time.Until(expires); d < 29*time.Minute || d > 31*time.Minute {
		t.Errorf("expires in %v, want about 30m", d)
	}
	for i, l := range links {
		u, err := url.Parse(l)
		if err != nil {
			t.Fatalf("link %d: %v", i, err)
		}
		q := u.Query()
		if u.Host != "acct.blob.core.windows.net" || u.Path != "/files/projects/p1/a.pdf" {
			t.Errorf("link %d points at %s%s", i, u.Host, u.Path)
		}
		// Write only, https only, this one blob, and the block it is for.
		if q.Get("sp") != "w" || q.Get("spr") != "https" || q.Get("sr") != "b" || q.Get("sig") == "" {
			t.Errorf("link %d query = %v", i, q)
		}
		if q.Get("comp") != "block" {
			t.Errorf("link %d comp = %q", i, q.Get("comp"))
		}
		got, err := parseBlockID(q.Get("blockid"))
		if err != nil || got != i {
			t.Errorf("link %d block id = %q (%d, %v)", i, q.Get("blockid"), got, err)
		}
	}
}

// TestUploadLinks_Refusals: a bad count, key or lifetime makes no link.
func TestUploadLinks_Refusals(t *testing.T) {
	s := keyedStore(t)
	for name, tc := range map[string]struct {
		key   string
		count int
		ttl   time.Duration
		want  error
	}{
		"no blocks": {"a.pdf", 0, time.Minute, ErrBlockCount},
		"too many":  {"a.pdf", MaxBlocks + 1, time.Minute, ErrBlockCount},
		"bad key":   {"../a.pdf", 1, time.Minute, ErrInvalidKey},
		"empty key": {"", 1, time.Minute, ErrInvalidKey},
	} {
		if _, _, err := s.UploadLinks(tc.key, tc.count, tc.ttl); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if _, _, err := s.UploadLinks("a.pdf", 1, 0); err == nil {
		t.Error("a link with no lifetime was made")
	}
}

// TestDownloadLink: a read-only link that answers as a download of the file's name and type.
func TestDownloadLink(t *testing.T) {
	s := keyedStore(t)
	l, expires, err := s.DownloadLink("projects/p1/a b.pdf", `r"é.pdf`, "application/octet-stream", 10*time.Minute)
	if err != nil {
		t.Fatalf("DownloadLink: %v", err)
	}
	if time.Until(expires) <= 0 {
		t.Errorf("expires = %v", expires)
	}
	u, err := url.Parse(l)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("sp") != "r" || q.Get("spr") != "https" || q.Get("sr") != "b" || q.Get("sig") == "" {
		t.Errorf("query = %v", q)
	}
	// The answer is a download with the name and type the link was signed for.
	if q.Get("rsct") != "application/octet-stream" {
		t.Errorf("rsct = %q", q.Get("rsct"))
	}
	if cd := q.Get("rscd"); !strings.HasPrefix(cd, "attachment;") || strings.Contains(cd, `"`) {
		t.Errorf("rscd = %q", cd)
	}
}

// TestLinks_NeedAnAccountKey: a store signed in as an Entra app cannot make a link.
func TestLinks_NeedAnAccountKey(t *testing.T) {
	entra, err := New(Config{Container: "c", AccountName: "a", TenantID: "11111111-1111-1111-1111-111111111111", ClientID: "22222222-2222-2222-2222-222222222222", ClientSecret: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if entra.CanSignLinks() {
		t.Error("an Entra store claims it can sign links")
	}
	if _, _, err := entra.UploadLinks("a.pdf", 1, time.Minute); !errors.Is(err, ErrLinksUnavailable) {
		t.Errorf("UploadLinks err = %v, want ErrLinksUnavailable", err)
	}
	if _, _, err := entra.DownloadLink("a.pdf", "a.pdf", "x/y", time.Minute); !errors.Is(err, ErrLinksUnavailable) {
		t.Errorf("DownloadLink err = %v, want ErrLinksUnavailable", err)
	}
	if !keyedStore(t).CanSignLinks() {
		t.Error("a keyed store cannot sign links")
	}
}

// TestLinks_ConnectionString: a connection string with a key signs links, one without does not.
func TestLinks_ConnectionString(t *testing.T) {
	cs := "DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=" + testKey + ";BlobEndpoint=http://127.0.0.1:10000/devstoreaccount1;"
	s, err := New(Config{Container: "files", ConnectionString: cs})
	if err != nil {
		t.Fatal(err)
	}
	if !s.CanSignLinks() {
		t.Fatal("a connection string with a key cannot sign links")
	}
	links, _, err := s.UploadLinks("a.pdf", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(links[0])
	// Azurite runs on http, so the link must allow it.
	if u.Scheme != "http" || u.Query().Get("spr") != "https,http" {
		t.Errorf("link = %s", links[0])
	}

	// A connection string with no key (for example one carrying a SAS) cannot sign.
	noKey, err := New(Config{Container: "files", ConnectionString: "BlobEndpoint=https://a.blob.core.windows.net;SharedAccessSignature=sv=2024"})
	if err != nil {
		t.Fatal(err)
	}
	if noKey.CanSignLinks() {
		t.Error("a connection string with no key claims it can sign links")
	}
}

// Copy reads its source through a link, so a store that cannot sign cannot copy.
func TestCopyNeedsAnAccountKey(t *testing.T) {
	s := &Store{}
	if err := s.Copy(t.Context(), "a/b.bin.upload", "a/b.bin"); err == nil || !errors.Is(err, ErrLinksUnavailable) {
		t.Fatalf("Copy without a signer = %v, want ErrLinksUnavailable", err)
	}
}

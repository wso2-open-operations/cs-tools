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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// Package filestore is entity-service's client for the SFTPGo server that
// stores attachment files, and the one place that knows the folder layout.
//
// entity-service logs in to SFTPGo as a single service user and never hands
// that login to a browser. A browser gets an SFTPGo *share* instead: a link
// scoped to one file (download) or one folder (upload), with an expiry and a
// use limit. The endpoints used here were checked by hand against SFTPGo
// v2.7.20260705; ListDir's JSON shape and ReadHead's Range support are the two
// that were not, and are marked where they are used.
package filestore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned when SFTPGo answers 404: a missing file or folder.
var ErrNotFound = errors.New("filestore: not found")

// tokenMaxAge is how long a service token is reused. SFTPGo's default token
// lifetime is 20 minutes, so this stays well inside it.
const tokenMaxAge = 10 * time.Minute

// Share scopes, as SFTPGo numbers them.
const (
	ShareRead  = 1
	ShareWrite = 2
)

// ShareSpec describes one share to create.
type ShareSpec struct {
	// Name is a label only; SFTPGo shows it to nobody we serve.
	Name string
	// Scope is ShareRead or ShareWrite.
	Scope int
	// Path is the one file (read) or existing folder (write) the share covers.
	Path string
	// MaxUses is how many requests the share serves before SFTPGo refuses it.
	// A write share with 1 accepts exactly one upload.
	MaxUses int
	// ExpiresAt is when the share stops working.
	ExpiresAt time.Time
}

// Entry is one item of a folder listing.
type Entry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	// Mode carries Go's os.FileMode bits, where bit 31 marks a folder.
	Mode uint32 `json:"mode"`
}

// IsDir reports whether the entry is a folder.
func (e Entry) IsDir() bool { return e.Mode&(1<<31) != 0 }

// Client talks to SFTPGo's HTTP API with the service user's credentials.
type Client struct {
	baseURL   string
	publicURL string
	user      string
	password  string
	http      *http.Client

	mu      sync.Mutex
	token   string
	tokenAt time.Time
}

// NewClient builds a Client. baseURL is where entity-service reaches SFTPGo;
// publicURL is where a browser reaches it (the two differ once SFTPGo is behind
// a public proxy) and falls back to baseURL when empty.
func NewClient(baseURL, publicURL, user, password string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	publicURL = strings.TrimRight(publicURL, "/")
	if publicURL == "" {
		publicURL = baseURL
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 30 * time.Second
	return &Client{
		baseURL:   baseURL,
		publicURL: publicURL,
		user:      user,
		password:  password,
		http:      &http.Client{Transport: t, Timeout: 60 * time.Second},
	}
}

// DownloadURL is the address a browser fetches a read share's file from.
// compress=false is what makes SFTPGo send the file itself: without it a share
// is served as a ZIP.
func (c *Client) DownloadURL(shareID string) string {
	return c.publicURL + "/api/v2/shares/" + url.PathEscape(shareID) + "?compress=false"
}

// UploadURL is the address a browser POSTs a multipart form (field "filenames")
// to for a write share.
func (c *Client) UploadURL(shareID string) string {
	return c.publicURL + "/api/v2/shares/" + url.PathEscape(shareID)
}

// CreateShare creates a share and returns its id. SFTPGo accepts a share on a
// path that does not exist and only fails when it is used, so callers create
// the folder (MkdirAll) first for a write share.
func (c *Client) CreateShare(ctx context.Context, spec ShareSpec) (string, error) {
	body, err := json.Marshal(map[string]any{
		"name":       spec.Name,
		"scope":      spec.Scope,
		"paths":      []string{abs(spec.Path)},
		"max_tokens": spec.MaxUses,
		"expires_at": spec.ExpiresAt.UnixMilli(),
	})
	if err != nil {
		return "", fmt.Errorf("filestore: encode share: %w", err)
	}
	resp, err := c.exec(ctx, http.MethodPost, "/api/v2/user/shares", nil, body, http.Header{"Content-Type": {"application/json"}})
	if err != nil {
		return "", err
	}
	defer drainClose(resp.Body)
	// The share's id is in a header; the body is only a message.
	id := resp.Header.Get("X-Object-Id")
	if id == "" {
		return "", errors.New("filestore: share created but SFTPGo returned no id")
	}
	return id, nil
}

// MkdirAll creates dir and any missing parents. Creating a folder that exists
// is not an error.
func (c *Client) MkdirAll(ctx context.Context, dir string) error {
	q := url.Values{"path": {abs(dir)}, "mkdir_parents": {"true"}}
	resp, err := c.exec(ctx, http.MethodPost, "/api/v2/user/dirs", q, nil, nil)
	if err != nil {
		return err
	}
	drainClose(resp.Body)
	return nil
}

// ListDir lists dir. A missing folder returns ErrNotFound.
//
// Not yet confirmed against a live server: the response is read as a JSON
// array of {name, size, mode} objects, as SFTPGo's API reference describes.
func (c *Client) ListDir(ctx context.Context, dir string) ([]Entry, error) {
	q := url.Values{"path": {abs(dir)}}
	resp, err := c.exec(ctx, http.MethodGet, "/api/v2/user/dirs", q, nil, nil)
	if err != nil {
		return nil, err
	}
	defer drainClose(resp.Body)
	var entries []Entry
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&entries); err != nil {
		return nil, fmt.Errorf("filestore: decode listing: %w", err)
	}
	return entries, nil
}

// Rename moves a file to a new path. The target's folder must exist.
func (c *Client) Rename(ctx context.Context, from, to string) error {
	q := url.Values{"path": {abs(from)}, "target": {abs(to)}}
	resp, err := c.exec(ctx, http.MethodPatch, "/api/v2/user/files", q, nil, nil)
	if err != nil {
		return err
	}
	drainClose(resp.Body)
	return nil
}

// RemoveFile deletes one file. A file that is already gone returns
// ErrNotFound, which a caller deleting an attachment can ignore.
func (c *Client) RemoveFile(ctx context.Context, path string) error {
	q := url.Values{"path": {abs(path)}}
	resp, err := c.exec(ctx, http.MethodDelete, "/api/v2/user/files", q, nil, nil)
	if err != nil {
		return err
	}
	drainClose(resp.Body)
	return nil
}

// RemoveDir deletes a folder and everything in it. A folder that is already
// gone returns ErrNotFound, which callers cleaning up can ignore.
func (c *Client) RemoveDir(ctx context.Context, dir string) error {
	q := url.Values{"path": {abs(dir)}}
	resp, err := c.exec(ctx, http.MethodDelete, "/api/v2/user/dirs", q, nil, nil)
	if err != nil {
		return err
	}
	drainClose(resp.Body)
	return nil
}

// ReadHead returns up to n leading bytes of a file, for checking its real type.
//
// Not yet confirmed against a live server: it asks for a byte range, and reads
// at most n bytes whether SFTPGo honours the range (206) or sends the whole file.
func (c *Client) ReadHead(ctx context.Context, path string, n int) ([]byte, error) {
	q := url.Values{"path": {abs(path)}}
	hdr := http.Header{"Range": {fmt.Sprintf("bytes=0-%d", n-1)}}
	resp, err := c.exec(ctx, http.MethodGet, "/api/v2/user/files", q, nil, hdr)
	if err != nil {
		return nil, err
	}
	defer drainClose(resp.Body)
	head, err := io.ReadAll(io.LimitReader(resp.Body, int64(n)))
	if err != nil {
		return nil, fmt.Errorf("filestore: read file head: %w", err)
	}
	return head, nil
}

// exec sends one request with the service token and maps the result: 2xx
// returns the response (the caller closes its body), 404 is ErrNotFound, and a
// 401 refreshes the token and tries once more, since the cached token may have
// expired or been revoked.
func (c *Client) exec(ctx context.Context, method, endpoint string, q url.Values, body []byte, header http.Header) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		tok, err := c.bearer(ctx, attempt > 0)
		if err != nil {
			return nil, err
		}
		target := c.baseURL + endpoint
		if len(q) > 0 {
			target += "?" + q.Encode()
		}
		var rd io.Reader
		if body != nil {
			rd = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, target, rd)
		if err != nil {
			return nil, fmt.Errorf("filestore: build request: %w", err)
		}
		for k, vs := range header {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
		req.Header.Set("Authorization", "Bearer "+tok)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("filestore: %s %s: %w", method, endpoint, err)
		}
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return resp, nil
		case resp.StatusCode == http.StatusUnauthorized && attempt == 0:
			drainClose(resp.Body)
			continue
		default:
			defer drainClose(resp.Body)
			return nil, statusError(req, resp)
		}
	}
}

// bearer returns the cached service token, fetching a new one when it is old or
// force is set. The user API takes basic auth only on the token endpoint.
func (c *Client) bearer(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.token != "" && time.Since(c.tokenAt) < tokenMaxAge {
		return c.token, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/user/token", nil)
	if err != nil {
		return "", fmt.Errorf("filestore: build token request: %w", err)
	}
	req.SetBasicAuth(c.user, c.password)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("filestore: fetch token: %w", err)
	}
	defer drainClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		// The status only: the body and the credentials stay out of the error.
		return "", fmt.Errorf("filestore: fetch token: status %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil || out.AccessToken == "" {
		return "", errors.New("filestore: SFTPGo returned no token")
	}
	c.token, c.tokenAt = out.AccessToken, time.Now()
	return c.token, nil
}

// statusError turns a non-2xx response into an error. 404 is ErrNotFound; the
// rest carry SFTPGo's own message (never a token or a credential).
func statusError(req *http.Request, resp *http.Response) error {
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s %s", ErrNotFound, req.Method, req.URL.Path)
	}
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&body)
	detail := strings.TrimSpace(body.Error + " " + body.Message)
	return fmt.Errorf("filestore: %s %s: status %d: %s", req.Method, req.URL.Path, resp.StatusCode, detail)
}

func drainClose(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 1<<20))
	_ = body.Close()
}

// abs gives an SFTPGo path its leading slash.
func abs(p string) string { return "/" + strings.TrimLeft(p, "/") }

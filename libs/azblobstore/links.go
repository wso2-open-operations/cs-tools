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
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

// ErrLinksUnavailable: the store does not sign in with an account key, and signing
// as an Entra app is not supported yet, so it makes no direct-transfer link.
// Callers keep sending the bytes themselves.
var ErrLinksUnavailable = errors.New("azblobstore: links need an account key or a connection string")

// clockSkew is how far a link's start is set back, so a client whose clock runs a
// little behind Azure's is not refused.
const clockSkew = time.Minute

// CanSignLinks reports whether UploadLinks and DownloadLink work. They need the
// account key; signing as an Entra app (a user delegation key) is not supported yet.
func (s *Store) CanSignLinks() bool { return s.signer != nil }

// UploadLinks returns one link per block index 0 to count-1. A client sends the
// bytes of block i with a PUT to links[i], straight to Azure, and the blocks are
// then made into the file with Commit, as for blocks sent through PutBlock.
//
// Every link is signed for this one blob, only for writing, and stops working at
// the returned time. Azure checks a link when a request starts, so a request that
// started before then finishes. A link cannot limit how much is sent, so the
// caller must check the real size after the upload (Stat).
func (s *Store) UploadLinks(key string, count int, ttl time.Duration) ([]string, time.Time, error) {
	if err := validateKey(key); err != nil {
		return nil, time.Time{}, err
	}
	if count < 1 || count > MaxBlocks {
		return nil, time.Time{}, fmt.Errorf("%w: %d", ErrBlockCount, count)
	}
	query, expires, err := s.sign(key, ttl, sas.BlobPermissions{Write: true}, "", "")
	if err != nil {
		return nil, time.Time{}, err
	}
	base := s.container.NewBlobClient(key).URL() + "?" + query
	links := make([]string, count)
	for i := range links {
		id, err := blockID(i)
		if err != nil {
			return nil, time.Time{}, err
		}
		links[i] = base + "&comp=block&blockid=" + url.QueryEscape(id)
	}
	return links, expires, nil
}

// DownloadLink returns a link a client reads the whole blob from, straight from
// Azure (a client can send a Range header to read part of it). The link is
// signed for this one blob, only for reading, and for the answer to be a download
// of fileName with the given content type, whatever the blob itself says.
func (s *Store) DownloadLink(key, fileName, contentType string, ttl time.Duration) (string, time.Time, error) {
	if err := validateKey(key); err != nil {
		return "", time.Time{}, err
	}
	disposition := "attachment; filename*=UTF-8''" + url.PathEscape(fileName)
	query, expires, err := s.sign(key, ttl, sas.BlobPermissions{Read: true}, disposition, contentType)
	if err != nil {
		return "", time.Time{}, err
	}
	return s.container.NewBlobClient(key).URL() + "?" + query, expires, nil
}

// copySourceTTL is how long the link Copy reads its source through lasts. Azure
// reads the source while it handles the request, so a short time is enough.
const copySourceTTL = 2 * time.Minute

// copyPollEvery is how often Copy checks a copy that Azure has not finished.
const copyPollEvery = 200 * time.Millisecond

// Copy makes dstKey a copy of the committed blob at srcKey (content and type),
// inside Azure, replacing what is at dstKey. It needs an account key, because
// Azure reads the source through a short-lived link. When srcETag is not empty
// the copy is refused with ErrSourceChanged if the source no longer has it, so a
// file that was checked is the file that is copied. It returns ErrNotFound when
// srcKey is missing and waits, until ctx ends, for a copy that is still pending.
func (s *Store) Copy(ctx context.Context, srcKey, dstKey, srcETag string) error {
	if err := validateKey(srcKey); err != nil {
		return err
	}
	if err := validateKey(dstKey); err != nil {
		return err
	}
	query, _, err := s.sign(srcKey, copySourceTTL, sas.BlobPermissions{Read: true}, "", "")
	if err != nil {
		return err
	}
	source := s.container.NewBlobClient(srcKey).URL() + "?" + query
	dst := s.container.NewBlobClient(dstKey)
	var opts *blob.StartCopyFromURLOptions
	if srcETag != "" {
		etag := azcore.ETag(srcETag)
		opts = &blob.StartCopyFromURLOptions{SourceModifiedAccessConditions: &blob.SourceModifiedAccessConditions{SourceIfMatch: &etag}}
	}
	resp, err := dst.StartCopyFromURL(ctx, source, opts)
	if bloberror.HasCode(err, bloberror.SourceConditionNotMet, bloberror.ConditionNotMet) {
		return ErrSourceChanged
	}
	if isNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("azblobstore: copy %s to %s: %w", srcKey, dstKey, err)
	}
	for status := derefStatus(resp.CopyStatus); status != blob.CopyStatusTypeSuccess; {
		if status != blob.CopyStatusTypePending {
			return fmt.Errorf("azblobstore: copy %s to %s ended %s", srcKey, dstKey, status)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("azblobstore: copy %s to %s: %w", srcKey, dstKey, ctx.Err())
		case <-time.After(copyPollEvery):
		}
		props, err := dst.GetProperties(ctx, nil)
		if err != nil {
			return fmt.Errorf("azblobstore: copy %s to %s: %w", srcKey, dstKey, err)
		}
		status = derefStatus(props.CopyStatus)
	}
	return nil
}

// derefStatus reads a copy status that Azure may not have sent.
func derefStatus(s *blob.CopyStatusType) blob.CopyStatusType {
	if s == nil {
		return blob.CopyStatusTypeSuccess
	}
	return *s
}

// sign makes the query string of a link for one blob. disposition and
// contentType, when not empty, are the headers Azure must answer with.
func (s *Store) sign(key string, ttl time.Duration, perms sas.BlobPermissions, disposition, contentType string) (string, time.Time, error) {
	if s.signer == nil {
		return "", time.Time{}, ErrLinksUnavailable
	}
	if ttl <= 0 {
		return "", time.Time{}, errors.New("azblobstore: a link needs a lifetime")
	}
	now := time.Now().UTC()
	expires := now.Add(ttl)
	protocol := sas.ProtocolHTTPS
	if strings.HasPrefix(s.container.URL(), "http://") {
		// Azurite on a developer's machine.
		protocol = sas.ProtocolHTTPSandHTTP
	}
	values := sas.BlobSignatureValues{
		Protocol:           protocol,
		StartTime:          now.Add(-clockSkew),
		ExpiryTime:         expires,
		Permissions:        perms.String(),
		ContainerName:      s.containerName(),
		BlobName:           key,
		ContentDisposition: disposition,
		ContentType:        contentType,
	}
	query, err := values.SignWithSharedKey(s.signer)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("azblobstore: sign link for %s: %w", key, err)
	}
	return query.Encode(), expires, nil
}

// containerName is the last part of the container's URL.
func (s *Store) containerName() string {
	u := strings.TrimRight(s.container.URL(), "/")
	return u[strings.LastIndex(u, "/")+1:]
}

// keyFromConnectionString reads the account name and key out of a connection
// string, or returns nil when it has none (for example a connection string that
// carries a SAS instead).
func keyFromConnectionString(connectionString string) *blob.SharedKeyCredential {
	var name, key string
	for _, part := range strings.Split(connectionString, ";") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "AccountName":
			name = strings.TrimSpace(v)
		case "AccountKey":
			// The key is base64 and can itself end in "=", so only the first "=" splits.
			key = strings.TrimSpace(v)
		}
	}
	if name == "" || key == "" {
		return nil
	}
	cred, err := blob.NewSharedKeyCredential(name, key)
	if err != nil {
		return nil
	}
	return cred
}

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
// Package inlineimages turns the image references in a comment's or
// description's HTML back into the images themselves, for notification emails.
//
// Since attachments live in Azure Blob Storage, a pasted image is stored as an
// attachment and the HTML only holds a reference to it (src="/<attachment id>").
// An email cannot show a reference, and the sanitizer in internal/notifications
// deliberately accepts only a self-contained data: image, so this package reads
// each referenced file and rewrites the reference to a data: URI before the
// sanitizer runs. The sanitizer then attaches it to the email as an inline
// picture exactly as it does for any other embedded image.
package inlineimages

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"
)

const (
	// maxImages is how many images of one body are resolved (entity-service
	// allows no more than this per comment or description).
	maxImages = 5
	// maxImageBytes bounds one image, in step with the email sanitizer's own cap.
	maxImageBytes = 5 << 20
	// maxTotalBytes bounds all images of one body, so a body is not read in full
	// only for the sanitizer to drop the excess.
	maxTotalBytes = 8 << 20
	// resolveTimeout bounds the whole of one Resolve call.
	resolveTimeout = 30 * time.Second
)

// Locator finds where an inline image's file is (entity-service).
type Locator interface {
	LocateInlineImage(ctx context.Context, id string) (entity.InlineImageLocation, error)
}

// Reader reads a file from storage (azblobstore.Store).
type Reader interface {
	Open(ctx context.Context, key string, rng *azblobstore.Range) (*azblobstore.Object, error)
}

// Resolver rewrites image references to data: URIs.
type Resolver struct {
	locator Locator
	reader  Reader
}

// New builds a Resolver.
func New(locator Locator, reader Reader) *Resolver {
	return &Resolver{locator: locator, reader: reader}
}

// imgSrc finds an <img> tag's src attribute.
var imgSrc = regexp.MustCompile(`(?i)(<img\b[^>]*?\bsrc\s*=\s*)(["'])([^"']*)(["'])`)

// refID reads the attachment id out of an image reference: src="/<id>" (also
// with the .iix suffix migrated comments use) or a URL ending
// attachments/inline/<id>/content.
var refID = regexp.MustCompile(`(?i)^(?:.*attachments/inline/)?/?([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:/content|\.iix)?$`)

// allowedTypes are the image types the email sanitizer accepts.
var allowedTypes = map[string]string{
	"image/png": "image/png", "image/jpeg": "image/jpeg", "image/jpg": "image/jpeg",
	"image/gif": "image/gif", "image/webp": "image/webp",
}

// Resolve returns html with each referenced image replaced by a data: URI. An
// image that cannot be read (removed, too big, not an image, storage down) is
// left as a reference, which the sanitizer then drops, so a failure only costs
// that one picture and never the email. Content with no reference is returned
// unchanged without any lookup.
func (r *Resolver) Resolve(ctx context.Context, html string) string {
	if r == nil || !strings.Contains(strings.ToLower(html), "<img") {
		return html
	}
	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	seen := map[string]string{} // attachment id -> data URI, "" when it could not be read
	resolved, total := 0, 0
	return imgSrc.ReplaceAllStringFunc(html, func(tag string) string {
		m := imgSrc.FindStringSubmatch(tag)
		id := refID.FindStringSubmatch(m[3])
		if id == nil {
			return tag
		}
		key := strings.ToLower(id[1])
		uri, done := seen[key]
		if !done {
			if resolved >= maxImages {
				return tag
			}
			var n int
			uri, n = r.dataURI(ctx, key, maxTotalBytes-total)
			seen[key] = uri
			if uri != "" {
				resolved++
				total += n
			}
		}
		if uri == "" {
			return tag
		}
		return m[1] + m[2] + uri + m[4]
	})
}

// dataURI reads one image and returns it as a data: URI and its size, or "" when
// it cannot be used. room is how many more bytes this body may add.
func (r *Resolver) dataURI(ctx context.Context, id string, room int) (string, int) {
	loc, err := r.locator.LocateInlineImage(ctx, id)
	if err != nil || !loc.Available || loc.StorageKey == "" {
		slog.WarnContext(ctx, "inlineimages: image not available for the email", "attachmentId", id)
		return "", 0
	}
	contentType := ""
	if loc.ContentType != nil {
		contentType = strings.ToLower(strings.TrimSpace(strings.SplitN(*loc.ContentType, ";", 2)[0]))
	}
	mediaType, ok := allowedTypes[contentType]
	if !ok {
		slog.WarnContext(ctx, "inlineimages: not an image type the email accepts", "attachmentId", id)
		return "", 0
	}
	limit := min(maxImageBytes, room)
	if limit <= 0 || (loc.SizeBytes != nil && *loc.SizeBytes > int64(limit)) {
		slog.WarnContext(ctx, "inlineimages: image too large for the email", "attachmentId", id)
		return "", 0
	}
	obj, err := r.reader.Open(ctx, loc.StorageKey, nil)
	if err != nil {
		slog.WarnContext(ctx, "inlineimages: cannot read the image file", "attachmentId", id)
		return "", 0
	}
	defer obj.Body.Close()
	data, err := io.ReadAll(io.LimitReader(obj.Body, int64(limit)+1))
	if err != nil || len(data) == 0 || len(data) > limit {
		slog.WarnContext(ctx, "inlineimages: image could not be read in full or is too large", "attachmentId", id)
		return "", 0
	}
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data), len(data)
}

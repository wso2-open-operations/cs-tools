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

package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/filestore"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// newInlineImageTypes are the only types an image added to a comment may have.
// SVG is left out on purpose (it can carry script), and so is GIF. Images already
// stored with another type are still shown (inlineImageTypes).
var newInlineImageTypes = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
}

// embeddedImageSrc finds an <img> whose src is a base64 data URI. Capture
// groups: 1 = everything up to and including the opening quote of src, 2 = the
// image subtype, 3 = the base64 payload (an editor may wrap it over several
// lines), 4 = the closing quote.
var embeddedImageSrc = regexp.MustCompile(`(?is)(<img\b[^>]*?\bsrc\s*=\s*["'])data:image/([a-z0-9.+-]+);base64,([A-Za-z0-9+/=\s]+)(["'])`)

// ExtractInlineImages implements InlineImageExtractor.
//
// An image pasted into a comment arrives as a base64 data URI inside the
// comment's HTML. Left there it would sit in the comment row for good, so each
// one is stored as a comment_attachment row with its file in SFTPGo, and the
// comment keeps only the reference, src="/<attachment id>" (the shape comments
// migrated from the previous system already use, which the portals resolve). The
// rows start unlinked (comment_id NULL); LinkFromContent attaches them once the
// comment exists.
//
// Only PNG, JPEG and WebP up to the inline image limit are accepted, and the
// bytes must really be one of those types. The caller's own access to the work
// item is checked first (a hidden work item is not found). If any image cannot
// be stored the ones already stored are removed and the error is returned, so
// the comment is not saved with a half-converted body. Comments that are
// already saved are never touched.
func (s *storedAttachmentService) ExtractInlineImages(ctx context.Context, workItemID, authorEmail, content string) (string, error) {
	if !strings.Contains(content, "base64,") || authorEmail == "" || !validate.IsUUID(workItemID) {
		return content, nil
	}
	matches := embeddedImageSrc.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, nil
	}
	if len(matches) > maxAttachmentLinkIDs {
		return "", &apierror.ValidationError{Msg: fmt.Sprintf("a comment can hold at most %d images", maxAttachmentLinkIDs)}
	}

	// Run as the caller: a work item they cannot see is not found.
	parent, err := s.repo.GetParent(ctx, domain.StoredAttachmentComment, workItemID)
	if err != nil {
		return "", err
	}

	type stored struct{ id, key string }
	var done []stored
	rollback := func() {
		for _, d := range done {
			if err := s.store.RemoveFile(ctx, d.key); err != nil {
				slog.WarnContext(ctx, "inline image rollback: could not remove file", "attachmentId", d.id, "error", err)
			}
			s.failQuietly(ctx, domain.StoredAttachmentComment, d.id, "comment was not saved")
		}
	}

	var out strings.Builder
	last := 0
	for i, m := range matches {
		prefix, subtype, payload, quote := content[m[2]:m[3]], content[m[4]:m[5]], content[m[6]:m[7]], content[m[8]:m[9]]
		declared := "image/" + strings.ToLower(subtype)
		if declared == "image/jpg" {
			declared = "image/jpeg"
		}
		if _, ok := newInlineImageTypes[declared]; !ok {
			rollback()
			return "", &apierror.ValidationError{Msg: "only PNG, JPEG and WebP images can be added to a comment"}
		}
		data, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(payload), ""))
		if err != nil {
			rollback()
			return "", &apierror.ValidationError{Msg: "an image in the comment could not be read"}
		}
		if int64(len(data)) > s.limits.InlineMaxBytes {
			rollback()
			return "", &apierror.ValidationError{Msg: "an image is larger than the " + formatByteLimit(s.limits.InlineMaxBytes) + " limit"}
		}
		// The type in the data URI is only a claim; look at the bytes.
		head := data
		if len(head) > sniffBytes {
			head = head[:sniffBytes]
		}
		actual := strings.ToLower(strings.SplitN(http.DetectContentType(head), ";", 2)[0])
		ext, ok := newInlineImageTypes[actual]
		if !ok {
			rollback()
			return "", &apierror.ValidationError{Msg: "an image in the comment is not a PNG, JPEG or WebP image"}
		}

		name := fmt.Sprintf("inline-image-%d-%d.%s", time.Now().UnixMilli(), i+1, ext)
		row, err := s.repo.InsertPending(ctx, domain.StoredAttachmentComment, domain.StoredAttachmentInsert{
			ParentID: workItemID, Name: name, ContentType: actual,
			SizeBytes: int64(len(data)), CreatedBy: authorEmail,
		})
		if err != nil {
			rollback()
			return "", err
		}
		key := filestore.Key(filestore.KeyParts{
			ProjectNumber: parent.ProjectNumber, WorkItemNumber: parent.Number,
			Inline: true, AttachmentNumber: row.Number, Name: name,
		})
		if err := s.store.PutFile(ctx, path.Dir(key), path.Base(key), data); err != nil {
			s.failQuietly(ctx, domain.StoredAttachmentComment, row.ID, "image could not be stored")
			rollback()
			return "", storageUnavailable(ctx, "store inline image", err)
		}
		ok, err = s.repo.MarkComplete(repository.WithSystemIdentity(ctx), domain.StoredAttachmentComment, row.ID, key, int64(len(data)))
		if err != nil || !ok {
			if rmErr := s.store.RemoveFile(ctx, key); rmErr != nil {
				slog.WarnContext(ctx, "inline image: could not remove file", "attachmentId", row.ID, "error", rmErr)
			}
			s.failQuietly(ctx, domain.StoredAttachmentComment, row.ID, "image could not be recorded")
			rollback()
			if err != nil {
				return "", err
			}
			return "", &apierror.ConflictError{Msg: "an image could not be stored; please try again"}
		}
		done = append(done, stored{id: row.ID, key: key})

		out.WriteString(content[last:m[2]])
		out.WriteString(prefix + "/" + strings.ToLower(row.ID) + quote)
		last = m[9]
	}
	out.WriteString(content[last:])

	slog.InfoContext(ctx, "comment images stored",
		"audit", true, "action", "extract", "callerId", auditCaller(ctx),
		"workItemId", workItemID, "images", len(done))
	return out.String(), nil
}

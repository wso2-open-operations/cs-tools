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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"
)

const (
	// relocateBlockBytes is the size of each block written while moving an image.
	relocateBlockBytes = 8 << 20
	// relocateDeadline bounds one move; it runs detached from the browser's
	// request so a closed tab does not leave it half done.
	relocateDeadline = 2 * time.Minute
)

// relocateInlineImage moves a description image's file from the project's drafts
// folder to the work item's inline_attachments folder (loc.RelocateTo), then tells
// entity-service, which points the row at the new key. It returns true only when
// everything succeeded, so the caller then reads from the new key; on any failure
// the old file is left as it was (and removed only after entity-service accepted
// the new key), the failure is logged, and the caller reads the old key. The image
// is never unavailable because a move failed. Not repeated for an image already
// moved: entity-service stops returning relocateTo once the row has the new key.
func (h *BlobAttachmentHandler) relocateInlineImage(ctx context.Context, id string, loc attachmentLocation) bool {
	from, to := loc.StorageKey, loc.RelocateTo
	if to == "" || from == "" || from == to {
		return false
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), relocateDeadline)
	defer cancel()

	src, err := h.store.Open(ctx, from, nil)
	if err != nil {
		slog.WarnContext(ctx, "inline image move: cannot read the file", "attachmentID", id, "err", summarizeErr(err))
		return false
	}
	defer src.Body.Close()
	size := src.ContentLength
	if size <= 0 {
		return false
	}
	blocks := 0
	for offset := int64(0); offset < size; blocks++ {
		n := min(int64(relocateBlockBytes), size-offset)
		if err := h.store.PutBlock(ctx, to, blocks, io.LimitReader(src.Body, n), n); err != nil {
			slog.WarnContext(ctx, "inline image move: cannot write the copy", "attachmentID", id, "err", summarizeErr(err))
			return false
		}
		offset += n
	}
	if err := h.store.Commit(ctx, to, blocks, src.Info.ContentType); err != nil {
		slog.WarnContext(ctx, "inline image move: cannot commit the copy", "attachmentID", id, "err", summarizeErr(err))
		return false
	}
	if got, err := h.store.Stat(ctx, to); err != nil || got.Size != size {
		slog.WarnContext(ctx, "inline image move: the copy is not the same size", "attachmentID", id, "want", size, "err", summarizeErr(err))
		_ = h.store.Delete(ctx, to)
		return false
	}
	body, _ := json.Marshal(map[string]string{"storageKey": to})
	if err := h.entity.InlineImageRelocated(ctx, id, body); err != nil {
		// The copy stays: the next read copies the same bytes over it.
		slog.WarnContext(ctx, "inline image move: entity-service did not accept the new key", "attachmentID", id, "err", summarizeErr(err))
		return false
	}
	if err := h.store.Delete(ctx, from); err != nil {
		slog.WarnContext(ctx, "inline image move: cannot remove the old file", "attachmentID", id, "err", fmt.Sprint(summarizeErr(err)))
	}
	slog.InfoContext(ctx, "inline image moved to its work item folder", "attachmentID", id)
	return true
}

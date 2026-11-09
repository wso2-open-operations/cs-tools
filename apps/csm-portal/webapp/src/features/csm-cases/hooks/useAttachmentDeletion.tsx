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


import { useCallback, useState, type JSX, type ReactNode } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useDeleteCsmCaseAttachment } from "@features/csm-cases/api/useCsmCaseAttachments";
import { canDeleteAttachment } from "@features/csm-cases/utils/canDeleteAttachment";
import type { CaseAttachment } from "@features/csm-cases/types/csmCases";
import type { BeReferenceType } from "@api/backend/types";

interface Args {
  /** The owning incident / change request id. */
  ownerId: string | undefined;
  referenceType: BeReferenceType;
  /** Whether the owner is in a terminal state (nobody may delete then). */
  closed: boolean;
}

interface Result {
  /** Pass to AttachmentsWidget `onDelete`; undefined hides every Delete button. */
  onDelete: ((a: CaseAttachment) => void) | undefined;
  canDeleteAttachment: (a: CaseAttachment) => boolean;
  deletingId: string | null;
  /** Render once in the page: the confirmation dialog. */
  dialog: ReactNode;
}

/**
 * Delete-an-attachment wiring for the incident and change-request pages: the
 * uploader may delete, and so may an admin; nobody once the owner is closed.
 * UX only -- the backend and entity-service enforce the same rule.
 */
export function useAttachmentDeletion({ ownerId, referenceType, closed }: Args): Result {
  const { canWrite, isAdmin } = usePortalAccess();
  const { user } = useCurrentUser();
  const claimEmail = useIdTokenClaims()?.email;
  const { showError } = useErrorBanner();
  const deleteAttachment = useDeleteCsmCaseAttachment();
  const [pending, setPending] = useState<CaseAttachment | null>(null);

  const canDeleteRow = useCallback(
    (a: CaseAttachment): boolean =>
      canDeleteAttachment({
        caseClosed: closed,
        isAdmin,
        myEmail: user?.email || claimEmail,
        uploaderEmail: a.uploadedByUser?.email || a.uploadedByEmail,
      }),
    [closed, isAdmin, user?.email, claimEmail],
  );

  const confirm = useCallback(() => {
    if (!ownerId || !pending) return;
    const target = pending;
    deleteAttachment.mutate(
      { caseId: ownerId, attachmentId: target.id, referenceType },
      {
        onSuccess: () => setPending(null),
        onError: (err) => {
          setPending(null);
          showError(`Could not delete ${target.filename}.`, err);
        },
      },
    );
  }, [ownerId, pending, referenceType, deleteAttachment, showError]);

  const dialog: JSX.Element = (
    <Dialog
      open={!!pending}
      onClose={() => {
        if (!deleteAttachment.isPending) setPending(null);
      }}
      maxWidth="xs"
      fullWidth
    >
      <DialogTitle>Delete attachment?</DialogTitle>
      <DialogContent>
        <Typography variant="body2">
          Permanently delete <strong>{pending?.filename}</strong>? This can't be undone.
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={() => setPending(null)} disabled={deleteAttachment.isPending}>
          Cancel
        </Button>
        <Button variant="contained" color="error" onClick={confirm} disabled={deleteAttachment.isPending}>
          {deleteAttachment.isPending ? "Deleting…" : "Delete"}
        </Button>
      </DialogActions>
    </Dialog>
  );

  return {
    onDelete: canWrite ? setPending : undefined,
    canDeleteAttachment: canDeleteRow,
    deletingId: deleteAttachment.isPending ? (pending?.id ?? null) : null,
    dialog,
  };
}

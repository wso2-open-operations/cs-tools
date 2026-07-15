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

import { Suspense, useState, type ReactNode } from "react";
import { useQueryClient, useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { Button, IconButton, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { Download, FileText } from "@wso2/oxygen-ui-icons-react";
import { attachments as attachmentsService } from "@src/services/attachments";
import type { CaseAttachment } from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { ErrorState } from "@components/support/ErrorState";
import { AttachmentsField } from "@components/support/AttachmentsField";
import { formatBytes, type PendingAttachment } from "@utils/attachments";
import { formatDate } from "@utils/dateTime";
import { openUrl } from "@components/microapp-bridge";
import { Logger } from "@utils/logger";

/**
 * Full attachment list + upload for a case — the standalone flow (10 MB cap), distinct from the
 * comment composer's 5 MB inline attach (see CommentComposer.tsx / utils/attachments.ts).
 */
export function AttachmentsTab({ caseId }: { caseId: string }) {
  return (
    <AttachmentsTabErrorBoundary>
      <Suspense fallback={<AttachmentsTabSkeleton />}>
        <AttachmentsTabContent caseId={caseId} />
      </Suspense>
    </AttachmentsTabErrorBoundary>
  );
}

function AttachmentsTabContent({ caseId }: { caseId: string }) {
  const queryClient = useQueryClient();
  const { data: caseAttachments } = useSuspenseQuery(attachmentsService.forCase(caseId));
  const [pending, setPending] = useState<PendingAttachment[]>([]);
  const [isUploading, setIsUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleUpload = async () => {
    if (pending.length === 0) return;
    setIsUploading(true);
    setError(null);
    const results = await Promise.allSettled(
      pending.map((attachment) =>
        attachmentsService.create({
          referenceId: caseId,
          referenceType: "case",
          name: attachment.name,
          type: attachment.type,
          file: attachment.file,
        }),
      ),
    );
    const failedCount = results.filter((r) => r.status === "rejected").length;
    if (failedCount > 0) {
      Logger.warn(`${failedCount} attachment(s) failed to upload to case ${caseId}`);
      setError(
        failedCount === pending.length
          ? "Could not upload the file(s). Please try again."
          : `${failedCount} file(s) failed to upload.`,
      );
    }
    setPending([]);
    setIsUploading(false);
    void queryClient.invalidateQueries({ queryKey: ["case", caseId, "attachments"] });
  };

  return (
    <Stack gap={2}>
      <AttachmentsField attachments={pending} onChange={setPending} disabled={isUploading} />

      {pending.length > 0 && (
        <Button
          variant="contained"
          size="small"
          disabled={isUploading}
          onClick={() => void handleUpload()}
          sx={{ alignSelf: "end" }}
        >
          Upload
        </Button>
      )}

      {error && (
        <Typography variant="caption" color="error.main">
          {error}
        </Typography>
      )}

      {caseAttachments.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No attachments on this case.
        </Typography>
      ) : (
        <Stack gap={1}>
          {caseAttachments.map((attachment) => (
            <AttachmentRow key={attachment.id} attachment={attachment} />
          ))}
        </Stack>
      )}
    </Stack>
  );
}

function AttachmentRow({ attachment }: { attachment: CaseAttachment }) {
  return (
    <Stack
      direction="row"
      alignItems="center"
      justifyContent="space-between"
      gap={1}
      sx={{ p: 1, border: "1px solid", borderColor: "divider", borderRadius: 1 }}
    >
      <Stack direction="row" alignItems="center" gap={1} sx={{ minWidth: 0 }}>
        <FileText size={16} />
        <Stack sx={{ minWidth: 0 }}>
          <Typography variant="body2" noWrap>
            {attachment.name}
          </Typography>
          <Typography variant="caption" color="text.secondary" noWrap>
            {formatBytes(attachment.sizeBytes)} · {attachment.createdBy} · {formatDate(attachment.createdOn)}
          </Typography>
        </Stack>
      </Stack>
      {attachment.downloadUrl && (
        <IconButton
          size="small"
          aria-label={`Open ${attachment.name}`}
          onClick={() => openUrl({ url: attachment.downloadUrl as string, presentationStyle: "fullScreen" })}
        >
          <Download size={16} />
        </IconButton>
      )}
    </Stack>
  );
}

function AttachmentsTabSkeleton() {
  return (
    <Stack gap={1}>
      <Skeleton variant="rounded" height={56} />
      <Skeleton variant="rounded" height={56} />
    </Stack>
  );
}

function AttachmentsTabErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}

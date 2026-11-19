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

import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useEffect, useRef, useState, type JSX } from "react";
import {
  useAddProductIssueComment,
  useCloseProductIssue,
  useMoveProductIssueToProductTeam,
} from "@features/csm-cases/api/useProductIssueActions";
import type {
  ProductGitHubCommand,
  ProductIssueDetail,
} from "@features/csm-cases/types/productWorkItems";
import {
  AWAITING_GITHUB_GRACE_MS,
  isAwaitingGithub,
  newRequestId,
  requestWasAnswered,
} from "@features/csm-cases/utils/productWorkItems";
import { usePortalAccess } from "@context/current-user/usePortalAccess";

const MAX_COMMENT_CHARS = 60000;

/** What the dialog shares with the write-back controls: one error slot (the
 * last action's error replaces the previous one) and one retry. */
export interface ProductIssueActionContext {
  setActionError: (message: string | null) => void;
  retry: (commandId: string) => void;
  retrying: boolean;
}

function hasPending(item: ProductIssueDetail, kind: ProductGitHubCommand["kind"]): boolean {
  return (item.commands ?? []).some((c) => c.kind === kind && c.status === "PENDING");
}

function lastFailed(
  item: ProductIssueDetail,
  kind: ProductGitHubCommand["kind"],
): ProductGitHubCommand | undefined {
  const newest = (item.commands ?? []).find((c) => c.kind === kind);
  return newest?.status === "FAILED" ? newest : undefined;
}

function awaiting(
  item: ProductIssueDetail,
  kind: ProductGitHubCommand["kind"],
  now: number,
): boolean {
  return (item.commands ?? []).some((c) => c.kind === kind && isAwaitingGithub(c, now));
}

/** One idempotency key per composed action. Kept only while the request got no
 * answer (network failure), so a true retry dedupes; dropped after success and
 * after any answer from the server. */
function useComposedKey(): {
  keyFor: (input: string) => string;
  clear: () => void;
} {
  const ref = useRef<{ input: string; key: string } | null>(null);
  return {
    keyFor: (input) => {
      if (ref.current?.input !== input) ref.current = { input, key: newRequestId() };
      return ref.current.key;
    },
    clear: () => {
      ref.current = null;
    },
  };
}

/** Comment box under the conversation. */
export function ProductIssueCommentBox({
  item,
  ctx,
}: {
  item: ProductIssueDetail;
  ctx: ProductIssueActionContext;
}): JSX.Element | null {
  const { canWrite } = usePortalAccess();
  const [text, setText] = useState("");
  const add = useAddProductIssueComment(item.id);
  const key = useComposedKey();
  if (!canWrite || item.linkState !== "ACTIVE") return null;

  const body = text.trim();
  const tooLong = body.length > MAX_COMMENT_CHARS;

  const send = (): void => {
    if (!body || tooLong || add.isPending) return;
    ctx.setActionError(null);
    add.mutate(
      { body, idempotencyKey: key.keyFor(body) },
      {
        onSuccess: () => {
          key.clear();
          setText("");
        },
        onError: (e) => {
          if (requestWasAnswered(e)) key.clear();
          ctx.setActionError(e.message);
        },
      },
    );
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <TextField
        label="Comment"
        multiline
        minRows={3}
        fullWidth
        size="small"
        value={text}
        disabled={add.isPending}
        onChange={(e) => setText(e.target.value)}
        error={tooLong}
        helperText={tooLong ? `Comments are limited to ${MAX_COMMENT_CHARS} characters.` : undefined}
      />
      <Box>
        <Button
          size="small"
          variant="contained"
          onClick={send}
          disabled={!body || tooLong || add.isPending}
          sx={{ textTransform: "none" }}
        >
          Send
        </Button>
      </Box>
    </Box>
  );
}

/** "Back to product team" and "Close issue", with pending and failed states. */
export function ProductIssueActions({
  item,
  ctx,
}: {
  item: ProductIssueDetail;
  ctx: ProductIssueActionContext;
}): JSX.Element | null {
  const { canWrite } = usePortalAccess();
  const [confirmOpen, setConfirmOpen] = useState(false);
  // Refreshed whenever the commands change and when the waiting window ends.
  const [now, setNow] = useState(() => Date.now());
  const move = useMoveProductIssueToProductTeam(item.id);
  const close = useCloseProductIssue(item.id);
  const moveKey = useComposedKey();
  const closeKey = useComposedKey();

  const active = item.linkState === "ACTIVE";
  const movePending = hasPending(item, "SET_BOARD_STATUS");
  const closePending = hasPending(item, "SET_STATE");
  const moveAwaiting = awaiting(item, "SET_BOARD_STATUS", now);
  const closeAwaiting = awaiting(item, "SET_STATE", now);
  const moveFailed = lastFailed(item, "SET_BOARD_STATUS");
  const closeFailed = lastFailed(item, "SET_STATE");
  const busy = move.isPending || close.isPending || ctx.retrying;

  const canMove =
    canWrite && active && item.boardStatus === "WAITING_ON_CS" && !movePending && !moveAwaiting;
  const canClose =
    canWrite && active && item.github.state === "OPEN" && !closePending && !closeAwaiting;

  // Re-render when the "waiting for GitHub" window ends, so the button returns
  // if the webhook never came (polling has stopped by then).
  const commands = item.commands;
  useEffect(() => {
    const at = Date.now();
    const expiries = (commands ?? [])
      .filter((c) => isAwaitingGithub(c, at))
      .map((c) => Date.parse(c.deliveredOn as string) + AWAITING_GITHUB_GRACE_MS - at);
    const delay = expiries.length > 0 ? Math.min(...expiries) + 50 : 0;
    const t = setTimeout(() => setNow(Date.now()), delay);
    return () => clearTimeout(t);
  }, [commands]);

  // Forget a stale confirmation once Close is no longer available.
  if (confirmOpen && !canClose) setConfirmOpen(false);

  const showAwaiting = moveAwaiting || closeAwaiting;
  if (
    !canWrite ||
    (!canMove && !canClose && !movePending && !closePending && !showAwaiting && !moveFailed && !closeFailed)
  ) {
    return null;
  }

  const doMove = (): void => {
    ctx.setActionError(null);
    move.mutate(
      { idempotencyKey: moveKey.keyFor("move") },
      {
        onSuccess: () => moveKey.clear(),
        onError: (e) => {
          if (requestWasAnswered(e)) moveKey.clear();
          ctx.setActionError(e.message);
        },
      },
    );
  };
  const doClose = (): void => {
    ctx.setActionError(null);
    close.mutate(
      { idempotencyKey: closeKey.keyFor("close") },
      {
        onSuccess: () => {
          closeKey.clear();
          setConfirmOpen(false);
        },
        onError: (e) => {
          if (requestWasAnswered(e)) closeKey.clear();
          ctx.setActionError(e.message);
          setConfirmOpen(false);
        },
      },
    );
  };

  const failedRow = (label: string, cmd: ProductGitHubCommand): JSX.Element => (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
      <Typography variant="body2" color="error">
        {label} failed{cmd.lastError ? `: ${cmd.lastError}` : ""}
      </Typography>
      <Button
        size="small"
        variant="outlined"
        disabled={busy}
        onClick={() => ctx.retry(cmd.id)}
        sx={{ textTransform: "none" }}
      >
        Retry
      </Button>
    </Box>
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap", alignItems: "center" }}>
        {canMove && (
          <Button
            size="small"
            variant="outlined"
            disabled={busy}
            onClick={doMove}
            sx={{ textTransform: "none" }}
          >
            Back to product team
          </Button>
        )}
        {canClose && (
          <Button
            size="small"
            variant="outlined"
            color="error"
            disabled={busy}
            onClick={() => setConfirmOpen(true)}
            sx={{ textTransform: "none" }}
          >
            Close issue
          </Button>
        )}
        {movePending && (
          <Typography variant="body2" color="text.secondary">
            Move to WOW pending
          </Typography>
        )}
        {closePending && (
          <Typography variant="body2" color="text.secondary">
            Close pending
          </Typography>
        )}
        {showAwaiting && (
          <Typography variant="body2" color="text.secondary">
            Waiting for GitHub to confirm
          </Typography>
        )}
      </Box>
      {moveFailed && failedRow("Move to WOW", moveFailed)}
      {closeFailed && failedRow("Close", closeFailed)}

      <Dialog
        open={confirmOpen && canClose}
        onClose={() => !close.isPending && setConfirmOpen(false)}
        aria-labelledby="close-product-issue-title"
      >
        <DialogTitle id="close-product-issue-title">Close issue</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            Close {item.github.owner}/{item.github.repository}#{item.github.issueNumber} on GitHub?
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            autoFocus
            onClick={() => setConfirmOpen(false)}
            disabled={close.isPending}
            sx={{ textTransform: "none" }}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            color="error"
            onClick={doClose}
            disabled={close.isPending}
            sx={{ textTransform: "none" }}
          >
            Close issue
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}

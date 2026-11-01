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

import { Box, Button, Divider, Paper, Skeleton, Typography } from "@wso2/oxygen-ui";
import { useMemo, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import SemanticChip from "@components/SemanticChip";
import { formatAbsoluteForUser, formatDateOnlyForDisplay } from "@utils/dateTime";
import { useAllocationUpdates } from "../api/useAllocations";
import { updateStateLabel, updateStateRole } from "../utils/allocations";
import type { AllocationUpdate } from "../types";
import PostUpdateForm from "./PostUpdateForm";
import UpdateContent from "./UpdateContent";

function UpdateItem({ update }: { update: AllocationUpdate }): JSX.Element {
  // A cancelled update stays in the list (so paging and the total still add
  // up) but is dimmed, struck through and labelled.
  const cancelled = update.state === "CANCELLED";
  const when = formatAbsoluteForUser(update.publishedDate ?? update.createdOn);
  const cycle = formatDateOnlyForDisplay(update.cycleStartDate);
  return (
    <Paper
      component="li"
      variant="outlined"
      sx={{ p: 2, listStyle: "none", opacity: cancelled ? 0.7 : 1 }}
    >
      <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1, mb: 0.5 }}>
        <Typography
          variant="subtitle2"
          component="h4"
          sx={{ textDecoration: cancelled ? "line-through" : undefined }}
        >
          {update.subject || "(no subject)"}
        </Typography>
        {update.state && (
          <SemanticChip role={updateStateRole(update.state)} label={updateStateLabel(update.state)} />
        )}
      </Box>
      <Typography variant="caption" color="text.secondary" component="div" sx={{ mb: 1 }}>
        {update.author?.name ?? "Unknown author"}
        {when ? ` · ${when}` : ""}
        {cycle ? ` · Cycle week of ${cycle}` : ""}
      </Typography>
      {update.content ? <UpdateContent content={update.content} /> : null}
    </Paper>
  );
}

/** The allocation's updates, newest first with "load more", and the post form. */
export default function AllocationUpdatesSection({ allocationId }: { allocationId: string }): JSX.Element {
  const q = useAllocationUpdates(allocationId);
  const updates = useMemo(() => q.data?.pages.flatMap((p) => p.updates) ?? [], [q.data]);
  const total = q.data?.pages[0]?.total ?? 0;

  return (
    <Box component="section" aria-labelledby="allocation-updates-heading" sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography id="allocation-updates-heading" variant="h6" component="h2">
        Updates
      </Typography>

      <Paper variant="outlined" sx={{ p: 2 }}>
        <PostUpdateForm allocationId={allocationId} />
      </Paper>

      <Divider />

      {q.isLoading ? (
        <Box aria-busy="true" sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          <Skeleton variant="rounded" height={72} />
          <Skeleton variant="rounded" height={72} />
        </Box>
      ) : q.isError ? (
        <QueryErrorState
          message={q.error.message.trim() || "Failed to load updates."}
          error={q.error}
          onRetry={() => void q.refetch()}
        />
      ) : updates.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No updates have been posted yet.
        </Typography>
      ) : (
        <>
          <Box component="ul" aria-label="Updates, newest first" sx={{ m: 0, p: 0, display: "flex", flexDirection: "column", gap: 1.5 }}>
            {updates.map((u) => (
              <UpdateItem key={u.id} update={u} />
            ))}
          </Box>
          <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
            <Typography variant="caption" color="text.secondary">
              Showing {updates.length} of {total}
            </Typography>
            {q.hasNextPage && (
              <Button size="small" onClick={() => void q.fetchNextPage()} disabled={q.isFetchingNextPage}>
                {q.isFetchingNextPage ? "Loading…" : "Load more"}
              </Button>
            )}
          </Box>
        </>
      )}
    </Box>
  );
}

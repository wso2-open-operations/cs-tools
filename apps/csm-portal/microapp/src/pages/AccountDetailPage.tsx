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

import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Card, Chip, Divider, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { accounts } from "@src/services/accounts";
import { formatDateOnly } from "@utils/customers";
import { MetaRow, MetaValue } from "@components/common/MetaRow";
import { ErrorState } from "@components/support/ErrorState";

export default function AccountDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data, isLoading, isError, refetch } = useQuery(accounts.get(id ?? ""));

  if (isLoading) {
    return (
      <Stack gap={2}>
        <Skeleton variant="rounded" height={28} width="60%" />
        <Skeleton variant="rounded" height={260} />
      </Stack>
    );
  }

  if (isError || !data) {
    return <ErrorState onRetry={() => void refetch()} />;
  }

  const a = data;

  return (
    <Stack gap={2}>
      <Stack gap={0.5}>
        <Stack direction="row" alignItems="center" gap={1} flexWrap="wrap">
          <Typography variant="h6">{a.name}</Typography>
          <Chip
            size="small"
            label={a.tier}
            color={a.tier === "enterprise" ? "primary" : "default"}
            variant="outlined"
            sx={{ textTransform: "capitalize" }}
          />
          {a.deactivationDate && <Chip size="small" label="Deactivated" variant="outlined" />}
        </Stack>
        <Typography variant="caption" color="text.secondary" sx={{ fontFamily: "monospace" }}>
          {a.sfId}
        </Typography>
      </Stack>

      <Card variant="outlined" sx={{ p: 2 }}>
        <Stack gap={1.5}>
          <MetaRow label="Region">
            <MetaValue>{a.region ?? "—"}</MetaValue>
          </MetaRow>
          <Divider />
          <MetaRow label="Activated">
            <MetaValue>{formatDateOnly(a.activationDate)}</MetaValue>
          </MetaRow>
          <Divider />
          <MetaRow label="Deactivated">
            <MetaValue>{formatDateOnly(a.deactivationDate)}</MetaValue>
          </MetaRow>
          <Divider />
          <MetaRow label="AI agent">
            <Chip
              size="small"
              variant="outlined"
              color={a.agentEnabled ? "success" : "default"}
              label={a.agentEnabled ? "Enabled" : "Disabled"}
            />
          </MetaRow>
          <Divider />
          <MetaRow label="KB references">
            <Chip
              size="small"
              variant="outlined"
              color={a.kbReferencesEnabled ? "success" : "default"}
              label={a.kbReferencesEnabled ? "Enabled" : "Disabled"}
            />
          </MetaRow>
          <Divider />
          <MetaRow label="Account owner">
            <MetaValue>{a.ownerName || a.ownerId || "—"}</MetaValue>
          </MetaRow>
          <Divider />
          <MetaRow label="Technical owner">
            <MetaValue>{a.technicalOwnerName || a.technicalOwnerId || "—"}</MetaValue>
          </MetaRow>
          <Divider />
          <MetaRow label="Created">
            <MetaValue>{formatDateOnly(a.createdOn)}</MetaValue>
          </MetaRow>
          <Divider />
          <MetaRow label="Updated">
            <MetaValue>{formatDateOnly(a.updatedOn)}</MetaValue>
          </MetaRow>
        </Stack>
      </Card>
    </Stack>
  );
}

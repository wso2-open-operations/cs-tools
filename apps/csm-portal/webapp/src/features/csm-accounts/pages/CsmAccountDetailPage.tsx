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
  Card,
  Chip,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ReactNode } from "react";
import { useNavigate, useParams } from "react-router";
import { useGetAccount } from "@features/csm-accounts/api/useGetAccount";

function formatDate(value?: string | null): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleDateString();
}

function MetaCell({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

function Mono({ children }: { children: ReactNode }): JSX.Element {
  return (
    <Typography variant="body2" sx={{ fontFamily: "monospace", wordBreak: "break-all" }}>
      {children}
    </Typography>
  );
}

function BackButton({ onClick }: { onClick: () => void }): JSX.Element {
  return (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={onClick}
      sx={{ alignSelf: "flex-start" }}
    >
      Back to accounts
    </Button>
  );
}

export default function CsmAccountDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data, isLoading, isError } = useGetAccount(id);

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rectangular" height={32} width={240} />
        <Skeleton variant="rectangular" height={220} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <BackButton onClick={() => navigate("/customers/accounts")} />
        <Typography variant="body1" color="error">
          Could not load account {id}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <BackButton onClick={() => navigate("/customers/accounts")} />
        <Typography variant="h5">Account not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No account with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const a = data;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <BackButton onClick={() => navigate("/customers/accounts")} />

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="h5">{a.name}</Typography>
          <Chip
            size="small"
            label={a.tier}
            color={a.tier === "enterprise" ? "primary" : "default"}
            variant="outlined"
          />
          {a.deactivationDate && (
            <Chip size="small" label="Deactivated" color="default" variant="outlined" />
          )}
        </Box>
        <Typography variant="body2" color="text.secondary" sx={{ fontFamily: "monospace" }}>
          {a.sfId}
        </Typography>
      </Box>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Overview</Typography>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              md: "repeat(3, minmax(0, 1fr))",
            },
          }}
        >
          <MetaCell label="Tier">
            <Typography variant="body2" sx={{ textTransform: "capitalize" }}>
              {a.tier}
            </Typography>
          </MetaCell>
          <MetaCell label="Region">
            <Typography variant="body2">{a.region ?? "—"}</Typography>
          </MetaCell>
          <MetaCell label="Salesforce ID">
            <Mono>{a.sfId || "—"}</Mono>
          </MetaCell>
          <MetaCell label="Activated">
            <Typography variant="body2">{formatDate(a.activationDate)}</Typography>
          </MetaCell>
          <MetaCell label="Deactivated">
            <Typography variant="body2">{formatDate(a.deactivationDate)}</Typography>
          </MetaCell>
          <MetaCell label="AI agent">
            <Chip
              size="small"
              variant="outlined"
              color={a.agentEnabled ? "success" : "default"}
              label={a.agentEnabled ? "Enabled" : "Disabled"}
            />
          </MetaCell>
          <MetaCell label="KB references">
            <Chip
              size="small"
              variant="outlined"
              color={a.kbReferencesEnabled ? "success" : "default"}
              label={a.kbReferencesEnabled ? "Enabled" : "Disabled"}
            />
          </MetaCell>
          <MetaCell label="Owner ID">
            <Mono>{a.ownerId || "—"}</Mono>
          </MetaCell>
          <MetaCell label="Technical owner ID">
            <Mono>{a.technicalOwnerId || "—"}</Mono>
          </MetaCell>
          <MetaCell label="Created">
            <Typography variant="body2">{formatDate(a.createdAt)}</Typography>
          </MetaCell>
          <MetaCell label="Last updated">
            <Typography variant="body2">{formatDate(a.updatedAt)}</Typography>
          </MetaCell>
          <MetaCell label="Account ID">
            <Mono>{a.id}</Mono>
          </MetaCell>
        </Box>
      </Card>
    </Box>
  );
}

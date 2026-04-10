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
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { useState, useMemo, type JSX } from "react";
import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  Grid,
  IconButton,
  InputAdornment,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Paper,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import {
  AlertCircle,
  CheckCircle,
  Info,
  KeyRound,
  Monitor,
  MoreVertical,
  RefreshCw,
  Search,
  Server,
  Trash2,
  User,
} from "@wso2/oxygen-ui-icons-react";
import TabBar from "@components/common/tab-bar/TabBar";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import GenerateTokenModal from "@components/settings/GenerateTokenModal";
import DeleteTokenModal from "@components/settings/DeleteTokenModal";
import RegenerateTokenModal from "@components/settings/RegenerateTokenModal";
import { useSearchRegistryTokens } from "@api/useSearchRegistryTokens";
import type { RegistryToken } from "@models/responses";

/** Placeholder for empty/null values. */
const DASH = "--";

export interface SettingsRegistryTokensProps {
  projectId: string;
  isAdmin: boolean;
}

/** Derive token status from its fields. */
function getTokenStatus(token: RegistryToken): "Active" | "Expired" | "Revoked" {
  if (token.disable) return "Revoked";
  if (token.expiresAt && token.expiresAt > 0) {
    const nowSec = Math.floor(Date.now() / 1000);
    if (token.expiresAt < nowSec) return "Expired";
  }
  return "Active";
}

/** Chip color for the token status. */
function getStatusColor(
  status: string,
): "success" | "error" | "default" | "warning" {
  switch (status) {
    case "Active":
      return "success";
    case "Expired":
      return "error";
    case "Revoked":
      return "default";
    default:
      return "warning";
  }
}

/** Format a unix-seconds timestamp to DD/MM/YYYY. */
function formatTimestamp(ts?: number): string {
  if (!ts || ts <= 0) return "Never";
  const d = new Date(ts * 1000);
  return d.toLocaleDateString("en-GB", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

/** Format ISO date string to DD/MM/YYYY. */
function formatDate(iso?: string): string {
  if (!iso) return DASH;
  const d = new Date(iso);
  if (isNaN(d.getTime())) return DASH;
  return d.toLocaleDateString("en-GB", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

/** Check if a token expires within the next N days. */
function expiresWithinDays(token: RegistryToken, days: number): boolean {
  if (!token.expiresAt || token.expiresAt <= 0) return false;
  if (token.disable) return false;
  const nowSec = Math.floor(Date.now() / 1000);
  if (token.expiresAt < nowSec) return false;
  const thresholdSec = nowSec + days * 86400;
  return token.expiresAt <= thresholdSec;
}

/**
 * Registry Tokens settings tab: stat cards, sub-tabs (User/Service), search, table.
 *
 * @param {SettingsRegistryTokensProps} props - Component props.
 * @returns {JSX.Element} The component.
 */
export default function SettingsRegistryTokens({
  projectId,
  isAdmin,
}: SettingsRegistryTokensProps): JSX.Element {
  const theme = useTheme();
  const [searchQuery, setSearchQuery] = useState("");
  const [tokenTab, setTokenTab] = useState<string>("user");

  const [generateModalOpen, setGenerateModalOpen] = useState(false);
  const [deleteToken, setDeleteToken] = useState<RegistryToken | null>(null);
  const [regenerateToken, setRegenerateToken] = useState<RegistryToken | null>(null);

  const [menuAnchor, setMenuAnchor] = useState<null | HTMLElement>(null);
  const [menuToken, setMenuToken] = useState<RegistryToken | null>(null);

  const {
    data: allTokens = [],
    isLoading,
    error,
  } = useSearchRegistryTokens(projectId);

  const userTokens = useMemo(
    () => allTokens.filter((t) => t.tokenType === "User"),
    [allTokens],
  );
  const serviceTokens = useMemo(
    () => allTokens.filter((t) => t.tokenType === "Service"),
    [allTokens],
  );

  const stats = useMemo(() => {
    const active = allTokens.filter((t) => getTokenStatus(t) === "Active").length;
    const revokedOrExpired = allTokens.filter(
      (t) => getTokenStatus(t) === "Expired" || getTokenStatus(t) === "Revoked",
    ).length;
    const expiringSoon = allTokens.filter((t) => expiresWithinDays(t, 7)).length;
    return {
      total: allTokens.length,
      active,
      revokedOrExpired,
      expiringSoon,
    };
  }, [allTokens]);

  const subTabs = useMemo(() => {
    const tabs = [
      {
        id: "user",
        label: `User Tokens (${userTokens.length})`,
        icon: User,
      },
    ];
    if (isAdmin) {
      tabs.push({
        id: "service",
        label: `Service Tokens (${serviceTokens.length})`,
        icon: Server,
      });
    }
    return tabs;
  }, [userTokens.length, serviceTokens.length, isAdmin]);

  const displayTokenTab = useMemo(
    () => (subTabs.some((t) => t.id === tokenTab) ? tokenTab : "user"),
    [subTabs, tokenTab],
  );

  const activeTokens = displayTokenTab === "user" ? userTokens : serviceTokens;

  const filteredTokens = useMemo(() => {
    if (!searchQuery.trim()) return activeTokens;
    const q = searchQuery.toLowerCase();
    return activeTokens.filter(
      (t) =>
        t.name?.toLowerCase().includes(q) ||
        t.displayName?.toLowerCase().includes(q) ||
        t.description?.toLowerCase().includes(q) ||
        t.createdFor?.toLowerCase().includes(q) ||
        t.createdBy?.toLowerCase().includes(q),
    );
  }, [activeTokens, searchQuery]);

  const statCards = [
    {
      value: stats.total,
      label: "Total Tokens",
      icon: KeyRound,
      color: theme.palette.warning.main,
      bgColor: alpha(theme.palette.warning.main, 0.08),
      borderColor: alpha(theme.palette.warning.main, 0.25),
    },
    {
      value: stats.active,
      label: "Active Tokens",
      subLabel: `${stats.revokedOrExpired} revoked/expired`,
      icon: CheckCircle,
      color: theme.palette.success.main,
      bgColor: alpha(theme.palette.success.main, 0.08),
      borderColor: alpha(theme.palette.success.main, 0.25),
    },
    {
      value: stats.expiringSoon,
      label: "Expiring Soon",
      subLabel: "Next 7 days",
      icon: AlertCircle,
      color: theme.palette.error.main,
      bgColor: alpha(theme.palette.error.main, 0.08),
      borderColor: alpha(theme.palette.error.main, 0.25),
    },
  ];

  const skeletonRows = (colCount: number) =>
    Array.from({ length: 3 }).map((_, i) => (
      <TableRow key={i}>
        {Array.from({ length: colCount }).map((__, j) => (
          <TableCell key={j}>
            <Skeleton variant="text" width="80%" />
          </TableCell>
        ))}
      </TableRow>
    ));

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {/* Admin alert */}
      {isAdmin && (
        <Alert
          severity="info"
          icon={<Info size={18} />}
          sx={{
            borderRadius: 1,
            "& .MuiAlert-message": { fontWeight: 500 },
          }}
        >
          <strong>Admin View:</strong>&nbsp; You can view and manage all tokens
          across the organization.
        </Alert>
      )}

      {/* Stat cards */}
      <Grid container spacing={2}>
        {statCards.map((card) => {
          const Icon = card.icon;
          return (
            <Grid key={card.label} size={{ xs: 12, sm: 4 }}>
              <Card
                sx={{
                  p: 2,
                  height: "100%",
                  border: "1px solid",
                  borderColor: card.borderColor,
                  bgcolor: card.bgColor,
                }}
              >
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1.5,
                  }}
                >
                  <Box
                    sx={{
                      width: 44,
                      height: 44,
                      borderRadius: "50%",
                      bgcolor: alpha(card.color, 0.15),
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      color: card.color,
                    }}
                  >
                    {isLoading ? (
                      <Skeleton variant="circular" width={22} height={22} />
                    ) : (
                      <Icon size={22} />
                    )}
                  </Box>
                  <Box>
                    <Typography variant="h4" sx={{ lineHeight: 1.2 }}>
                      {isLoading ? (
                        <Skeleton variant="text" width={28} height={32} />
                      ) : error ? (
                        DASH
                      ) : (
                        card.value
                      )}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {isLoading ? (
                        <Skeleton variant="text" width={80} height={16} />
                      ) : (
                        card.label
                      )}
                    </Typography>
                    {card.subLabel && (
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{ display: "block" }}
                      >
                        {isLoading ? (
                          <Skeleton variant="text" width={100} height={14} />
                        ) : (
                          card.subLabel
                        )}
                      </Typography>
                    )}
                  </Box>
                </Box>
              </Card>
            </Grid>
          );
        })}
      </Grid>

      {/* Header + description */}
      <Box>
        <Typography variant="h6" sx={{ mb: 0.5 }}>
          Registry Tokens
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Manage registry tokens for WSO2 Updates 2.0. User tokens are for
          individual access, while service tokens are for automation and CI/CD
          pipelines.
        </Typography>
      </Box>

      {/* Sub-tabs: User Tokens / Service Tokens */}
      <TabBar
        tabs={subTabs}
        activeTab={displayTokenTab}
        onTabChange={setTokenTab}
        sx={{ mb: 0 }}
      />

      {/* Search + Generate button */}
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", sm: "row" },
          gap: 1.5,
        }}
      >
        <TextField
          fullWidth
          size="small"
          placeholder={
            displayTokenTab === "user"
              ? "Search by user, token name..."
              : "Search by token name, description..."
          }
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <Search size={18} color={theme.palette.text.secondary} />
              </InputAdornment>
            ),
          }}
        />
        {/* Generate button: user token for all, service token for admins only */}
        <Button
          variant="contained"
          color="warning"
          startIcon={
            displayTokenTab === "user" ? (
              <KeyRound size={18} />
            ) : (
              <Monitor size={18} />
            )
          }
          sx={{ whiteSpace: "nowrap", pl: 3, pr: 3 }}
          onClick={() => setGenerateModalOpen(true)}
        >
          {displayTokenTab === "user"
            ? "Generate User Token"
            : "Generate Service Token"}
        </Button>
      </Box>

      {/* User Tokens Table */}
      {displayTokenTab === "user" && (
        <TableContainer component={Paper} variant="outlined">
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Token Name</TableCell>
                <TableCell>User</TableCell>
                <TableCell>Created</TableCell>
                <TableCell>Last Used</TableCell>
                <TableCell>Expires</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Usage</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                skeletonRows(8)
              ) : error ? (
                <TableRow>
                  <TableCell colSpan={8} align="center" sx={{ py: 3 }}>
                    <ErrorIndicator
                      entityName="registry tokens"
                      size="medium"
                    />
                  </TableCell>
                </TableRow>
              ) : filteredTokens.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} align="center" sx={{ py: 3 }}>
                    <Typography variant="body2" color="text.secondary">
                      No user tokens found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                filteredTokens.map((token) => {
                  const status = getTokenStatus(token);
                  return (
                    <TableRow key={token.id ?? token.name} hover>
                      <TableCell>
                        <Box>
                          <Typography variant="body2" fontWeight={500}>
                            {token.displayName ?? token.name}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Box>
                          <Typography variant="body2">
                            {token.createdFor ?? DASH}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {formatDate(token.creationTime)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">{DASH}</Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {formatTimestamp(token.expiresAt)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          label={status}
                          color={getStatusColor(status)}
                          variant="outlined"
                          sx={{ typography: "caption" }}
                        />
                      </TableCell>
                      <TableCell align="right">
                        <Typography variant="body2">{DASH}</Typography>
                      </TableCell>
                      <TableCell align="right">
                        <IconButton
                          size="small"
                          onClick={(e) => {
                            setMenuAnchor(e.currentTarget);
                            setMenuToken(token);
                          }}
                          aria-label="Token actions"
                        >
                          <MoreVertical size={18} />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      {/* Service Tokens Table */}
      {displayTokenTab === "service" && isAdmin && (
        <TableContainer component={Paper} variant="outlined">
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Token Name</TableCell>
                <TableCell>Description</TableCell>
                <TableCell>Created By</TableCell>
                <TableCell>Last Used</TableCell>
                <TableCell>Expires</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Usage</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                skeletonRows(8)
              ) : error ? (
                <TableRow>
                  <TableCell colSpan={8} align="center" sx={{ py: 3 }}>
                    <ErrorIndicator
                      entityName="registry tokens"
                      size="medium"
                    />
                  </TableCell>
                </TableRow>
              ) : filteredTokens.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} align="center" sx={{ py: 3 }}>
                    <Typography variant="body2" color="text.secondary">
                      No service tokens found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                filteredTokens.map((token) => {
                  const status = getTokenStatus(token);
                  return (
                    <TableRow key={token.id ?? token.name} hover>
                      <TableCell>
                        <Typography variant="body2" fontWeight={500}>
                          {token.displayName ?? token.name}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {token.description ?? DASH}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Box>
                          <Typography variant="body2">
                            {token.createdBy ?? DASH}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">{DASH}</Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {formatTimestamp(token.expiresAt)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          label={status}
                          color={getStatusColor(status)}
                          variant="outlined"
                          sx={{ typography: "caption" }}
                        />
                      </TableCell>
                      <TableCell align="right">
                        <Typography variant="body2">{DASH}</Typography>
                      </TableCell>
                      <TableCell align="right">
                        <IconButton
                          size="small"
                          onClick={(e) => {
                            setMenuAnchor(e.currentTarget);
                            setMenuToken(token);
                          }}
                          aria-label="Token actions"
                        >
                          <MoreVertical size={18} />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      {/* Shared actions menu */}
      <Menu
        anchorEl={menuAnchor}
        open={Boolean(menuAnchor)}
        onClose={() => {
          setMenuAnchor(null);
          setMenuToken(null);
        }}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        transformOrigin={{ vertical: "top", horizontal: "right" }}
      >
        <MenuItem
          onClick={() => {
            setRegenerateToken(menuToken);
            setMenuAnchor(null);
            setMenuToken(null);
          }}
        >
          <ListItemIcon>
            <RefreshCw size={16} />
          </ListItemIcon>
          <ListItemText>Regenerate Secret</ListItemText>
        </MenuItem>
        <MenuItem
          onClick={() => {
            setDeleteToken(menuToken);
            setMenuAnchor(null);
            setMenuToken(null);
          }}
          sx={{ color: "error.main" }}
        >
          <ListItemIcon sx={{ color: "error.main" }}>
            <Trash2 size={16} />
          </ListItemIcon>
          <ListItemText>Delete Token</ListItemText>
        </MenuItem>
      </Menu>

      {/* Modals */}
      <GenerateTokenModal
        open={generateModalOpen}
        onClose={() => setGenerateModalOpen(false)}
        projectId={projectId}
        tokenType={displayTokenTab === "service" ? "Service" : "User"}
        isAdmin={isAdmin}
      />

      <DeleteTokenModal
        open={deleteToken !== null}
        onClose={() => setDeleteToken(null)}
        projectId={projectId}
        token={deleteToken}
      />

      <RegenerateTokenModal
        open={regenerateToken !== null}
        onClose={() => setRegenerateToken(null)}
        projectId={projectId}
        token={regenerateToken}
      />
    </Box>
  );
}

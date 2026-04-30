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
  Box,
  Button,
  Chip,
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
  Tooltip,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import {
  KeyRound,
  Monitor,
  MoreVertical,
  RefreshCw,
  Search,
  Server,
  Trash2,
  User,
} from "@wso2/oxygen-ui-icons-react";
import TabBar from "@components/tab-bar/TabBar";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import GenerateTokenModal from "./GenerateTokenModal";
import DeleteTokenModal from "./DeleteTokenModal";
import RegenerateTokenModal from "./RegenerateTokenModal";
import { useSearchRegistryTokens } from "@features/settings/api/useSearchRegistryTokens";
import { type RegistryToken, RegistryTokenType } from "@features/settings/types/registryTokens";
import {
  NULL_PLACEHOLDER,
  REGISTRY_GENERATE_SERVICE_TOKEN,
  REGISTRY_GENERATE_USER_TOKEN,
  REGISTRY_MENU_DELETE,
  REGISTRY_MENU_REGENERATE,
  REGISTRY_PAGE_DESCRIPTION,
  REGISTRY_PAGE_TITLE,
  REGISTRY_SEARCH_PLACEHOLDER_SERVICE,
  REGISTRY_SEARCH_PLACEHOLDER_USER,
  REGISTRY_SERVICE_TOKENS_EMPTY,
  REGISTRY_SUBTAB_SERVICE_BASE,
  REGISTRY_SUBTAB_USER_BASE,
  REGISTRY_USER_TOKENS_EMPTY,
} from "@features/settings/constants/settingsConstants";
import {
  RegistryTokenSubTabId,
  type SettingsRegistryTokensProps,
} from "@features/settings/types/settings";
import {
  formatRegistrySubTabLabel,
  formatRegistryTokenIsoDate,
  formatRegistryTokenTimestamp,
  getRegistryTokenDisplayStatus,
  getRegistryTokenStatusChipColor,
} from "@features/settings/utils/registryTokens";
import { resolveRegistryTokenSubTabId } from "@features/settings/utils/settingsPage";

/**
 * Registry Tokens settings tab: sub-tabs (User/Service), search, table.
 *
 * @param {SettingsRegistryTokensProps} props - Component props.
 * @returns {JSX.Element} The component.
 */
export default function SettingsRegistryTokens({
  projectId,
  isAdmin,
  isRestricted = false,
}: SettingsRegistryTokensProps): JSX.Element {
  const theme = useTheme();
  const [searchQuery, setSearchQuery] = useState("");
  const [tokenTab, setTokenTab] = useState<string>(
    RegistryTokenSubTabId.USER,
  );

  const [generateModalOpen, setGenerateModalOpen] = useState(false);
  const [deleteToken, setDeleteToken] = useState<RegistryToken | null>(null);
  const [regenerateToken, setRegenerateToken] = useState<RegistryToken | null>(
    null,
  );

  const [menuAnchor, setMenuAnchor] = useState<null | HTMLElement>(null);
  const [menuToken, setMenuToken] = useState<RegistryToken | null>(null);

  const {
    data: allTokens = [],
    isLoading,
    error,
  } = useSearchRegistryTokens(projectId);
  const isTableLoading = isLoading;
  const userTokens = useMemo(
    () => allTokens.filter((t) => t.tokenType === RegistryTokenType.USER),
    [allTokens],
  );
  const serviceTokens = useMemo(
    () => allTokens.filter((t) => t.tokenType === RegistryTokenType.SERVICE),
    [allTokens],
  );

  const subTabs = useMemo(() => {
    const tabs = [
      {
        id: RegistryTokenSubTabId.USER,
        label: formatRegistrySubTabLabel(
          REGISTRY_SUBTAB_USER_BASE,
          userTokens.length,
        ),
        icon: User,
      },
    ];
    if (isAdmin) {
      tabs.push({
        id: RegistryTokenSubTabId.SERVICE,
        label: formatRegistrySubTabLabel(
          REGISTRY_SUBTAB_SERVICE_BASE,
          serviceTokens.length,
        ),
        icon: Server,
      });
    }
    return tabs;
  }, [userTokens.length, serviceTokens.length, isAdmin]);

  const displayTokenTab = useMemo(
    () => resolveRegistryTokenSubTabId(tokenTab, isAdmin),
    [tokenTab, isAdmin],
  );

  const activeTokens =
    displayTokenTab === RegistryTokenSubTabId.USER
      ? userTokens
      : serviceTokens;

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
  const restrictedTooltip = "You do not have permission to perform this action.";

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {/* Header + description */}
      <Box>
        <Typography variant="h6" sx={{ mb: 0.5 }}>
          {REGISTRY_PAGE_TITLE}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {REGISTRY_PAGE_DESCRIPTION}
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
            displayTokenTab === RegistryTokenSubTabId.USER
              ? REGISTRY_SEARCH_PLACEHOLDER_USER
              : REGISTRY_SEARCH_PLACEHOLDER_SERVICE
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
        <Tooltip
          title={isRestricted ? restrictedTooltip : ""}
          disableHoverListener={!isRestricted}
        >
          <span>
            <Button
              variant="contained"
              color="warning"
              startIcon={
                displayTokenTab === RegistryTokenSubTabId.USER ? (
                  <KeyRound size={18} />
                ) : (
                  <Monitor size={18} />
                )
              }
              sx={{ whiteSpace: "nowrap", pl: 3, pr: 3 }}
              onClick={() => setGenerateModalOpen(true)}
              disabled={isRestricted}
            >
              {displayTokenTab === RegistryTokenSubTabId.USER
                ? REGISTRY_GENERATE_USER_TOKEN
                : REGISTRY_GENERATE_SERVICE_TOKEN}
            </Button>
          </span>
        </Tooltip>
      </Box>

      {/* User Tokens Table */}
      {displayTokenTab === RegistryTokenSubTabId.USER && (
        <TableContainer component={Paper} variant="outlined">
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Token Name</TableCell>
                <TableCell>Subscription End Date</TableCell>
                <TableCell>Products</TableCell>
                {isAdmin && <TableCell>Token Type</TableCell>}
                {isAdmin && <TableCell>Created For</TableCell>}
                {isAdmin && <TableCell>Created By</TableCell>}
                <TableCell>Created On</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isTableLoading ? (
                skeletonRows(isAdmin ? 9 : 6)
              ) : error ? (
                <TableRow>
                  <TableCell colSpan={isAdmin ? 9 : 6} align="center" sx={{ py: 3 }}>
                    <ErrorIndicator
                      entityName="registry tokens"
                      size="medium"
                    />
                  </TableCell>
                </TableRow>
              ) : filteredTokens.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={isAdmin ? 9 : 6} align="center" sx={{ py: 3 }}>
                    <Typography variant="body2" color="text.secondary">
                      {REGISTRY_USER_TOKENS_EMPTY}
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                filteredTokens.map((token) => {
                  const status = getRegistryTokenDisplayStatus(token);
                  return (
                    <TableRow key={token.id ?? token.name} hover>
                      <TableCell>
                        <Typography variant="body2" fontWeight={500}>
                          {token.displayName ?? token.name}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {formatRegistryTokenTimestamp(token.expiresAt)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {token.permissions?.map((p) => p.namespace).join(", ") || NULL_PLACEHOLDER}
                        </Typography>
                      </TableCell>
                      {isAdmin && (
                        <TableCell>
                          <Chip
                            size="small"
                            icon={token.tokenType === RegistryTokenType.SERVICE ? <Server size={12} /> : <User size={12} />}
                            label={token.tokenType ?? NULL_PLACEHOLDER}
                            variant="outlined"
                            color={token.tokenType === RegistryTokenType.SERVICE ? "error" : "info"}
                            sx={{ typography: "caption", "& .MuiChip-icon": { ml: 0.75, mr: 0.5 }, "& .MuiChip-label": { pl: 0.5 } }}
                          />
                        </TableCell>
                      )}
                      {isAdmin && (
                        <TableCell>
                          <Typography variant="body2">
                            {token.createdFor ?? NULL_PLACEHOLDER}
                          </Typography>
                        </TableCell>
                      )}
                      {isAdmin && (
                        <TableCell>
                          <Typography variant="body2">
                            {token.createdBy ?? NULL_PLACEHOLDER}
                          </Typography>
                        </TableCell>
                      )}
                      <TableCell>
                        <Typography variant="body2">
                          {formatRegistryTokenIsoDate(token.creationTime ?? token.createdOn)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          label={status}
                          color={getRegistryTokenStatusChipColor(status)}
                          variant="outlined"
                          sx={{ typography: "caption" }}
                        />
                      </TableCell>
                      <TableCell align="right">
                        <Tooltip
                          title={isRestricted ? restrictedTooltip : ""}
                          disableHoverListener={!isRestricted}
                        >
                          <span>
                            <IconButton
                              size="small"
                              onClick={(e) => {
                                setMenuAnchor(e.currentTarget);
                                setMenuToken(token);
                              }}
                              aria-label="Token actions"
                              disabled={isRestricted}
                            >
                              <MoreVertical size={18} />
                            </IconButton>
                          </span>
                        </Tooltip>
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
      {displayTokenTab === RegistryTokenSubTabId.SERVICE && isAdmin && (
        <TableContainer component={Paper} variant="outlined">
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Token Name</TableCell>
                <TableCell>Subscription End Date</TableCell>
                <TableCell>Products</TableCell>
                <TableCell>Token Type</TableCell>
                <TableCell>Created For</TableCell>
                <TableCell>Created By</TableCell>
                <TableCell>Created On</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isTableLoading ? (
                skeletonRows(9)
              ) : error ? (
                <TableRow>
                  <TableCell colSpan={9} align="center" sx={{ py: 3 }}>
                    <ErrorIndicator
                      entityName="registry tokens"
                      size="medium"
                    />
                  </TableCell>
                </TableRow>
              ) : filteredTokens.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={9} align="center" sx={{ py: 3 }}>
                    <Typography variant="body2" color="text.secondary">
                      {REGISTRY_SERVICE_TOKENS_EMPTY}
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                filteredTokens.map((token) => {
                  const status = getRegistryTokenDisplayStatus(token);
                  return (
                    <TableRow key={token.id ?? token.name} hover>
                      <TableCell>
                        <Typography variant="body2" fontWeight={500}>
                          {token.displayName ?? token.name}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {formatRegistryTokenTimestamp(token.expiresAt)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {token.permissions?.map((p) => p.namespace).join(", ") || NULL_PLACEHOLDER}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          icon={token.tokenType === RegistryTokenType.SERVICE ? <Server size={12} /> : <User size={12} />}
                          label={token.tokenType ?? NULL_PLACEHOLDER}
                          variant="outlined"
                          color={token.tokenType === RegistryTokenType.SERVICE ? "error" : "info"}
                          sx={{ typography: "caption", "& .MuiChip-icon": { ml: 0.75, mr: 0.5 }, "& .MuiChip-label": { pl: 0.5 } }}
                        />
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {token.createdFor ?? NULL_PLACEHOLDER}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {token.createdBy ?? NULL_PLACEHOLDER}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {formatRegistryTokenIsoDate(token.creationTime ?? token.createdOn)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          label={status}
                          color={getRegistryTokenStatusChipColor(status)}
                          variant="outlined"
                          sx={{ typography: "caption" }}
                        />
                      </TableCell>
                      <TableCell align="right">
                        <Tooltip
                          title={isRestricted ? restrictedTooltip : ""}
                          disableHoverListener={!isRestricted}
                        >
                          <span>
                            <IconButton
                              size="small"
                              onClick={(e) => {
                                setMenuAnchor(e.currentTarget);
                                setMenuToken(token);
                              }}
                              aria-label="Token actions"
                              disabled={isRestricted}
                            >
                              <MoreVertical size={18} />
                            </IconButton>
                          </span>
                        </Tooltip>
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
        <Tooltip
          title={isRestricted ? restrictedTooltip : ""}
          disableHoverListener={!isRestricted}
        >
          <span>
            <MenuItem
              disabled={isRestricted}
              onClick={() => {
                if (isRestricted) return;
                setRegenerateToken(menuToken);
                setMenuAnchor(null);
                setMenuToken(null);
              }}
            >
              <ListItemIcon>
                <RefreshCw size={16} />
              </ListItemIcon>
              <ListItemText>{REGISTRY_MENU_REGENERATE}</ListItemText>
            </MenuItem>
          </span>
        </Tooltip>
        <Tooltip
          title={isRestricted ? restrictedTooltip : ""}
          disableHoverListener={!isRestricted}
        >
          <span>
            <MenuItem
              disabled={isRestricted}
              onClick={() => {
                if (isRestricted) return;
                setDeleteToken(menuToken);
                setMenuAnchor(null);
                setMenuToken(null);
              }}
              sx={{ color: "error.main" }}
            >
              <ListItemIcon sx={{ color: "error.main" }}>
                <Trash2 size={16} />
              </ListItemIcon>
              <ListItemText>{REGISTRY_MENU_DELETE}</ListItemText>
            </MenuItem>
          </span>
        </Tooltip>
      </Menu>

      {/* Modals */}
      <GenerateTokenModal
        open={generateModalOpen}
        onClose={() => setGenerateModalOpen(false)}
        projectId={projectId}
        tokenType={
          displayTokenTab === RegistryTokenSubTabId.SERVICE
            ? RegistryTokenType.SERVICE
            : RegistryTokenType.USER
        }
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

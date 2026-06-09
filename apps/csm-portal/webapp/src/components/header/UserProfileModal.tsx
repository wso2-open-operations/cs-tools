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
  Avatar,
  Box,
  Button,
  Chip,
  Dialog,
  IconButton,
  Skeleton,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { PencilLine, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useCallback, useEffect, useState } from "react";
import { useGetUsersMe } from "@features/settings/api/useGetUsersMe";
import { usePatchUsersMe } from "@features/settings/api/usePatchUsersMe";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { initialsOf, resolveUserInfo } from "@utils/userClaims";

// E.164 phone validator: leading + required, then country code digit + 7-14 more digits.
const E164 = /^\+[1-9]\d{7,14}$/;

// Show at most this many group chips inline before collapsing with "+N more".
const GROUPS_PREVIEW_LIMIT = 4;

function formatLastPasswordUpdate(epochMs: string | undefined): string {
  if (!epochMs) return "Not available";
  const ms = parseInt(epochMs, 10);
  if (Number.isNaN(ms)) return "Not available";
  return new Date(ms).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "2-digit",
  });
}

export interface UserProfileModalProps {
  open: boolean;
  onClose: () => void;
}

export default function UserProfileModal({
  open,
  onClose,
}: UserProfileModalProps): JSX.Element {
  const claims = useIdTokenClaims();
  const { data: userMe, isLoading } = useGetUsersMe();
  const patchUserMe = usePatchUsersMe();
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();

  const [phoneDraft, setPhoneDraft] = useState("");
  const [isPhoneEditing, setIsPhoneEditing] = useState(false);

  const info = resolveUserInfo(claims);
  const email = userMe?.email || info.email || "—";
  const initials = initialsOf(info.fullName);

  useEffect(() => {
    if (open && userMe) {
      // Seed the editable draft from the latest server value each time the
      // dialog opens (the accepted "reset form state on open" pattern).
      // eslint-disable-next-line react-hooks/set-state-in-effect -- form reset on dialog open
      setPhoneDraft(userMe.phoneNumber ?? "");
      setIsPhoneEditing(false);
    }
  }, [open, userMe]);

  const handlePhoneEditCancel = useCallback(() => {
    setPhoneDraft(userMe?.phoneNumber ?? "");
    setIsPhoneEditing(false);
  }, [userMe?.phoneNumber]);

  const handleSave = useCallback(() => {
    const current = userMe?.phoneNumber ?? "";
    const next = phoneDraft.trim();

    if (next === current) {
      onClose();
      return;
    }

    if (next && !E164.test(next)) {
      showError("Phone number must be in E.164 format, e.g. +14155552671.");
      return;
    }

    patchUserMe.mutate(
      { phoneNumber: next },
      {
        onSuccess: () => {
          showSuccess("Profile updated.");
          setIsPhoneEditing(false);
          onClose();
        },
        onError: () => {
          showError("Could not update phone number. Please try again.");
        },
      },
    );
  }, [
    phoneDraft,
    userMe?.phoneNumber,
    patchUserMe,
    showError,
    showSuccess,
    onClose,
  ]);

  const visibleGroups = info.groups.slice(0, GROUPS_PREVIEW_LIMIT);
  const hiddenGroupCount = info.groups.length - visibleGroups.length;

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <Box sx={{ p: 3, display: "flex", flexDirection: "column", gap: 3 }}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <Typography variant="h6">Profile</Typography>
          <IconButton onClick={onClose} size="small" aria-label="Close">
            <X size={18} />
          </IconButton>
        </Box>

        <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
          <Avatar
            src={info.avatarUrl}
            imgProps={{ referrerPolicy: "no-referrer" }}
            sx={{ width: 64, height: 64, fontSize: 24 }}
          >
            {initials}
          </Avatar>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography variant="subtitle1" noWrap>
              {info.fullName}
            </Typography>
            <Typography variant="body2" color="text.secondary" noWrap>
              {email}
            </Typography>
            {info.orgName && (
              <Typography variant="caption" color="text.secondary">
                {info.orgName}
                {info.orgHandle && info.orgHandle !== info.orgName
                  ? ` (${info.orgHandle})`
                  : ""}
              </Typography>
            )}
          </Box>
        </Box>

        {info.groups.length > 0 && (
          <Box>
            <Typography variant="caption" color="text.secondary">
              Groups
            </Typography>
            <Box
              sx={{
                display: "flex",
                flexWrap: "wrap",
                gap: 0.5,
                mt: 0.5,
              }}
            >
              {visibleGroups.map((g) => (
                <Chip key={g} size="small" label={g} variant="outlined" />
              ))}
              {hiddenGroupCount > 0 && (
                <Chip
                  size="small"
                  label={`+${hiddenGroupCount} more`}
                  variant="outlined"
                  color="default"
                />
              )}
            </Box>
          </Box>
        )}

        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box>
            <Typography variant="caption" color="text.secondary">
              Phone number
            </Typography>
            {isPhoneEditing ? (
              <Box sx={{ display: "flex", gap: 1, alignItems: "center", mt: 0.5 }}>
                <TextField
                  size="small"
                  fullWidth
                  placeholder="+14155552671"
                  value={phoneDraft}
                  onChange={(e) => setPhoneDraft(e.target.value)}
                  helperText="E.164 format: + followed by country code and number"
                />
              </Box>
            ) : (
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  mt: 0.5,
                }}
              >
                <Typography variant="body2">
                  {isLoading ? (
                    <Skeleton width={160} />
                  ) : (
                    userMe?.phoneNumber || "Not set"
                  )}
                </Typography>
                <IconButton
                  size="small"
                  onClick={() => setIsPhoneEditing(true)}
                  aria-label="Edit phone number"
                >
                  <PencilLine size={16} />
                </IconButton>
              </Box>
            )}
          </Box>

          <Box>
            <Typography variant="caption" color="text.secondary">
              Last password update
            </Typography>
            <Typography variant="body2" sx={{ mt: 0.5 }}>
              {isLoading ? (
                <Skeleton width={160} />
              ) : (
                formatLastPasswordUpdate(userMe?.lastPasswordUpdateTime)
              )}
            </Typography>
          </Box>
        </Box>

        <Stack direction="row" justifyContent="flex-end" spacing={1}>
          {isPhoneEditing && (
            <Button variant="text" onClick={handlePhoneEditCancel}>
              Cancel
            </Button>
          )}
          <Button
            variant="contained"
            onClick={handleSave}
            disabled={patchUserMe.isPending}
          >
            {isPhoneEditing ? "Save" : "Close"}
          </Button>
        </Stack>
      </Box>
    </Dialog>
  );
}

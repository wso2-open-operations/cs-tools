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

import { type JSX, useState, useEffect, useCallback, useMemo } from "react";
import {
  Avatar,
  Box,
  Button,
  Dialog,
  FormControl,
  IconButton,
  MenuItem,
  Select,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ExternalLink, Lock, PencilLine, X } from "@wso2/oxygen-ui-icons-react";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import useGetMetadata from "@api/useGetMetadata";
import { usePatchUserMe } from "@features/settings/api/usePatchUserMe";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import {
  formatPhoneForDisplay,
  parseE164ToCountryCode,
  PHONE_COUNTRY_OPTIONS,
  toE164FromCountryCode,
  validatePhoneE164,
  type PhoneCountryOption,
} from "@features/settings/utils/phone";
import { resolveDisplayTimeZone } from "@utils/dateTime";

const PASSWORD_RESET_URL = "https://wso2.com/user/password";

/**
 * Formats epoch timestamp to localized date string in user's timezone.
 *
 * @param {string | undefined} epochMs - Epoch timestamp in milliseconds as string.
 * @param {string | undefined} timeZone - IANA timezone string.
 * @returns {string} Formatted date string or "Not Available".
 */
const formatLastPasswordUpdate = (
  epochMs: string | undefined,
  timeZone: string | undefined,
): string => {
  if (!epochMs) return "Not Available";

  try {
    const timestamp = parseInt(epochMs, 10);
    if (isNaN(timestamp)) return "Not Available";

    const date = new Date(timestamp);

    const displayTimeZone = resolveDisplayTimeZone(timeZone);
    return date.toLocaleDateString("en-US", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      timeZone: displayTimeZone,
    });
  } catch {
    return "Not Available";
  }
};

/**
 * Maps role strings to user-friendly labels. Returns highest role when multiple roles present.
 *
 * @param {string[]} roles - Array of role strings from users/me endpoint.
 * @returns {string} The user-friendly role label.
 */
const getRoleLabel = (roles: string[] | undefined): string => {
  if (!roles || roles.length === 0) return "Not Available";
  if (roles.includes("sn_customerservice.customer_admin")) return "Admin";
  return "System User";
};

export interface UserProfileModalProps {
  open: boolean;
  onClose: () => void;
}

/**
 * Large modal displaying user profile with editable phone number and time zone.
 *
 * @param {UserProfileModalProps} props - open, onClose.
 * @returns {JSX.Element} The profile modal.
 */
export default function UserProfileModal({
  open,
  onClose,
}: UserProfileModalProps): JSX.Element {
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const { data: userDetails, isLoading } = useGetUserDetails();
  const { data: metadata } = useGetMetadata();
  const patchUserMe = usePatchUserMe();

  const [phoneNumber, setPhoneNumber] = useState("");
  const [countryCode, setCountryCode] = useState("US");
  const [nationalNumber, setNationalNumber] = useState("");
  const [timeZone, setTimeZone] = useState("");
  const [isPhoneEditing, setIsPhoneEditing] = useState(false);

  // Initialize form state when modal opens
  useEffect(() => {
    if (open && userDetails) {
      const raw = userDetails.phoneNumber ?? "";
      const { countryCode: cc, nationalNumber: nn } =
        parseE164ToCountryCode(raw);
      queueMicrotask(() => {
        setPhoneNumber(raw);
        setCountryCode(cc);
        setNationalNumber(nn);
        setTimeZone(userDetails.timeZone ?? "");
        setIsPhoneEditing(false);
      });
    }
  }, [open, userDetails]);

  const handlePhoneCountryChange = useCallback(
    (e: { target: { value: unknown } }) =>
      setCountryCode(String(e.target.value ?? "US")),
    [],
  );
  const handlePhoneNumberChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) =>
      setNationalNumber(e.target.value.replace(/\D/g, "").slice(0, 15)),
    [],
  );
  const handleTimeZoneChange = useCallback(
    (e: { target: { value: unknown } }) =>
      setTimeZone(String(e.target.value ?? "")),
    [],
  );

  const resetPhoneDraft = useCallback(() => {
    if (userDetails) {
      const raw = userDetails.phoneNumber ?? "";
      const { countryCode: cc, nationalNumber: nn } = parseE164ToCountryCode(raw);
      setCountryCode(cc);
      setNationalNumber(nn);
    }
  }, [userDetails]);

  const handleSave = useCallback(() => {
    if (!userDetails) return;

    const e164 = toE164FromCountryCode(countryCode, nationalNumber);
    const phoneError = validatePhoneE164(e164);
    if (phoneError) {
      showError(phoneError);
      return;
    }

    const payload: { phoneNumber?: string; timeZone?: string } = {};
    if (String(userDetails.phoneNumber ?? "") !== e164) {
      payload.phoneNumber = e164;
    }
    if (String(userDetails.timeZone ?? "") !== timeZone) {
      payload.timeZone = timeZone;
    }

    if (Object.keys(payload).length === 0) {
      onClose();
      return;
    }

    patchUserMe.mutate(payload, {
      onSuccess: (response) => {
        const hasPhoneInPayload = payload.phoneNumber !== undefined;
        const hasTimeZoneInPayload = payload.timeZone !== undefined;
        const isPhoneUpdated =
          !hasPhoneInPayload || response?.phoneNumber === payload.phoneNumber;
        const isTimeZoneUpdated =
          !hasTimeZoneInPayload || response?.timeZone === payload.timeZone;
        const isSuccessfulUpdate = isPhoneUpdated && isTimeZoneUpdated;

        if (!isSuccessfulUpdate) {
          if (hasPhoneInPayload && hasTimeZoneInPayload) {
            showError("Failed to update phone number and time zone.");
          } else if (hasPhoneInPayload) {
            showError("Failed to update phone number.");
          } else if (hasTimeZoneInPayload) {
            showError("Failed to update time zone.");
          } else {
            showError("Failed to update profile.");
          }
          return;
        }

        showSuccess("Profile updated successfully");
        onClose();
      },
      onError: (err) => {
        const msg = err?.message ?? "";
        const hasPhoneInPayload = payload.phoneNumber !== undefined;
        const isValidationError =
          hasPhoneInPayload &&
          (/invalid|validation/i.test(msg) ||
            (msg.includes("400") && /phone|format/i.test(msg)));
        const isGenericPhoneFailure =
          hasPhoneInPayload &&
          /failed to update phone|update phone number/i.test(msg);
        if (isValidationError) {
          showError("Please enter a valid phone number");
        } else if (isGenericPhoneFailure) {
          showError("Something went wrong while updating phone number.");
        } else {
          showError(msg || "Failed to update profile");
        }
      },
    });
  }, [
    userDetails,
    countryCode,
    nationalNumber,
    timeZone,
    patchUserMe,
    showSuccess,
    showError,
    onClose,
  ]);

  const handleClose = useCallback(() => {
    if (!patchUserMe.isPending) {
      onClose();
    }
  }, [onClose, patchUserMe.isPending]);

  const name =
    userDetails?.firstName || userDetails?.lastName
      ? `${userDetails.firstName ?? ""} ${userDetails.lastName ?? ""}`.trim()
      : "--";
  const email = userDetails?.email ?? "--";
  const role = getRoleLabel(userDetails?.roles);

  const initials = (() => {
    const first = userDetails?.firstName?.charAt(0) ?? "";
    const last = userDetails?.lastName?.charAt(0) ?? "";
    return (first + last).toUpperCase() || "?";
  })();

  const timeZoneValue = useMemo(() => {
    if (!timeZone) return "";
    return timeZone;
  }, [timeZone]);
  const timeZoneOptions = useMemo(
    () =>
      (metadata?.timeZones ?? []).filter(
        (tz) => !!tz.id && !!tz.label,
      ),
    [metadata?.timeZones],
  );

  const lastPasswordUpdate = useMemo(() => {
    return formatLastPasswordUpdate(
      userDetails?.lastPasswordUpdateTime,
      userDetails?.timeZone,
    );
  }, [userDetails?.lastPasswordUpdateTime, userDetails?.timeZone]);

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="md"
      fullWidth
      aria-labelledby="profile-dialog-title"
      PaperProps={{
        sx: {
          height: "90vh",
          maxHeight: "90vh",
        },
      }}
    >
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          height: "100%",
          overflow: "hidden",
        }}
      >
        {isLoading ? (
          <Box sx={{ p: 4, display: "flex", flexDirection: "column", gap: 4 }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
              }}
            >
              <Skeleton variant="text" width={150} height={40} />
              <Skeleton variant="circular" width={32} height={32} />
            </Box>
            <Box sx={{ display: "flex", gap: 3, alignItems: "center" }}>
              <Skeleton variant="circular" width={120} height={120} />
              <Box
                sx={{
                  flex: 1,
                  display: "flex",
                  flexDirection: "column",
                  gap: 1,
                }}
              >
                <Skeleton variant="text" width={200} height={32} />
                <Skeleton variant="text" width={100} height={20} />
              </Box>
            </Box>
            <Skeleton variant="rectangular" width="100%" height={300} />
          </Box>
        ) : !userDetails ? (
          <>
            <Box
              sx={{
                p: 3,
                borderBottom: 1,
                borderColor: "divider",
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
              }}
            >
              <Typography id="profile-dialog-title" variant="h5">
                Profile
              </Typography>
              <IconButton
                aria-label="Close profile"
                size="small"
                onClick={handleClose}
              >
                <X size={20} />
              </IconButton>
            </Box>
            <Box
              sx={{
                flex: 1,
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                justifyContent: "center",
                p: 4,
                gap: 2,
              }}
            >
              <Typography variant="h6" color="text.secondary">
                Unable to load profile
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ textAlign: "center" }}>
                We couldn't load your profile information. Please try again.
              </Typography>
              <Box sx={{ display: "flex", gap: 2, mt: 2 }}>
                <Button variant="outlined" onClick={handleClose}>
                  Close
                </Button>
              </Box>
            </Box>
          </>
        ) : (
          <>
            <Box
              sx={{
                p: 3,
                borderBottom: 1,
                borderColor: "divider",
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
              }}
            >
              <Typography id="profile-dialog-title" variant="h5">
                Profile
              </Typography>
              <IconButton
                aria-label="Close profile"
                size="small"
                onClick={handleClose}
                disabled={patchUserMe.isPending}
              >
                <X size={20} />
              </IconButton>
            </Box>

            <Box
              sx={{
                flex: 1,
                overflow: "auto",
                p: 4,
              }}
            >
              <Box sx={{ display: "flex", flexDirection: "column", gap: 4 }}>
                <Box sx={{ display: "flex", gap: 3, alignItems: "center" }}>
                  <Avatar
                    src={userDetails.avatar ?? undefined}
                    sx={{
                      width: 120,
                      height: 120,
                      fontSize: "2rem",
                      borderRadius: "50%",
                    }}
                  >
                    {!userDetails.avatar ? initials : null}
                  </Avatar>
                  <Box sx={{ flex: 1 }}>
                    <Typography variant="h5" sx={{ mb: 0.5 }}>
                      {name}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {role}
                    </Typography>
                  </Box>
                </Box>

                <Box sx={{ p: 3, width: "100%" }}>
                  <Box
                    sx={{ display: "flex", flexDirection: "column", gap: 3 }}
                  >
                    <Box sx={{ display: "flex", gap: 3, flexWrap: "wrap" }}>
                      <Box
                        sx={{
                          display: "flex",
                          flexDirection: "column",
                          gap: 1,
                          flex: "1 1 45%",
                          minWidth: 200,
                        }}
                      >
                        <Typography variant="body2" fontWeight={500}>
                          Full Name
                        </Typography>
                        <TextField
                          value={name}
                          size="small"
                          disabled
                          fullWidth
                          InputProps={{
                            readOnly: true,
                          }}
                        />
                      </Box>
                      <Box
                        sx={{
                          display: "flex",
                          flexDirection: "column",
                          gap: 1,
                          flex: "1 1 45%",
                          minWidth: 200,
                        }}
                      >
                        <Typography variant="body2" fontWeight={500}>
                          Email
                        </Typography>
                        <TextField
                          value={email}
                          size="small"
                          disabled
                          fullWidth
                          InputProps={{
                            readOnly: true,
                          }}
                        />
                      </Box>
                    </Box>
                    <Box
                      sx={{ display: "flex", flexDirection: "column", gap: 1 }}
                    >
                      <Typography variant="body2" fontWeight={500}>
                        Mobile Number
                      </Typography>
                      {isPhoneEditing ? (
                        <Box
                          sx={{
                            display: "flex",
                            flexDirection: "row",
                            alignItems: "center",
                            gap: 2,
                            flexWrap: "wrap",
                          }}
                        >
                          <FormControl
                            size="small"
                            sx={{ minWidth: 200 }}
                            disabled={patchUserMe.isPending}
                          >
                            <Select
                              labelId="country-code-label"
                              id="country-code-select"
                              value={countryCode}
                              onChange={handlePhoneCountryChange}
                              displayEmpty
                              size="small"
                              aria-label="Country code"
                            >
                              {PHONE_COUNTRY_OPTIONS.map((o: PhoneCountryOption) => (
                                <MenuItem
                                  key={o.countryCode}
                                  value={o.countryCode}
                                >
                                  {o.flag} {o.label}
                                </MenuItem>
                              ))}
                            </Select>
                          </FormControl>
                          <TextField
                            id="phone-number-input"
                            placeholder="Phone number"
                            value={nationalNumber}
                            onChange={handlePhoneNumberChange}
                            size="small"
                            disabled={patchUserMe.isPending}
                            sx={{ minWidth: 250, flex: 1 }}
                            inputProps={{ inputMode: "tel", "aria-label": "Phone number" }}
                          />
                          <Button
                            variant="text"
                            size="small"
                            onClick={() => {
                              resetPhoneDraft();
                              setIsPhoneEditing(false);
                            }}
                            disabled={patchUserMe.isPending}
                          >
                            Cancel
                          </Button>
                        </Box>
                      ) : (
                        <Box
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 2,
                          }}
                        >
                          <TextField
                            value={
                              formatPhoneForDisplay(phoneNumber) || "Not set"
                            }
                            size="small"
                            disabled
                            fullWidth
                            InputProps={{
                              readOnly: true,
                            }}
                          />
                          <IconButton
                            size="small"
                            aria-label="Edit phone number"
                            onClick={() => setIsPhoneEditing(true)}
                            disabled={patchUserMe.isPending}
                            sx={{
                              border: 1,
                              borderColor: "divider",
                            }}
                          >
                            <PencilLine size={16} />
                          </IconButton>
                        </Box>
                      )}
                    </Box>
                    <Box sx={{ display: "flex", gap: 3, flexWrap: "wrap" }}>
                      <Box
                        sx={{
                          display: "flex",
                          flexDirection: "column",
                          gap: 1,
                          flex: "1 1 45%",
                          minWidth: 200,
                        }}
                      >
                        <FormControl
                          fullWidth
                          size="small"
                          disabled={patchUserMe.isPending}
                        >
                          <Typography
                            id="timezone-label"
                            variant="body2"
                            fontWeight={500}
                            sx={{ mb: 1 }}
                          >
                            Time Zone
                          </Typography>
                          <Select
                            labelId="timezone-label"
                            id="timezone-select"
                            value={timeZoneValue}
                            onChange={handleTimeZoneChange}
                            displayEmpty
                            aria-label="Time Zone"
                            renderValue={(selected) => {
                              if (!selected) {
                                return (
                                  <Typography
                                    variant="body2"
                                    color="text.secondary"
                                  >
                                    Select timezone
                                  </Typography>
                                );
                              }
                              const option = timeZoneOptions.find(
                                (tz) => tz.id === selected,
                              );
                              return option?.label ?? String(selected);
                            }}
                          >
                            {timeZoneOptions.map((tz) => (
                              <MenuItem key={tz.id} value={tz.id}>
                                {tz.label}
                              </MenuItem>
                            ))}
                          </Select>
                        </FormControl>
                      </Box>
                    </Box>

                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "space-between",
                        p: 2,
                        bgcolor: "background.default",
                        borderRadius: 1,
                        border: 1,
                        borderColor: "divider",
                        mt: 2,
                      }}
                    >
                      <Box
                        sx={{ display: "flex", alignItems: "center", gap: 2 }}
                      >
                        <Box sx={{ p: 1 }}>
                          <Lock size={20} />
                        </Box>
                        <Box>
                          <Typography variant="body2" fontWeight={500}>
                            Password
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            Last changed : {lastPasswordUpdate}
                          </Typography>
                        </Box>
                      </Box>
                      <Button
                        component="a"
                        href={PASSWORD_RESET_URL}
                        target="_blank"
                        rel="noopener noreferrer"
                        variant="outlined"
                        size="small"
                        startIcon={<ExternalLink size={16} />}
                        disabled={patchUserMe.isPending}
                      >
                        Change Password
                      </Button>
                    </Box>
                  </Box>
                </Box>
              </Box>
            </Box>

            <Box
              sx={{
                p: 3,
                borderTop: 1,
                borderColor: "divider",
                display: "flex",
                justifyContent: "flex-end",
                gap: 2,
              }}
            >
              <Button onClick={handleClose} disabled={patchUserMe.isPending}>
                Cancel
              </Button>
              <Button
                variant="contained"
                onClick={handleSave}
                disabled={patchUserMe.isPending || isLoading || !userDetails}
              >
                {patchUserMe.isPending ? "Saving..." : "Save Changes"}
              </Button>
            </Box>
          </>
        )}
      </Box>
    </Dialog>
  );
}

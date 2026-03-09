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

import { type JSX, useState, useEffect, useCallback } from "react";
import {
  Avatar,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { PencilLine, X } from "@wso2/oxygen-ui-icons-react";
import useGetUserDetails from "@api/useGetUserDetails";
import { usePatchUserMe } from "@api/usePatchUserMe";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import {
  formatPhoneForDisplay,
  parseE164ToCountryCode,
  PHONE_COUNTRY_OPTIONS,
  toE164FromCountryCode,
  validatePhoneE164,
} from "@utils/phone";
import { TIME_ZONE_OPTIONS } from "@constants/timeZoneConstants";

export interface UserProfileModalProps {
  open: boolean;
  onClose: () => void;
}

/**
 * Modal displaying user profile from useGetUserDetails (avatar, name, email)
 * with editable phone number and time zone. PATCH /users/me with only changed fields.
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
  const patchUserMe = usePatchUserMe();

  const [phoneNumber, setPhoneNumber] = useState("");
  const [countryCode, setCountryCode] = useState("US");
  const [nationalNumber, setNationalNumber] = useState("");
  const [timeZone, setTimeZone] = useState("");
  const [isPhoneEditing, setIsPhoneEditing] = useState(false);

  useEffect(() => {
    if (open && userDetails) {
      const raw = userDetails.phoneNumber ?? "";
      setPhoneNumber(raw);
      const { countryCode: cc, nationalNumber: nn } = parseE164ToCountryCode(raw);
      setCountryCode(cc);
      setNationalNumber(nn);
      setTimeZone(userDetails.timeZone ?? "");
      setIsPhoneEditing(false);
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
      onSuccess: () => {
        showSuccess("Profile updated successfully");
        onClose();
      },
      onError: (err) => {
        const msg = err?.message ?? "";
        const hasPhoneInPayload = payload.phoneNumber !== undefined;
        const isValidationError =
          hasPhoneInPayload &&
          (/invalid|validation/i.test(msg) || (msg.includes("400") && /phone|format/i.test(msg)));
        const isGenericPhoneFailure =
          hasPhoneInPayload && /failed to update phone|update phone number/i.test(msg);
        if (isValidationError) {
          showError("Please enter a valid phone number");
        } else if (isGenericPhoneFailure) {
          showError("Something went wrong while updating phone number.");
        } else {
          showError(msg || "Failed to update profile");
        }
      },
    });
  }, [userDetails, countryCode, nationalNumber, timeZone, patchUserMe, showSuccess, showError, onClose]);

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

  const initials = (() => {
    const first = userDetails?.firstName?.charAt(0) ?? "";
    const last = userDetails?.lastName?.charAt(0) ?? "";
    return (first + last).toUpperCase() || "?";
  })();

  const PASSWORD_RESET_URL = "https://wso2.com/user/password";

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          pr: 6,
        }}
      >
        <Typography variant="h6">Profile</Typography>
        <IconButton
          aria-label="Close profile"
          size="small"
          onClick={handleClose}
          sx={{ position: "absolute", right: 8, top: 8 }}
        >
          <X size={18} />
        </IconButton>
      </DialogTitle>
      <DialogContent>
        {isLoading || !userDetails ? (
          <Box sx={{ py: 4, textAlign: "center" }}>
            <CircularProgress size={32} />
          </Box>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3, pt: 1 }}>
            <Box
              sx={{
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                gap: 1,
              }}
            >
              <Avatar
                src={userDetails.avatar ?? undefined}
                sx={{
                  width: 80,
                  height: 80,
                  fontSize: "1.5rem",
                  borderRadius: "50%",
                }}
              >
                {!userDetails.avatar ? initials : null}
              </Avatar>
              <Typography variant="h6">{name}</Typography>
              <Typography variant="body2" color="text.secondary">
                {email}
              </Typography>
            </Box>

            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Typography variant="body2" color="text.secondary">
                Phone Number:
              </Typography>
              {isPhoneEditing ? (
                <Box
                  sx={{
                    display: "flex",
                    flexDirection: "row",
                    alignItems: "center",
                    gap: 1,
                    flexWrap: "wrap",
                  }}
                >
                  <FormControl
                    size="small"
                    sx={{ minWidth: 160 }}
                    disabled={patchUserMe.isPending}
                  >
                    <Select
                      value={countryCode}
                      onChange={handlePhoneCountryChange}
                      displayEmpty
                      size="small"
                    >
                      {PHONE_COUNTRY_OPTIONS.map((o) => (
                        <MenuItem key={o.countryCode} value={o.countryCode}>
                          {o.flag} {o.label}
                        </MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                  <TextField
                    placeholder="Phone number"
                    value={nationalNumber}
                    onChange={handlePhoneNumberChange}
                    size="small"
                    disabled={patchUserMe.isPending}
                    sx={{ minWidth: 220, flex: 1 }}
                    inputProps={{ inputMode: "tel" }}
                  />
                </Box>
              ) : (
                <Box
                  sx={{
                    border: "1px solid",
                    borderColor: "divider",
                    borderRadius: 1,
                    py: 0.75,
                    px: 1,
                    backgroundColor: "background.default",
                    display: "flex",
                    flexDirection: "row",
                    alignItems: "center",
                    justifyContent: "space-between",
                    minHeight: 40,
                  }}
                >
                  <Typography variant="body2" color="text.primary">
                    {formatPhoneForDisplay(phoneNumber) || "Not set"}
                  </Typography>
                  <IconButton
                    size="small"
                    aria-label="Edit phone number"
                    onClick={() => setIsPhoneEditing(true)}
                    disabled={patchUserMe.isPending}
                  >
                    <PencilLine size={16} />
                  </IconButton>
                </Box>
              )}
            </Box>

            <FormControl fullWidth size="small" disabled={patchUserMe.isPending}>
              <InputLabel id="profile-timezone-label">Time Zone</InputLabel>
              <Select
                labelId="profile-timezone-label"
                label="Time Zone"
                value={timeZone}
                onChange={handleTimeZoneChange}
              >
                {TIME_ZONE_OPTIONS.map((tz) => (
                  <MenuItem key={tz} value={tz}>
                    {tz}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <Button
              component="a"
              href={PASSWORD_RESET_URL}
              target="_blank"
              rel="noopener noreferrer"
              variant="text"
              color="primary"
              size="small"
              disabled={patchUserMe.isPending}
              sx={{ alignSelf: "flex-start", textTransform: "none", pointerEvents: patchUserMe.isPending ? "none" : "auto" }}
            >
              Reset Password
            </Button>
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={patchUserMe.isPending}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleSave}
          disabled={
            patchUserMe.isPending || isLoading || !userDetails
          }
          startIcon={
            patchUserMe.isPending ? (
              <CircularProgress size={18} sx={{ color: "inherit" }} />
            ) : null
          }
        >
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
}

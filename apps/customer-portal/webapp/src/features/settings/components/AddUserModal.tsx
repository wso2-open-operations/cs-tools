// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License
// at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { useState, useCallback, useEffect, useRef, type JSX } from "react";
import {
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  IconButton,
  Input,
  InputLabel,
  MenuItem,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import { useValidateProjectContact } from "@features/settings/api/useValidateProjectContact";
import {
  ADD_USER_DETAILS_INTRO,
  ADD_USER_EMAIL_INVALID_CONTACT_ERROR,
  ADD_USER_EMAIL_INVALID_ERROR,
  ADD_USER_EMAIL_LABEL,
  ADD_USER_EMAIL_PLACEHOLDER,
  ADD_USER_EMAIL_REGEX,
  ADD_USER_EMAIL_REQUIRED_ERROR,
  ADD_USER_EMAIL_STEP_DESCRIPTION,
  ADD_USER_EMAIL_VALIDATE_ERROR,
  ADD_USER_FIRST_NAME_LABEL,
  ADD_USER_FIRST_NAME_PLACEHOLDER,
  ADD_USER_LAST_NAME_LABEL,
  ADD_USER_LAST_NAME_PLACEHOLDER,
  ADD_USER_MODAL_BACK,
  ADD_USER_MODAL_CANCEL,
  ADD_USER_MODAL_NEXT,
  ADD_USER_MODAL_SENDING,
  ADD_USER_MODAL_SEND_INVITATION,
  ADD_USER_MODAL_TITLE,
  ADD_USER_MODAL_VALIDATING,
  ADD_USER_ROLE_OPTIONS,
  ADD_USER_TYPE_LABEL,
} from "@features/settings/constants/settingsConstants";
import {
  AddUserContactRole,
  AddUserModalStep,
  type AddUserModalProps,
} from "@features/settings/types/settings";

/**
 * Two-step modal to add a new user (contact) to the project.
 * Enter and validate email address.
 * Fill in full name and role, then send invitation.
 *
 * @param {AddUserModalProps} props - Modal props.
 * @returns {JSX.Element} The modal.
 */
export default function AddUserModal({
  open,
  projectId,
  onClose,
  onSubmit,
  isSubmitting = false,
}: AddUserModalProps): JSX.Element {
  const [step, setStep] = useState<AddUserModalStep>(AddUserModalStep.EMAIL);
  const [email, setEmail] = useState("");
  const [emailError, setEmailError] = useState("");
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [role, setRole] = useState<AddUserContactRole>(
    AddUserContactRole.PORTAL_USER,
  );
  const [isExistingContact, setIsExistingContact] = useState(false);
  const emailInputRef = useRef<HTMLInputElement>(null);

  const validateContact = useValidateProjectContact(projectId);

  const resetForm = useCallback(() => {
    setStep(AddUserModalStep.EMAIL);
    setEmail("");
    setEmailError("");
    setFirstName("");
    setLastName("");
    setRole(AddUserContactRole.PORTAL_USER);
    setIsExistingContact(false);
  }, []);

  useEffect(() => {
    if (!open) {
      resetForm();
      validateContact.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, resetForm]);

  const isBusy = isSubmitting || validateContact.isPending;

  const handleClose = useCallback(() => {
    if (isBusy) return;
    resetForm();
    onClose();
  }, [isBusy, resetForm, onClose]);

  const handleEmailNext = useCallback(() => {
    const trimmedEmail = email.trim();
    setEmailError("");

    if (!trimmedEmail) {
      setEmailError(ADD_USER_EMAIL_REQUIRED_ERROR);
      emailInputRef.current?.focus();
      return;
    }
    if (!ADD_USER_EMAIL_REGEX.test(trimmedEmail)) {
      setEmailError(ADD_USER_EMAIL_INVALID_ERROR);
      emailInputRef.current?.focus();
      return;
    }

    validateContact.mutate(
      { contactEmail: trimmedEmail },
      {
        onSuccess: (data) => {
          if (!data.isContactValid) {
            setEmailError(
              data.message || ADD_USER_EMAIL_INVALID_CONTACT_ERROR,
            );
            return;
          }
          // Pre-fill fields if the contact already exists (deactivated)
          if (data.contactDetails) {
            setFirstName(data.contactDetails.firstName ?? "");
            setLastName(data.contactDetails.lastName ?? "");
            setRole(
              data.contactDetails.isCsIntegrationUser
                ? AddUserContactRole.SYSTEM_USER
                : AddUserContactRole.PORTAL_USER,
            );
            setIsExistingContact(true);
          } else {
            setIsExistingContact(false);
          }
          setStep(AddUserModalStep.DETAILS);
        },
        onError: (err) => {
          setEmailError(err.message || ADD_USER_EMAIL_VALIDATE_ERROR);
        },
      },
    );
  }, [email, validateContact]);

  const handleBackToEmail = useCallback(() => {
    setStep(AddUserModalStep.EMAIL);
    setFirstName("");
    setLastName("");
    setRole(AddUserContactRole.PORTAL_USER);
    setIsExistingContact(false);
  }, []);

  const handleSubmit = useCallback(() => {
    const trimmedFirst = firstName.trim();
    const trimmedLast = lastName.trim();
    const trimmedEmail = email.trim();

    if (!trimmedEmail || !trimmedFirst) return;

    onSubmit({
      contactEmail: trimmedEmail,
      contactFirstName: trimmedFirst,
      contactLastName: trimmedLast,
      isCsIntegrationUser: role === AddUserContactRole.SYSTEM_USER,
      isSecurityContact: role === AddUserContactRole.SECURITY_USER,
    });
  }, [firstName, lastName, email, role, onSubmit]);

  const isDetailsValid = firstName.trim().length > 0 && email.trim().length > 0;
  const selectedRole = ADD_USER_ROLE_OPTIONS.find((r) => r.id === role);

  const handleRoleChange = useCallback(
    (event: SelectChangeEvent<AddUserContactRole>) => {
      setRole(event.target.value as AddUserContactRole);
    },
    [],
  );

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="add-user-modal-title"
      aria-describedby="add-user-modal-description"
      slotProps={{
        paper: { sx: { position: "relative" } },
      }}
    >
      <IconButton
        aria-label="Close"
        size="small"
        onClick={handleClose}
        disabled={isBusy}
        sx={{
          position: "absolute",
          right: 8,
          top: 8,
          zIndex: 1,
        }}
      >
        <X size={18} />
      </IconButton>
      <DialogTitle id="add-user-modal-title">{ADD_USER_MODAL_TITLE}</DialogTitle>

      {/* ─── Email validation ─── */}
      {step === AddUserModalStep.EMAIL && (
        <>
          <DialogContent>
            <Typography
              id="add-user-modal-description"
              variant="body2"
              color="text.secondary"
              sx={{ mb: 2 }}
            >
              {ADD_USER_EMAIL_STEP_DESCRIPTION}
            </Typography>

            <Box>
              <InputLabel
                htmlFor="add-user-email"
                sx={{ display: "block", mb: 1, fontSize: "0.875rem" }}
              >
                {ADD_USER_EMAIL_LABEL}{" "}
                <span style={{ color: "var(--oxygen-palette-error-main)" }}>
                  *
                </span>
              </InputLabel>
              <Input
                ref={emailInputRef}
                id="add-user-email"
                type="email"
                fullWidth
                placeholder={ADD_USER_EMAIL_PLACEHOLDER}
                value={email}
                onChange={(e) => {
                  setEmail(e.target.value);
                  setEmailError("");
                }}
                disabled={validateContact.isPending}
                error={!!emailError}
                slotProps={{
                  input: {
                    "aria-invalid": !!emailError,
                    "aria-errormessage": emailError
                      ? "add-user-email-error"
                      : undefined,
                  },
                }}
              />
              {emailError && (
                <Typography
                  id="add-user-email-error"
                  variant="caption"
                  color="error"
                  sx={{ display: "block", mt: 0.5 }}
                >
                  {emailError}
                </Typography>
              )}
            </Box>
          </DialogContent>
          <DialogActions sx={{ px: 3, pb: 2 }}>
            <Button
              variant="outlined"
              onClick={handleClose}
              disabled={validateContact.isPending}
            >
              {ADD_USER_MODAL_CANCEL}
            </Button>
            <Button
              variant="contained"
              color="warning"
              onClick={handleEmailNext}
              disabled={!email.trim() || validateContact.isPending}
              startIcon={
                validateContact.isPending ? (
                  <CircularProgress size={16} color="inherit" />
                ) : undefined
              }
            >
              {validateContact.isPending
                ? ADD_USER_MODAL_VALIDATING
                : ADD_USER_MODAL_NEXT}
            </Button>
          </DialogActions>
        </>
      )}

      {/* ─── Full name, email (read-only), role ─── */}
      {step === AddUserModalStep.DETAILS && (
        <>
          <DialogContent>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              {ADD_USER_DETAILS_INTRO}
            </Typography>

            <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
              <Box>
                <InputLabel
                  htmlFor="add-user-firstname"
                  sx={{ display: "block", mb: 1, fontSize: "0.875rem" }}
                >
                  {ADD_USER_FIRST_NAME_LABEL}{" "}
                  <span style={{ color: "var(--oxygen-palette-error-main)" }}>
                    *
                  </span>
                </InputLabel>
                <Input
                  id="add-user-firstname"
                  fullWidth
                  placeholder={ADD_USER_FIRST_NAME_PLACEHOLDER}
                  value={firstName}
                  onChange={(e) => setFirstName(e.target.value)}
                  disabled={isSubmitting || isExistingContact}
                />
              </Box>

              <Box>
                <InputLabel
                  htmlFor="add-user-lastname"
                  sx={{ display: "block", mb: 1, fontSize: "0.875rem" }}
                >
                  {ADD_USER_LAST_NAME_LABEL}
                </InputLabel>
                <Input
                  id="add-user-lastname"
                  fullWidth
                  placeholder={ADD_USER_LAST_NAME_PLACEHOLDER}
                  value={lastName}
                  onChange={(e) => setLastName(e.target.value)}
                  disabled={isSubmitting || isExistingContact}
                />
              </Box>

              <Box>
                <InputLabel
                  htmlFor="add-user-email-readonly"
                  sx={{ display: "block", mb: 1, fontSize: "0.875rem" }}
                >
                  {ADD_USER_EMAIL_LABEL}
                </InputLabel>
                <Input
                  id="add-user-email-readonly"
                  type="email"
                  fullWidth
                  value={email}
                  disabled
                />
              </Box>

              <FormControl fullWidth size="medium">
                <InputLabel id="add-user-role-label">
                  {ADD_USER_TYPE_LABEL}
                </InputLabel>
                <Select<AddUserContactRole>
                  labelId="add-user-role-label"
                  id="add-user-role"
                  value={role}
                  label={ADD_USER_TYPE_LABEL}
                  onChange={handleRoleChange}
                  disabled={isSubmitting || isExistingContact}
                  renderValue={() => {
                    const RoleIcon = selectedRole?.Icon;
                    return RoleIcon ? (
                      <Box
                        sx={{ display: "flex", alignItems: "center", gap: 1 }}
                      >
                        <RoleIcon size={16} />
                        {selectedRole?.label ?? role}
                      </Box>
                    ) : (
                      role
                    );
                  }}
                >
                  {ADD_USER_ROLE_OPTIONS.map((r) => {
                    const RoleIcon = r.Icon;
                    return (
                      <MenuItem key={r.id} value={r.id}>
                        <Box
                          sx={{ display: "flex", alignItems: "center", gap: 1 }}
                        >
                          <RoleIcon size={16} />
                          {r.label}
                        </Box>
                      </MenuItem>
                    );
                  })}
                </Select>
              </FormControl>
            </Box>
          </DialogContent>
          <DialogActions sx={{ px: 3, pb: 2 }}>
            <Button
              variant="outlined"
              onClick={handleBackToEmail}
              disabled={isSubmitting}
            >
              {ADD_USER_MODAL_BACK}
            </Button>
            <Button
              variant="contained"
              color="warning"
              onClick={handleSubmit}
              disabled={!isDetailsValid || isSubmitting}
            >
              {isSubmitting
                ? ADD_USER_MODAL_SENDING
                : ADD_USER_MODAL_SEND_INVITATION}
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  );
}

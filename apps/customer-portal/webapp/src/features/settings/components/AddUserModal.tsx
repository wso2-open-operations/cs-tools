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
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  IconButton,
  Input,
  InputLabel,
  Typography,
  alpha,
  colors,
  useTheme,
} from "@wso2/oxygen-ui";
import { Code, Crown, Monitor, Shield, X } from "@wso2/oxygen-ui-icons-react";
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
} from "@features/settings/constants/settingsConstants";
import {
  AddUserModalStep,
  type AddUserModalProps,
} from "@features/settings/types/settings";

const ADMIN_COLOR = colors.purple?.[600] ?? "#7c3aed";

const ADD_USER_ROLES = [
  {
    id: "admin" as const,
    label: "Admin",
    description: "Full administrative privileges and user management",
    Icon: Crown,
    colorKey: "admin" as const,
  },
  {
    id: "portal" as const,
    label: "Portal User",
    description: "Can log in to and access the Support Portal",
    Icon: Monitor,
    colorKey: "info" as const,
  },
  {
    id: "security" as const,
    label: "Security Contact",
    description: "Receives security bulletins and critical security announcements",
    Icon: Shield,
    colorKey: "error" as const,
  },
  {
    id: "webuser" as const,
    label: "System User",
    description: "Used exclusively for system to system integrations. Cannot log in to the Support Portal",
    Icon: Code,
    colorKey: "warning" as const,
  },
] as const;

/**
 * Two-step modal to add a new user (contact) to the project.
 * Step 1: validate email. Step 2: fill name and assign roles.
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
  const theme = useTheme();
  const [step, setStep] = useState<AddUserModalStep>(AddUserModalStep.EMAIL);
  const [email, setEmail] = useState("");
  const [emailError, setEmailError] = useState("");
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [isCsAdmin, setIsCsAdmin] = useState(false);
  const [isPortalUser, setIsPortalUser] = useState(true);
  const [isSecurityContact, setIsSecurityContact] = useState(false);
  const [isWebUser, setIsWebUser] = useState(false);
  const [isExistingContact, setIsExistingContact] = useState(false);
  const emailInputRef = useRef<HTMLInputElement>(null);

  const validateContact = useValidateProjectContact(projectId);

  const resetForm = useCallback(() => {
    setStep(AddUserModalStep.EMAIL);
    setEmail("");
    setEmailError("");
    setFirstName("");
    setLastName("");
    setIsCsAdmin(false);
    setIsPortalUser(true);
    setIsSecurityContact(false);
    setIsWebUser(false);
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
            setEmailError(data.message || ADD_USER_EMAIL_INVALID_CONTACT_ERROR);
            return;
          }
          if (data.contactDetails) {
            setFirstName(data.contactDetails.firstName ?? "");
            setLastName(data.contactDetails.lastName ?? "");
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
    setIsCsAdmin(false);
    setIsPortalUser(true);
    setIsSecurityContact(false);
    setIsWebUser(false);
    setIsExistingContact(false);
  }, []);

  const handleRoleChange = useCallback(
    (roleId: (typeof ADD_USER_ROLES)[number]["id"]) => {
      if (isSubmitting) return;
      if (roleId === "webuser") {
        const next = !isWebUser;
        setIsWebUser(next);
        if (next) {
          setIsCsAdmin(false);
          setIsPortalUser(false);
          setIsSecurityContact(false);
        }
      } else {
        if (isWebUser) return;
        if (roleId === "admin") setIsCsAdmin((p) => !p);
        else if (roleId === "portal") setIsPortalUser((p) => !p);
        else setIsSecurityContact((p) => !p);
      }
    },
    [isSubmitting, isWebUser],
  );

  const handleSubmit = useCallback(() => {
    const trimmedFirst = firstName.trim();
    const trimmedLast = lastName.trim();
    const trimmedEmail = email.trim();
    if (!trimmedEmail || !trimmedFirst) return;

    onSubmit({
      contactEmail: trimmedEmail,
      contactFirstName: trimmedFirst,
      contactLastName: trimmedLast,
      isCsAdmin: isWebUser ? false : isCsAdmin,
      isCsIntegrationUser: isWebUser,
      isPortalUser: isWebUser ? false : isPortalUser,
      isSecurityContact: isWebUser ? false : isSecurityContact,
    });
  }, [
    firstName,
    lastName,
    email,
    isCsAdmin,
    isPortalUser,
    isSecurityContact,
    isWebUser,
    onSubmit,
  ]);

  const isDetailsValid = firstName.trim().length > 0 && email.trim().length > 0;

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="add-user-modal-title"
      aria-describedby="add-user-modal-description"
      slotProps={{ paper: { sx: { position: "relative" } } }}
    >
      <IconButton
        aria-label="Close"
        size="small"
        onClick={handleClose}
        disabled={isBusy}
        sx={{ position: "absolute", right: 8, top: 8, zIndex: 1 }}
      >
        <X size={18} />
      </IconButton>
      <DialogTitle id="add-user-modal-title">{ADD_USER_MODAL_TITLE}</DialogTitle>

      {/* ─── Step 1: Email validation ─── */}
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
                <span style={{ color: "var(--oxygen-palette-error-main)" }}>*</span>
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
                    "aria-errormessage": emailError ? "add-user-email-error" : undefined,
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
              {validateContact.isPending ? ADD_USER_MODAL_VALIDATING : ADD_USER_MODAL_NEXT}
            </Button>
          </DialogActions>
        </>
      )}

      {/* ─── Step 2: Name + roles ─── */}
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
                  <span style={{ color: "var(--oxygen-palette-error-main)" }}>*</span>
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

              <Box>
                <Typography variant="subtitle2" sx={{ mb: 1 }}>
                  Membership Roles
                </Typography>
                <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
                  {ADD_USER_ROLES.map((role, index) => {
                    const RoleIcon = role.Icon;
                    const resolvedColor =
                      role.colorKey === "admin"
                        ? ADMIN_COLOR
                        : theme.palette[role.colorKey as "info" | "error" | "warning"]
                            ?.main ?? theme.palette.text.primary;
                    const isOtherRole = role.id !== "webuser";
                    const isChecked =
                      role.id === "webuser"
                        ? isWebUser
                        : role.id === "admin"
                          ? isCsAdmin
                          : role.id === "portal"
                            ? isPortalUser
                            : isSecurityContact;
                    const isDisabled = isSubmitting || (isOtherRole && isWebUser);

                    return (
                      <Box key={role.id}>
                        {index > 0 && <Divider sx={{ my: 0.5 }} />}
                        <FormControlLabel
                          control={
                            <Checkbox
                              checked={isChecked}
                              onChange={() => handleRoleChange(role.id)}
                              disabled={isDisabled}
                              size="small"
                              sx={{
                                color: resolvedColor,
                                "&.Mui-checked": { color: resolvedColor },
                              }}
                            />
                          }
                          label={
                            <Box
                              sx={{
                                display: "flex",
                                alignItems: "flex-start",
                                gap: 1,
                                py: 0.5,
                                opacity: isDisabled ? 0.4 : 1,
                              }}
                            >
                              <Box
                                sx={{
                                  width: 28,
                                  height: 28,
                                  borderRadius: 0.75,
                                  display: "flex",
                                  alignItems: "center",
                                  justifyContent: "center",
                                  bgcolor: alpha(resolvedColor, 0.1),
                                  color: resolvedColor,
                                  flexShrink: 0,
                                  mt: 0.25,
                                }}
                              >
                                <RoleIcon size={14} />
                              </Box>
                              <Box>
                                <Typography variant="body2" sx={{ fontWeight: 500 }}>
                                  {role.label}
                                </Typography>
                                <Typography variant="caption" color="text.secondary">
                                  {role.description}
                                </Typography>
                              </Box>
                            </Box>
                          }
                          sx={{ alignItems: "flex-start", mx: 0, width: "100%" }}
                        />
                      </Box>
                    );
                  })}
                </Box>
              </Box>
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
              {isSubmitting ? ADD_USER_MODAL_SENDING : ADD_USER_MODAL_SEND_INVITATION}
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  );
}

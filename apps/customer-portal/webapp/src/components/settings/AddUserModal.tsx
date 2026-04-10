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
import { Code, X, Monitor, Shield } from "@wso2/oxygen-ui-icons-react";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import type { CreateProjectContactRequest } from "@models/requests";
import { useValidateProjectContact } from "@api/useValidateProjectContact";

type ContactRole = "portal_user" | "security_user" | "system_user";

const ROLES: { id: ContactRole; label: string; Icon: typeof Code }[] = [
  { id: "portal_user", label: "Portal User", Icon: Monitor },
  { id: "security_user", label: "Security User", Icon: Shield },
  { id: "system_user", label: "System User", Icon: Code },
];

/** Basic email format validation: local@domain.tld */
const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

type ModalStep = "email" | "details";

export interface AddUserModalProps {
  open: boolean;
  projectId: string;
  onClose: () => void;
  onSubmit: (data: CreateProjectContactRequest) => void;
  isSubmitting?: boolean;
}

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
  const [step, setStep] = useState<ModalStep>("email");
  const [email, setEmail] = useState("");
  const [emailError, setEmailError] = useState("");
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [role, setRole] = useState<ContactRole>("portal_user");
  const [isExistingContact, setIsExistingContact] = useState(false);
  const emailInputRef = useRef<HTMLInputElement>(null);

  const validateContact = useValidateProjectContact(projectId);

  const resetForm = useCallback(() => {
    setStep("email");
    setEmail("");
    setEmailError("");
    setFirstName("");
    setLastName("");
    setRole("portal_user");
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
      setEmailError("Email address is required");
      emailInputRef.current?.focus();
      return;
    }
    if (!EMAIL_REGEX.test(trimmedEmail)) {
      setEmailError("Enter a valid email address (e.g. user@company.com)");
      emailInputRef.current?.focus();
      return;
    }

    validateContact.mutate(
      { contactEmail: trimmedEmail },
      {
        onSuccess: (data) => {
          if (!data.isContactValid) {
            setEmailError(data.message || "This email cannot be added.");
            return;
          }
          // Pre-fill fields if the contact already exists (deactivated)
          if (data.contactDetails) {
            setFirstName(data.contactDetails.firstName ?? "");
            setLastName(data.contactDetails.lastName ?? "");
            setRole(data.contactDetails.isCsIntegrationUser ? "system_user" : "portal_user");
            setIsExistingContact(true);
          } else {
            setIsExistingContact(false);
          }
          setStep("details");
        },
        onError: (err) => {
          setEmailError(err.message || "Email validation failed. Please try again.");
        },
      },
    );
  }, [email, validateContact]);

  const handleBackToEmail = useCallback(() => {
    setStep("email");
    setFirstName("");
    setLastName("");
    setRole("portal_user");
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
      isCsIntegrationUser: role === "system_user",
      isSecurityContact: role === "security_user",
    });
  }, [firstName, lastName, email, role, onSubmit]);

  const isDetailsValid =
    firstName.trim().length > 0 &&
    email.trim().length > 0;
  const selectedRole = ROLES.find((r) => r.id === role);

  const handleRoleChange = useCallback((event: SelectChangeEvent<ContactRole>) => {
    setRole(event.target.value as ContactRole);
  }, []);

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
      <DialogTitle id="add-user-modal-title">Add New User</DialogTitle>

      {/* ─── Email validation ─── */}
      {step === "email" && (
        <>
          <DialogContent>
            <Typography
              id="add-user-modal-description"
              variant="body2"
              color="text.secondary"
              sx={{ mb: 2 }}
            >
              Enter the email address of the user you want to add
            </Typography>

            <Box>
              <InputLabel
                htmlFor="add-user-email"
                sx={{ display: "block", mb: 1, fontSize: "0.875rem" }}
              >
                Email Address <span style={{ color: "var(--oxygen-palette-error-main)" }}>*</span>
              </InputLabel>
              <Input
                ref={emailInputRef}
                id="add-user-email"
                type="email"
                fullWidth
                placeholder="user@company.com"
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
            <Button variant="outlined" onClick={handleClose} disabled={validateContact.isPending}>
              Cancel
            </Button>
            <Button
              variant="contained"
              color="warning"
              onClick={handleEmailNext}
              disabled={!email.trim() || validateContact.isPending}
              startIcon={
                validateContact.isPending ? <CircularProgress size={16} color="inherit" /> : undefined
              }
            >
              {validateContact.isPending ? "Validating..." : "Next"}
            </Button>
          </DialogActions>
        </>
      )}

      {/* ─── Full name, email (read-only), role ─── */}
      {step === "details" && (
        <>
          <DialogContent>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Send an invitation to a new user to access the portal
            </Typography>

            <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
              <Box>
                <InputLabel
                  htmlFor="add-user-firstname"
                  sx={{ display: "block", mb: 1, fontSize: "0.875rem" }}
                >
                  First Name <span style={{ color: "var(--oxygen-palette-error-main)" }}>*</span>
                </InputLabel>
                <Input
                  id="add-user-firstname"
                  fullWidth
                  placeholder="Enter first name"
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
                  Last Name
                </InputLabel>
                <Input
                  id="add-user-lastname"
                  fullWidth
                  placeholder="Enter last name"
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
                  Email Address
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
                <InputLabel id="add-user-role-label">User Type</InputLabel>
                <Select<ContactRole>
                  labelId="add-user-role-label"
                  id="add-user-role"
                  value={role}
                  label="User Type"
                  onChange={handleRoleChange}
                  disabled={isSubmitting || isExistingContact}
                  renderValue={() => {
                    const RoleIcon = selectedRole?.Icon;
                    return RoleIcon ? (
                      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                        <RoleIcon size={16} />
                        {selectedRole?.label ?? role}
                      </Box>
                    ) : (
                      role
                    );
                  }}
                >
                  {ROLES.map((r) => {
                    const RoleIcon = r.Icon;
                    return (
                      <MenuItem key={r.id} value={r.id}>
                        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
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
            <Button variant="outlined" onClick={handleBackToEmail} disabled={isSubmitting}>
              Back
            </Button>
            <Button
              variant="contained"
              color="warning"
              onClick={handleSubmit}
              disabled={!isDetailsValid || isSubmitting}
            >
              {isSubmitting ? "Sending..." : "Send Invitation"}
            </Button>
          </DialogActions>
        </>
      )}
    </Dialog>
  );
}

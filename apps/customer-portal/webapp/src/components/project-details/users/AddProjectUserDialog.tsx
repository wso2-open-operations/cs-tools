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

import { useState } from "react";
import {
    Box,
    Button,
    Dialog,
    DialogActions,
    DialogContent,
    DialogTitle,
    TextField,
    Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface AddProjectUserDialogProps {
    open: boolean;
    onClose: () => void;
    onSubmit: (user: { email: string; firstName: string; lastName: string }) => void;
}

/**
 * AddProjectUserDialog component provides a multi step dialog
 * for adding a new project user.
 *
 * @param {AddProjectUserDialogProps} props - The props for the component.
 * @returns {JSX.Element} The AddProjectUserDialog component.
 */
export default function AddProjectUserDialog({
    open,
    onClose,
    onSubmit,
}: AddProjectUserDialogProps): JSX.Element {
    const [step, setStep] = useState<1 | 2>(1);
    const [email, setEmail] = useState<string>("");
    const [firstName, setFirstName] = useState<string>("");
    const [lastName, setLastName] = useState<string>("");
    const [emailError, setEmailError] = useState<string>("");

    const resetForm = (): void => {
        setStep(1);
        setEmail("");
        setFirstName("");
        setLastName("");
        setEmailError("");
    };

    const handleClose = (): void => {
        resetForm();
        onClose();
    };

    const validateEmail = (value: string): boolean => {
        const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        return emailRegex.test(value);
    };

    const handleNext = (): void => {
        if (!email.trim()) {
            setEmailError("Email is required.");
            return;
        }
        if (!validateEmail(email.trim())) {
            setEmailError("Please enter a valid email address.");
            return;
        }
        setEmailError("");
        setStep(2);
    };

    const handleDone = (): void => {
        onSubmit({
            email: email.trim(),
            firstName: firstName.trim(),
            lastName: lastName.trim(),
        });
        resetForm();
        onClose();
    };

    return (
        <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
            <DialogTitle>
                {step === 1 ? "Add Project User" : "User Details"}
            </DialogTitle>
            <DialogContent>
                {step === 1 ? (
                    <Box sx={{ pt: 1 }}>
                        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                            Enter the email address of the user you want to add to this
                            project.
                        </Typography>
                        <TextField
                            autoFocus
                            label="Email Address"
                            type="email"
                            fullWidth
                            required
                            value={email}
                            onChange={(e) => {
                                setEmail(e.target.value);
                                if (emailError) setEmailError("");
                            }}
                            error={!!emailError}
                            helperText={emailError}
                            onKeyDown={(e) => {
                                if (e.key === "Enter") handleNext();
                            }}
                        />
                    </Box>
                ) : (
                    <Box sx={{ pt: 1 }}>
                        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                            Provide the name details for <strong>{email}</strong>.
                        </Typography>
                        <TextField
                            autoFocus
                            label="First Name"
                            fullWidth
                            value={firstName}
                            onChange={(e) => setFirstName(e.target.value)}
                            sx={{ mb: 2 }}
                        />
                        <TextField
                            label="Last Name"
                            fullWidth
                            value={lastName}
                            onChange={(e) => setLastName(e.target.value)}
                        />
                    </Box>
                )}
            </DialogContent>
            <DialogActions sx={{ px: 3, pb: 2 }}>
                {step === 1 ? (
                    <>
                        <Button variant="text" color="secondary" onClick={handleClose}>
                            Cancel
                        </Button>
                        <Button
                            variant="contained"
                            color="primary"
                            onClick={handleNext}
                            disabled={!email.trim()}
                        >
                            Next
                        </Button>
                    </>
                ) : (
                    <>
                        <Button variant="text" color="secondary" onClick={() => setStep(1)}>
                            Back
                        </Button>
                        <Button variant="contained" color="primary" onClick={handleDone}>
                            Done
                        </Button>
                    </>
                )}
            </DialogActions>
        </Dialog>
    );
}

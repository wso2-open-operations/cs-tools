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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  LinearProgress,
  Typography,
} from "@wso2/oxygen-ui";
import { useEffect, useRef, useState, type JSX } from "react";
import { CONTINUE_LOADER_MS } from "@constants/authConstants";

const TICK_MS = 100;
const INCREMENT = 100 / (CONTINUE_LOADER_MS / TICK_MS);
const BUFFER_LEAD = 8;

export interface SessionWarningDialogProps {
  open: boolean;
  isContinuing: boolean;
  onContinue: () => void;
  onLogout: () => void;
}

/**
 * Dialog that asks "Are you still there?" when the user has been idle.
 * Continue shows a buffer LinearProgress for CONTINUE_LOADER_MS; Logout signs the user out.
 * Backdrop click and Escape key are intentionally disabled — the user must
 * make an explicit choice.
 *
 * @param {SessionWarningDialogProps} props - open, isContinuing, onContinue, onLogout.
 * @returns {JSX.Element} The session warning dialog.
 */
export default function SessionWarningDialog({
  open,
  isContinuing,
  onContinue,
  onLogout,
}: SessionWarningDialogProps): JSX.Element {
  const [progress, setProgress] = useState(0);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    if (isContinuing) {
      setProgress(0);
      intervalRef.current = setInterval(() => {
        setProgress((prev) => Math.min(100, prev + INCREMENT));
      }, TICK_MS);
    } else {
      if (intervalRef.current !== null) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
      setProgress(0);
    }
    return () => {
      if (intervalRef.current !== null) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [isContinuing]);

  const valueBuffer = Math.min(100, progress + BUFFER_LEAD);

  return (
    <Dialog
      open={open}
      onClose={() => undefined}
      disableEscapeKeyDown
      maxWidth="sm"
      fullWidth
      aria-labelledby="session-warning-dialog-title"
      slotProps={{
        backdrop: {
          sx: {
            backdropFilter: "blur(16px)",
            backgroundColor: "rgba(0,0,0,0.65)",
          },
        },
      }}
    >
      <DialogTitle
        id="session-warning-dialog-title"
        sx={isContinuing ? { position: "absolute", width: 1, height: 1, overflow: "hidden", clip: "rect(0 0 0 0)", whiteSpace: "nowrap" } : undefined}
      >
        Are you still there?
      </DialogTitle>
      <DialogContent>
        {isContinuing ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, py: 1 }}>
            <Typography color="text.secondary">
              Resuming your session…
            </Typography>
            <Box sx={{ width: "100%" }}>
              <LinearProgress
                variant="buffer"
                value={progress}
                valueBuffer={valueBuffer}
              />
            </Box>
          </Box>
        ) : (
          <Typography color="text.secondary">
            It looks like you&apos;ve been inactive for a while. Would you like
            to continue?
          </Typography>
        )}
      </DialogContent>
      {!isContinuing && (
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button variant="outlined" onClick={onLogout}>
            Logout
          </Button>
          <Button variant="contained" color="primary" onClick={onContinue}>
            Continue
          </Button>
        </DialogActions>
      )}
    </Dialog>
  );
}

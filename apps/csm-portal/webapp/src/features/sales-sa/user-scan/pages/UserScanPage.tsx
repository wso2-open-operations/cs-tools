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

// Ported from apps/support-portal-lite/webapp's UserScanPage (itself via
// one-wso2, from the original source app's UserScanning.tsx). UI/JSX
// unchanged; the data layer is rewritten onto this app's useBackendApi()/
// React Query convention instead of the source's own useSplHttpRequest —
// see useScanUser.ts. No SplShell/PageHeader wrapper — this app's own
// AppLayout/RouteGuard already provide the page chrome.
import { useState, type ChangeEvent, type JSX } from "react";
import {
  Box,
  Button,
  Checkbox,
  CircularProgress,
  FormControlLabel,
  IconButton,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { CheckCircleIcon, XCircleIcon, CopyIcon } from "@wso2/oxygen-ui-icons-react";
import { useScanUser, type ScanResponseItem } from "@features/sales-sa/user-scan/api/useScanUser";

function copyToClipboard(value: string) {
  void navigator.clipboard.writeText(value);
}

export default function UserScanPage(): JSX.Element {
  const scanUser = useScanUser();

  const [email, setEmail] = useState("");
  const [projectKey, setProjectKey] = useState("");
  const [isPartner, setIsPartner] = useState(false);
  const [responseData, setResponseData] = useState<ScanResponseItem[]>([]);
  const [isEmailError, setIsEmailError] = useState(false);
  const [isPageLoading, setIsPageLoading] = useState(false);
  const [isProjectKeyError, setIsProjectKeyError] = useState(false);
  const [scanErrorMessage, setScanErrorMessage] = useState("");

  const runScan = () => {
    setResponseData([]);
    setScanErrorMessage("");
    if (email === "") {
      setIsEmailError(true);
    } else if (projectKey === "") {
      setIsProjectKeyError(true);
    } else if (!/^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$/i.test(email)) {
      setIsEmailError(true);
    } else {
      setIsEmailError(false);
      setIsProjectKeyError(false);
      // FIX (carried from the earlier reconciliation pass, see
      // useScanUser.ts): isPartner is the real checkbox value, not a
      // hardcoded `false` — sending the wrong value here silently
      // mis-analyzes every partner-contact scan (internal/handler/
      // user_scan.go branches on it directly).
      scanUser.mutate(
        { email, subscriptionKey: projectKey, isPartner },
        {
          onSuccess: (result) => setResponseData(result),
          onError: () => setScanErrorMessage("Something went wrong while scanning this user. Please try again."),
        },
      );
    }
  };

  // Stale results (including a confidential invitation link) must not
  // linger once the operator changes what they're scanning for.
  const clearScanResults = () => {
    setResponseData([]);
    setScanErrorMessage("");
  };

  const handleEmailFieldChange = (event: ChangeEvent<HTMLInputElement>) => {
    setEmail(event.target.value);
    setIsEmailError(false);
    setIsPageLoading(true);
    clearScanResults();
  };

  const handleProjectFieldChange = (event: ChangeEvent<HTMLInputElement>) => {
    setProjectKey(event.target.value);
    setIsProjectKeyError(false);
    setIsPageLoading(true);
    clearScanResults();
  };

  const handleIsPartnerChange = () => {
    setIsPartner(!isPartner);
    clearScanResults();
  };

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h5" sx={{ mb: 2 }}>
        User scan
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, mb: 2 }}>
        <Typography variant="h6" sx={{ mb: 2 }}>
          User Scan
        </Typography>
        <Stack direction="row" spacing={2} alignItems="center" flexWrap="wrap" useFlexGap>
          <TextField
            id="email"
            size="small"
            label="Email"
            variant="outlined"
            required
            helperText="Please provide a valid Email"
            onChange={handleEmailFieldChange}
            color="primary"
          />
          <TextField
            id="projectKey"
            size="small"
            required
            label="Subscription Key"
            variant="outlined"
            helperText="Please provide a valid subscription key"
            onChange={handleProjectFieldChange}
          />
          <FormControlLabel
            control={<Checkbox checked={isPartner} onChange={handleIsPartnerChange} />}
            label="Is Partner"
          />
          <Button
            variant="contained"
            size="small"
            color="secondary"
            onClick={runScan}
            disabled={!isPageLoading || isEmailError || isProjectKeyError}
          >
            Analyze
          </Button>
        </Stack>
        {isEmailError && (
          <Typography variant="body2" color="error" sx={{ mt: 1 }}>
            Please provide valid Email address
          </Typography>
        )}
        {isProjectKeyError && (
          <Typography variant="body2" color="error" sx={{ mt: 1 }}>
            Please provide valid subscription key
          </Typography>
        )}
        {scanErrorMessage && (
          <Typography variant="body2" color="error" sx={{ mt: 1 }}>
            {scanErrorMessage}
          </Typography>
        )}
      </Paper>

      {scanUser.isPending && (
        <Box sx={{ display: "flex", justifyContent: "center", py: 2 }}>
          <CircularProgress size={24} />
        </Box>
      )}

      {responseData.map((responseObject, idx) => (
        <Paper key={idx} variant="outlined" sx={{ p: 2, mb: 2 }}>
          <Typography sx={{ fontWeight: "bold", mb: 1 }}>{responseObject.system}</Typography>
          {responseObject.systemResult?.map((systemResult, resultIdx) => (
            <Box key={resultIdx} sx={{ mb: 1.5 }}>
              <Stack direction="row" spacing={1} alignItems="center">
                {systemResult.state ? (
                  <CheckCircleIcon size={18} color="var(--mui-palette-success-main, #2e7d32)" />
                ) : (
                  <XCircleIcon size={18} color="var(--mui-palette-error-main, #d32f2f)" />
                )}
                <Typography variant="body2">{systemResult.label}</Typography>
              </Stack>
              {!systemResult.state && systemResult.information && (
                <Box component="table" sx={{ mt: 1, ml: 3.5 }}>
                  <Box component="tr">
                    <Box component="td" sx={{ pr: 2, verticalAlign: "top" }}>
                      <strong>Issue</strong>
                    </Box>
                    <Box component="td">{systemResult.information.issue}</Box>
                  </Box>
                  <Box component="tr">
                    <Box component="td" sx={{ pr: 2, verticalAlign: "top" }}>
                      <strong>Solution</strong>
                    </Box>
                    <Box component="td">
                      {systemResult.information.solution}{" "}
                      <a href={systemResult.information.documentation} target="_blank" rel="noreferrer">
                        (instructions)
                      </a>
                    </Box>
                  </Box>
                  {systemResult.information.invitationUrl && (
                    <Box component="tr">
                      <Box component="td" sx={{ pr: 2, verticalAlign: "top" }}>
                        <strong>Invitation</strong>
                      </Box>
                      <Box component="td">
                        You can copy the invitation link and share it with the user&nbsp;
                        <strong>directly without CC-ing anyone or group.</strong>&nbsp; It is
                        confidential. To copy, click the icon.&nbsp;
                        <Tooltip title="Copy here">
                          <IconButton
                            size="small"
                            aria-label="Copy invitation link"
                            sx={{ verticalAlign: "middle" }}
                            onClick={() => copyToClipboard(systemResult.information.invitationUrl!)}
                          >
                            <CopyIcon size={16} />
                          </IconButton>
                        </Tooltip>
                      </Box>
                    </Box>
                  )}
                </Box>
              )}
            </Box>
          ))}
        </Paper>
      ))}
    </Box>
  );
}

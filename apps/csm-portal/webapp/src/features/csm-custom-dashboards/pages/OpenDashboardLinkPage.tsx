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

import { Box, Button, CircularProgress, Typography } from "@wso2/oxygen-ui";
import { Link2Off } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useEffect, useRef, useState } from "react";
import { Navigate, useNavigate, useParams } from "react-router";
import { BackendApiError } from "@api/backend/client";
import {
  type OpenedDashboardLink,
  useOpenDashboardLink,
} from "@features/csm-custom-dashboards/api/useDashboardLink";

/** What to say when the link opened nothing: the backend's words for a 4xx, else generic. */
function openErrorMessage(err: Error): string {
  if (err instanceof BackendApiError && err.status >= 400 && err.status < 500 && err.message) {
    return err.message;
  }
  return "The link couldn't be opened. Try again in a moment.";
}

/**
 * A dashboard's link (`/dashboard/shared/link/:linkId`): opens it once
 * (`POST /dashboards/shared/link`, the id in the body) and replaces this page
 * with the dashboard, under My Dashboards when it is the caller's own, else
 * under Shared Dashboards, where it now is. A link that is off or unknown says
 * so, with a way to the Shared Dashboards list. The link id is never shown.
 */
export default function OpenDashboardLinkPage(): JSX.Element {
  const { linkId = "" } = useParams<{ linkId: string }>();
  const navigate = useNavigate();
  const { mutateAsync } = useOpenDashboardLink();
  const [answer, setAnswer] = useState<
    ({ linkId: string } & ({ opened: OpenedDashboardLink } | { error: Error })) | null
  >(null);
  // Opened once per link id, also under StrictMode's double effect. The
  // answer is kept here, not read from the mutation: StrictMode's remount
  // detaches the mutation's observer, which then never sees it.
  const requested = useRef<string | null>(null);

  useEffect(() => {
    if (requested.current === linkId) return;
    requested.current = linkId;
    // An answer for a link the page has since moved on from is dropped, so a
    // late one cannot replace the current link's answer.
    mutateAsync(linkId).then(
      (opened) => {
        if (requested.current === linkId) setAnswer({ linkId, opened });
      },
      (error: Error) => {
        if (requested.current === linkId) setAnswer({ linkId, error });
      },
    );
  }, [linkId, mutateAsync]);

  // Only this link's answer counts: when the page moves on to another link,
  // the previous one's error or dashboard is not shown while the new link
  // opens.
  const result = answer?.linkId === linkId ? answer : null;

  if (result && "opened" in result) {
    const { dashboardId, own } = result.opened;
    const to = `/dashboard/${own ? "my" : "shared"}/${encodeURIComponent(dashboardId)}`;
    return <Navigate to={to} replace />;
  }

  if (result && "error" in result) {
    return (
      <Box
        sx={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 1, py: 8, textAlign: "center" }}
      >
        <Link2Off size={32} />
        <Typography variant="h6">Couldn&apos;t open the link</Typography>
        <Typography variant="body2" color="text.secondary">
          {openErrorMessage(result.error)}
        </Typography>
        <Button variant="outlined" sx={{ mt: 1 }} onClick={() => void navigate("/dashboard/shared")}>
          Go to Shared Dashboards
        </Button>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 1.5, py: 8 }}>
      <CircularProgress size={20} />
      <Typography variant="body2" color="text.secondary">
        Opening the dashboard…
      </Typography>
    </Box>
  );
}

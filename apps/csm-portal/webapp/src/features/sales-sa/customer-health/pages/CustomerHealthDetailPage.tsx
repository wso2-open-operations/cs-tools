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

// Ported from apps/support-portal-lite/webapp's CustomerHealthDetailPage.tsx.
// SplShell + HealthToastProvider both dropped — RouteGuard/AppLayout
// provide the chrome/auth boundary, and this app's global
// SuccessBannerProvider/ErrorBannerProvider (see App.tsx) replace the
// domain-local toast. Route paths updated from /customer-health/... to
// /spl/customer-health/... to match this app's route tree.
import { useState } from "react";
import { useParams, Link, useLocation, useNavigate } from "react-router";
import { Typography, Box, Divider, Button, CircularProgress } from "@wso2/oxygen-ui";
import { ArrowLeftIcon, ChevronLeftIcon, ChevronRightIcon } from "@wso2/oxygen-ui-icons-react";
import { useBackendApi } from "@api/backend/client";
import { fetchCustomerHealthSummaryPage, type CustomerHealthSummaryPayload } from "../api/useCustomerHealthSummary";
import CustomerHealthDetailTable from "../components/CustomerHealthDetailTable";

interface FilterPayload {
  email: string;
  phrase: string;
  risks: string;
  region: string[];
  product: string;
  abtTeam: string;
  healthStatus: string;
}

interface AccountNavState {
  accountList: { accountSysId: string; accountName: string }[];
  currentIndex: number;
  absoluteIndex: number;
  totalCount: number;
  filterPayload: FilterPayload;
}

export default function CustomerHealthDetailPage() {
  const { accountId } = useParams<{ accountId: string }>();
  const [accountName, setAccountName] = useState("");
  const [fetchingNav, setFetchingNav] = useState(false);
  const location = useLocation();
  const navigate = useNavigate();
  const backendApi = useBackendApi();

  const navState = location.state as AccountNavState | null;
  const accountList = navState?.accountList ?? [];
  const currentIndex = navState?.currentIndex ?? -1;
  const absoluteIndex = navState?.absoluteIndex ?? -1;
  const totalCount = navState?.totalCount ?? 0;

  const hasPrev = absoluteIndex > 0;
  const hasNext = absoluteIndex >= 0 && absoluteIndex < totalCount - 1;

  const fetchAccountAtIndex = async (targetIdx: number): Promise<{ accountSysId: string; accountName: string } | null> => {
    try {
      const payload: CustomerHealthSummaryPayload = {
        offset: targetIdx,
        limit: 1,
        email: navState!.filterPayload.email,
        phrase: navState!.filterPayload.phrase,
        risks: navState!.filterPayload.risks,
        region: navState!.filterPayload.region,
        product: navState!.filterPayload.product,
        abtTeam: navState!.filterPayload.abtTeam,
        healthStatus: navState!.filterPayload.healthStatus,
      };
      const result = await fetchCustomerHealthSummaryPage(backendApi, payload);
      const account = result.data[0];
      return account ? { accountSysId: account.accountSysId, accountName: account.accountName ?? "" } : null;
    } catch {
      return null;
    }
  };

  const navigateTo = async (targetAbsoluteIdx: number) => {
    if (targetAbsoluteIdx < 0 || targetAbsoluteIdx >= totalCount) return;

    const localRangeStart = absoluteIndex - currentIndex;
    const localRangeEnd = localRangeStart + accountList.length - 1;

    if (targetAbsoluteIdx >= localRangeStart && targetAbsoluteIdx <= localRangeEnd) {
      const localIdx = targetAbsoluteIdx - localRangeStart;
      navigate(`/spl/customer-health/account/${accountList[localIdx].accountSysId}`, {
        state: { ...navState, currentIndex: localIdx, absoluteIndex: targetAbsoluteIdx },
      });
      return;
    }

    setFetchingNav(true);
    const account = await fetchAccountAtIndex(targetAbsoluteIdx);
    setFetchingNav(false);
    if (account) {
      navigate(`/spl/customer-health/account/${account.accountSysId}`, {
        state: { accountList: [account], currentIndex: 0, absoluteIndex: targetAbsoluteIdx, totalCount, filterPayload: navState!.filterPayload },
      });
    }
  };

  if (!accountId) {
    return (
      <Box>
        <Link to="/spl/customer-health" style={{ textDecoration: "none" }}>
          <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: "4px", fontSize: "0.85rem", color: "text.secondary", "&:hover": { color: "#e96900" } }}>
            <ArrowLeftIcon size={16} />Back to Dashboard
          </Box>
        </Link>
      </Box>
    );
  }

  const navButtonSx = {
    fontSize: "0.8rem", textTransform: "none", borderColor: "divider", color: "text.secondary",
    "&:hover:not(:disabled)": { borderColor: "#e96900", color: "#e96900" },
  };

  return (
    <Box>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <Link to="/spl/customer-health" style={{ textDecoration: "none" }}>
          <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: "4px", fontSize: "0.85rem", color: "text.secondary", "&:hover": { color: "#e96900" } }}>
            <ArrowLeftIcon size={16} />Back to Dashboard
          </Box>
        </Link>

        {absoluteIndex >= 0 && totalCount > 0 && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Button
              size="small" variant="outlined" disabled={!hasPrev || fetchingNav} onClick={() => navigateTo(absoluteIndex - 1)}
              startIcon={fetchingNav ? <CircularProgress size={12} sx={{ color: "inherit" }} /> : <ChevronLeftIcon size={14} />}
              sx={navButtonSx}
            >
              Previous
            </Button>
            <Typography variant="caption" sx={{ color: "text.secondary", minWidth: 64, textAlign: "center" }}>
              {absoluteIndex + 1} / {totalCount}
            </Typography>
            <Button
              size="small" variant="outlined" disabled={!hasNext || fetchingNav} onClick={() => navigateTo(absoluteIndex + 1)}
              endIcon={fetchingNav ? <CircularProgress size={12} sx={{ color: "inherit" }} /> : <ChevronRightIcon size={14} />}
              sx={navButtonSx}
            >
              Next
            </Button>
          </Box>
        )}
      </Box>

      <Box sx={{ mt: 1, mb: 0 }}>
        <Box sx={{ display: "flex", alignItems: "baseline", gap: 1.5 }}>
          <Typography variant="h5" fontWeight={700} sx={{ fontSize: "1.5rem", color: "text.primary", lineHeight: 1.3 }}>
            {accountName || "Customer Health Analysis"}
          </Typography>
          {accountName && (
            <Typography variant="body2" sx={{ color: "text.secondary", fontWeight: 400, fontSize: "0.9rem" }}>Health Analysis</Typography>
          )}
        </Box>
        <Divider sx={{ mt: 1.5 }} />
      </Box>

      <CustomerHealthDetailTable accountId={accountId} onAccountNameLoaded={setAccountName} />
    </Box>
  );
}

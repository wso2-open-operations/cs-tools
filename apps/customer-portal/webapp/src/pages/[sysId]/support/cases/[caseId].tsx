import { Box, Tab, Tabs } from "@mui/material";
import React, { useState, useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { CaseHeader } from "@/components/CaseDetails/CaseHeader";
import { CaseActivity } from "@/components/CaseDetails/CaseActivity";
import { CaseInfo } from "@/components/CaseDetails/CaseInfo";
import { CaseCalls } from "@/components/CaseDetails/CaseCalls";
import { CaseAttachments } from "@/components/CaseDetails/CaseAttachments";
import { CaseKnowledge } from "@/components/CaseDetails/CaseKnowledge";
// import { CaseCommentInput } from "@/components/CaseDetails/CaseCommentInput";
import type { CaseDetails } from "@/types/case.types";

import { Endpoints } from "@/services/endpoints";
import { useGet } from "@/services/useApi";
import {
  BookOpenIcon,
  InfoIcon,
  ChatIcon,
  AttachmentIcon,
  PhoneIcon,
} from "@/assets/icons/common-icons";

const CaseDetailsPage: React.FC = () => {
  const { sysId, caseId } = useParams<{ sysId: string; caseId: string }>();
  const navigate = useNavigate();
  const [currentTab, setCurrentTab] = useState("activity");

  // Fetch case details using the API
  const {
    data: caseData,
    isLoading,
    error,
  } = useGet<CaseDetails>(
    ["getCaseDetails", sysId, caseId],
    Endpoints.getCaseDetails(sysId || "", caseId || ""),
    {
      enabled: !!sysId && !!caseId,
    }
  );

  // Scroll to top when component mounts
  // useEffect(() => {
  //   window.scrollTo(0, 0);
  // }, []);

  if (isLoading)
    return (
      <Box sx={{ p: 4, textAlign: "center", color: "#64748B" }}>
        Loading case details...
      </Box>
    );

  if (error)
    return (
      <Box sx={{ p: 4, textAlign: "center", color: "error.main" }}>
        Error loading case details: {error.message}
      </Box>
    );

  if (!caseData)
    return (
      <Box sx={{ p: 4, textAlign: "center", color: "error.main" }}>
        Case not found
      </Box>
    );

  return (
    <Box
      sx={{
        height: "100vh",
        display: "flex",
        flexDirection: "column",
        bgcolor: "#F8FAFC",
      }}
    >
      <CaseHeader
        caseData={caseData}
        onBack={() => navigate(`/${sysId}/support`)}
      />

      <Box sx={{ borderBottom: 1, borderColor: "divider", bgcolor: "white" }}>
        <Box sx={{ px: 4 }}>
          <Tabs
            value={currentTab}
            onChange={(_, v) => setCurrentTab(v)}
            sx={{
              "& .MuiTab-root": {
                textTransform: "none",
                minHeight: 48,
                fontWeight: 500,
                fontSize: "0.75rem",
                minWidth: 0,
                px: 1.5,
                py: 1,
              },
              "& .Mui-selected": { color: "#0F172A" },
              "& .MuiTabs-indicator": {
                backgroundColor: "#EA580C",
                height: 2,
              },
            }}
          >
            <Tab
              value="activity"
              label="Activity"
              icon={<ChatIcon width={14} height={14} />}
              iconPosition="start"
              sx={{ gap: 0.75 }}
            />
            <Tab
              value="details"
              label="Details"
              icon={<InfoIcon width={14} height={14} />}
              iconPosition="start"
              sx={{ gap: 0.75 }}
            />
            <Tab
              value="attachments"
              label={`Attachments (${caseData.attachments?.length || 0})`}
              icon={<AttachmentIcon width={14} height={14} />}
              iconPosition="start"
              sx={{ gap: 0.75 }}
            />
            <Tab
              value="calls"
              label={`Calls (${caseData.callRequests?.length || 0})`}
              icon={<PhoneIcon width={14} height={14} />}
              iconPosition="start"
              sx={{ gap: 0.75 }}
            />
            <Tab
              value="knowledge"
              label={`Knowledge (${caseData.kbArticles?.length || 0})`}
              icon={<BookOpenIcon width={14} height={14} />}
              iconPosition="start"
              sx={{ gap: 0.75 }}
            />
          </Tabs>
        </Box>
      </Box>

      <Box
        sx={{
          flex: 1,
          overflow: "hidden",
          bgcolor: "#F8FAFC",
          display: "flex",
          flexDirection: "column",
        }}
      >
        {currentTab === "activity" ? (
          <CaseActivity
            comments={caseData.initialComments}
            createdDate={caseData.createdOn}
          />
        ) : (
          <Box
            sx={{
              flex: 1,
              overflowY: "auto",
              px: 2,
              py: 2,
            }}
          >
            <Box sx={{ maxWidth: "1280px", mx: "auto" }}>
              {currentTab === "details" && <CaseInfo caseData={caseData} />}
              {currentTab === "attachments" && (
                <CaseAttachments attachments={caseData.attachments} />
              )}
              {currentTab === "calls" && (
                <CaseCalls callRequests={caseData.callRequests} />
              )}
              {currentTab === "knowledge" && (
                <CaseKnowledge kbArticles={caseData.kbArticles} />
              )}
            </Box>
          </Box>
        )}

        {/* Comment Input Section - Only show in Activity tab */}
        {/* {currentTab === "activity" && <CaseCommentInput />} */}
      </Box>
    </Box>
  );
};

export default CaseDetailsPage;

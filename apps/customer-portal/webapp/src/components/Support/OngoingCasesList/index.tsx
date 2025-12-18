import { Box, Card, Divider } from "@mui/material";
import React from "react";
import { useNavigate, useParams } from "react-router-dom";
import type { Case } from "../../../types/support.types";
import { CaseListFooter } from "./CaseListFooter";
import { CaseListHeader } from "./CaseListHeader";
import { CaseListItem } from "./CaseListItem";

interface OngoingCasesListProps {
  cases: Case[];
}

export const OngoingCasesList: React.FC<OngoingCasesListProps> = ({
  cases,
}) => {
  const navigate = useNavigate();
  const { sysId } = useParams();

  return (
    <Card
      sx={{
        p: 3,
        borderRadius: "12px", // rounded-xl
        border: "1px solid",
        borderColor: "grey.200",
        height: "100%",
        display: "flex",
        flexDirection: "column",
        gap: 3, // gap-6
        boxShadow: "none",
      }}
    >
      <CaseListHeader />

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        {cases.map((caseItem) => (
          <CaseListItem key={caseItem.sysId} item={caseItem} />
        ))}
      </Box>

      <Divider sx={{ my: 0 }} />

      <CaseListFooter onClick={() => navigate(`/${sysId}/support/cases`)} />
    </Card>
  );
};

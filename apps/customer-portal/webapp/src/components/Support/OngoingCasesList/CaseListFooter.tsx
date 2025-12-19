import { Button } from "@mui/material";
import React from "react";
import { ArrowRightIcon } from "../../../assets/icons/common-icons";

interface CaseListFooterProps {
  onClick?: () => void;
}

export const CaseListFooter: React.FC<CaseListFooterProps> = ({ onClick }) => {
  return (
    <Button
      fullWidth
      endIcon={<ArrowRightIcon width={16} height={16} />}
      onClick={onClick}
      sx={{
        justifyContent: "space-between",
        color: "#ea580c", // text-orange-600
        "&:hover": { backgroundColor: "#fff7ed", color: "#c2410c" }, // hover:bg-orange-50 hover:text-orange-700
        textTransform: "none",
        height: 36, // h-9
        px: 2,
        fontSize: "0.875rem",
        fontWeight: 500,
      }}
    >
      View all cases
    </Button>
  );
};

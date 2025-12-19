import { Box, Typography } from "@mui/material";
import React from "react";
import { FileTextIcon } from "../../../assets/icons/common-icons";

export const CaseListHeader: React.FC = () => {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
      <Box
        sx={{
          width: 40,
          height: 40,
          backgroundColor: "#ffedd5", // bg-orange-100
          borderRadius: "8px", // rounded-lg
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          color: "#ea580c", // text-orange-600
        }}
      >
        <FileTextIcon width={20} height={20} />
      </Box>
      <Box>
        <Typography
          variant="subtitle1"
          sx={{ fontWeight: 400, color: "grey.900", lineHeight: 1.2 }}
        >
          Ongoing Cases
        </Typography>
        <Typography variant="caption" sx={{ color: "grey.600" }}>
          Latest 5 support tickets
        </Typography>
      </Box>
    </Box>
  );
};

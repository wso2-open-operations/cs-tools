import { Box } from "@mui/material";
import React from "react";
import { BotIcon } from "../../../assets/icons/common-icons";

export const CTAIcon: React.FC = () => {
  return (
    <Box
      sx={{
        width: 48,
        height: 48,
        backgroundColor: "#ea580c",
        borderRadius: "8px",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        color: "white",
      }}
    >
      <BotIcon width={24} height={24} />
    </Box>
  );
};

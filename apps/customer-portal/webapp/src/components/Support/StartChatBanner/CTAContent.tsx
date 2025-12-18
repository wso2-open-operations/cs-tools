import { Box, Typography } from "@mui/material";
import React from "react";
import { CTAIcon } from "./CTAIcon";

export const CTAContent: React.FC = () => {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
      <CTAIcon />

      <Box>
        <Typography
          variant="subtitle1"
          sx={{
            fontWeight: 400,
            mb: 0.5,
            color: "#111827",
            fontSize: "1rem",
          }}
        >
          Need help with something new?
        </Typography>

        <Typography variant="body2" sx={{ color: "#4b5563" }}>
          Chat with Novera to get instant assistance or create a new support case
        </Typography>
      </Box>
    </Box>
  );
};

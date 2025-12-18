import { Box, Card } from "@mui/material";
import React from "react";
import { CTAContent } from "./CTAContent";
import { CTAButton } from "./CTAButton";

export const NewChatCTA: React.FC = () => {
  return (
    <Box sx={{ mb: 4 }}>
      <Card
        sx={{
          p: 3,
          background: "linear-gradient(to bottom right, #fff7ed, #ffedd580)",
          border: "1px solid",
          borderColor: "#fed7aa",
          borderRadius: "12px",
          boxShadow: "none",
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            flexWrap: "wrap",
            gap: 2,
          }}
        >
          <CTAContent />
          <CTAButton />
        </Box>
      </Card>
    </Box>
  );
};

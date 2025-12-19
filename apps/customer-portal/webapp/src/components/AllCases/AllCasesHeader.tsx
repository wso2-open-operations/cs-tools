import { Box, Button, Typography } from "@mui/material";
import React from "react";
import { useNavigate } from "react-router-dom";
import { ArrowLeftIcon } from "@/assets/icons/common-icons";

export const AllCasesHeader: React.FC = () => {
  const navigate = useNavigate();

  return (
    <Box sx={{ mb: 3 }}>
      <Button
        startIcon={<ArrowLeftIcon width={16} height={16} />}
        onClick={() => navigate(-1)}
        sx={{
          color: "text.primary",
          textTransform: "none",
          fontWeight: 500,
          mb: 2,
          "&:hover": {
            backgroundColor: "action.hover",
          },
        }}
      >
        Back to Support Center
      </Button>
      <Box>
        <Typography
          variant="h6"
          sx={{ fontWeight: 600, color: "grey.900", mb: 1 }}
        >
          All Cases
        </Typography>
        <Typography variant="body1" sx={{ color: "grey.600" }}>
          Manage and track all your support cases
        </Typography>
      </Box>
    </Box>
  );
};

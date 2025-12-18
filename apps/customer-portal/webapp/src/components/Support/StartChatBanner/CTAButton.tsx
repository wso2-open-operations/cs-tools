import { Button } from "@mui/material";
import React from "react";
import { useNavigate, useParams } from "react-router-dom";

export const CTAButton: React.FC = () => {
  const navigate = useNavigate();
  const { sysId } = useParams<{ sysId: string }>();

  return (
    <Button
      variant="contained"
      onClick={() => navigate(`/${sysId}/support/cases/create-case`)}
      sx={{
        backgroundColor: "#ea580c",
        "&:hover": { backgroundColor: "#c2410c" },
        textTransform: "none",
        fontWeight: 500,
        boxShadow: "none",
        height: 36,
        px: 1.5,
        py: 1,
        fontSize: "0.875rem",
        borderRadius: "6px",
        transition: "all 0.2s",
      }}
    >
      Create Case
    </Button>
  );
};

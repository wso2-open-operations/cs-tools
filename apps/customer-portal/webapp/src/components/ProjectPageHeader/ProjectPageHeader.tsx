// Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
//
// This software is the property of WSO2 LLC. and its suppliers, if any.
// Dissemination of any information or reproduction of any material contained
// herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
// You may not alter or remove any copyright or other notice from copies of this content.

import { Box, Typography } from "@mui/material";

interface ProjectPageHeaderProps {
  projectName: string;
  sysId: string;
}

const ProjectPageHeader: React.FC<ProjectPageHeaderProps> = ({
  projectName,
  sysId,
}) => {
  return (
    <Box
      sx={{
        width: "100%",
        display: "flex",
        justifyContent: "center",
        mt: 4,
      }}
    >
      <Box
        sx={{
          bgcolor: "#fff7ed", // light orange background
          border: "1px solid #fdba74", // orange border
          borderRadius: "12px",
          px: 4,
          py: 3,
          textAlign: "center",
          boxShadow: "0 2px 6px rgba(0,0,0,0.06)",
          maxWidth: "500px", // control width of the box
          width: "100%",
        }}
      >
        <Typography
          variant="h5"
          component="h1"
          sx={{
            color: "#ea580c", // orange text
            fontWeight: 600,
            mb: "4px",
          }}
        >
          {projectName}
        </Typography>

        <Typography
          variant="body2"
          sx={{
            color: "#9a3412", // darker orange text
            fontSize: "0.875rem",
          }}
        >
          Project ID: {sysId}
        </Typography>
      </Box>
    </Box>
  );
};

export default ProjectPageHeader;

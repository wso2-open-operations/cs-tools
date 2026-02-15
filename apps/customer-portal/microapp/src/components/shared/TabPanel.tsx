import { Box } from "@mui/material";

interface TabPanelProps {
  index: number;
  value: number;
  children?: React.ReactNode;
}

export function TabPanel(props: TabPanelProps) {
  const { children, value, index, ...other } = props;

  return (
    <Box role="tabpanel" hidden={value !== index} id={`tab-${index}`} {...other}>
      {value === index && <>{children}</>}
    </Box>
  );
}

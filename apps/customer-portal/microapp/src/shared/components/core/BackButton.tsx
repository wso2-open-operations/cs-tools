import { IconButton, pxToRem } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";

export function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <IconButton
      aria-label="Go back"
      onClick={onClick}
      disableRipple
      sx={{
        p: 0,
        m: 1,
        position: "relative",
        "::after": {
          content: '""',
          position: "absolute",
          inset: "-8px",
        },
      }}
    >
      <ArrowLeft size={pxToRem(20)} />
    </IconButton>
  );
}

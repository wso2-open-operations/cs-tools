import { IconButton, pxToRem } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";

export function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <IconButton aria-label="Go back" onClick={onClick} sx={{ p: 0 }} disableRipple>
      <ArrowLeft size={pxToRem(20)} />
    </IconButton>
  );
}

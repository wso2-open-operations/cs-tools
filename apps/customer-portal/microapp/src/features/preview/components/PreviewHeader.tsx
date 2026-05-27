import { IconButton, Stack, Typography } from "@wso2/oxygen-ui";
import { File, Image, X } from "@wso2/oxygen-ui-icons-react";

interface PreviewHeaderProps {
  fileName: string | undefined;
  isTypeImage: boolean;
  onClose: () => void;
}

export function PreviewHeader({ fileName, isTypeImage, onClose }: PreviewHeaderProps) {
  return (
    <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mt: "var(--safe-top)", p: 1 }}>
      <Stack direction="row" alignItems="center" flex={1} gap={1}>
        {isTypeImage ? <Image size={18} /> : <File size={18} />}

        <Typography noWrap variant="subtitle1">
          {fileName}
        </Typography>
      </Stack>
      <IconButton size="small" onClick={onClose} sx={{ mr: -1 }}>
        <X />
      </IconButton>
    </Stack>
  );
}

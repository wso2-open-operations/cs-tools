import { type ChangeEvent, type DragEvent, useRef, useState } from "react";

import { Card, Divider, IconButton, Stack, Typography } from "@wso2/oxygen-ui";
import { File, Image, Paperclip, Trash } from "@wso2/oxygen-ui-icons-react";

import { useNotify } from "@context/snackbar";

import { toAttachmentFile } from "@shared/utils/attachments.utils";

const MAX_MB = 10;
const MAX_BYTES = MAX_MB * 1024 * 1024;

export interface AttachmentFile {
  id: string;
  name: string;
  size: string;
  type: string;
  raw: File;
}

export function AttachmentField({ onChange }: { onChange?: (attachments: AttachmentFile[]) => void }) {
  const notify = useNotify();
  const ref = useRef<HTMLInputElement | null>(null);
  const [attachments, setAttachments] = useState<AttachmentFile[]>([]);

  const addFiles = (incoming: File[]) => {
    const valid = incoming.filter((f) => f.size <= MAX_BYTES);
    const rejected = incoming.filter((f) => f.size > MAX_BYTES);

    if (rejected.length > 0)
      notify.warn(
        `${rejected.length} file${rejected.length > 1 ? "s" : ""} exceeded the ${MAX_MB}MB limit and were not attached.`,
      );

    const mapped = valid.map(toAttachmentFile);

    setAttachments((prev) => {
      const existingIds = new Set(prev.map((a) => a.id));
      const deduped = mapped.filter((a) => !existingIds.has(a.id));
      const next = [...prev, ...deduped];
      onChange?.(next);
      return next;
    });
  };

  const handleRemove = (id: string) => {
    setAttachments((prev) => {
      const next = prev.filter((a) => a.id !== id);
      onChange?.(next);
      return next;
    });
  };

  const handleClick = () => {
    ref.current?.click();
  };

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files;
    if (!files) return;
    addFiles(Array.from(files));
    event.target.value = "";
  };

  const handleDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();

    const files = event.dataTransfer.files;
    if (!files) return;

    addFiles(Array.from(files));
  };

  const handleDragOver = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
  };

  return (
    <Stack gap={1}>
      <input hidden multiple type="file" ref={ref} onChange={handleFileChange} />
      <Typography variant="subtitle2" color="text.secondary" sx={{ opacity: 0.6 }}>
        Attach any supporting files or documents.
      </Typography>
      <Card
        component={Stack}
        sx={{
          borderStyle: "dashed",
          borderColor: "divider",
          bgcolor: "background.paper",
        }}
      >
        <Stack
          sx={{ p: 2, gap: 2, textAlign: "center", alignItems: "center" }}
          onClick={handleClick}
          onDrop={handleDrop}
          onDragOver={handleDragOver}
        >
          <Paperclip />
          <Stack>
            <Typography variant="body2">Drag and drop files here, or click to browse</Typography>
            <Typography variant="caption" sx={{ opacity: 0.8 }}>
              Max {MAX_MB} MB
            </Typography>
          </Stack>
        </Stack>

        {attachments.length > 0 && (
          <Stack
            sx={{ borderTop: "1px dashed", borderColor: "divider" }}
            divider={<Divider sx={{ borderStyle: "dashed" }} />}
          >
            {attachments.map((attachment) => (
              <AttachmentItem
                key={attachment.id}
                type={attachment.type}
                name={attachment.name}
                size={attachment.size}
                onRemove={() => handleRemove(attachment.id)}
              />
            ))}
          </Stack>
        )}
      </Card>
    </Stack>
  );
}

function AttachmentItem({
  type,
  name,
  size,
  onRemove,
}: {
  type: string;
  name: string;
  size: string;
  onRemove: () => void;
}) {
  return (
    <Stack direction="row" alignItems="center" gap={1} p={1}>
      {type.startsWith("image/") ? <Image size={18} /> : <File size={18} />}
      <Typography variant="body2" noWrap flex={1}>
        {name}
      </Typography>
      <Typography variant="caption" color="text.secondary" flexShrink={0} sx={{ opacity: 0.8 }}>
        {size}
      </Typography>
      <IconButton
        size="small"
        color="error"
        onClick={onRemove}
        sx={{ color: "text.disabled", "&:hover": { color: "error.main" } }}
      >
        <Trash size={16} />
      </IconButton>
    </Stack>
  );
}

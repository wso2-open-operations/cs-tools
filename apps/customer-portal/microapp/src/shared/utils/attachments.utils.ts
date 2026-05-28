import type { AttachmentFile } from "@features/case-types/cases/components";

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

export function toAttachmentFile(file: File): AttachmentFile {
  return {
    id: `${file.name}-${file.lastModified}-${file.size}`,
    name: file.name,
    size: formatBytes(file.size),
    type: file.type,
    raw: file,
  };
}

export function toBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve((reader.result as string).split(",")[1]); // strips the data:*/*;base64, prefix
    reader.onerror = reject;
    reader.readAsDataURL(file);
  });
}

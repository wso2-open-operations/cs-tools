// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Never actually talks to the real Google APIs client-side — fetches folder
// listings from the Go backend's /spl/files proxy endpoint (internal/
// googledrive), same auth/backend pattern as everything else in this port.
import { useState } from "react";
import { openExternalUrl } from "@utils/openExternalUrl";
import { Box, Button } from "@wso2/oxygen-ui";
import {
  ChevronRightIcon,
  ExternalLinkIcon,
  FolderIcon,
  FileTextIcon,
  ImageIcon,
  FilmIcon,
  MusicIcon,
  CodeIcon,
  SheetIcon,
} from "@wso2/oxygen-ui-icons-react";
import { useGetDriveFiles, type DriveFile } from "../api/useAccountsApi";

const FILE_VIEW_BASE_URL = "https://drive.google.com/file/d/";
const FOLDER_VIEW_BASE_URL = "https://drive.google.com/drive/folders/";

function getFileIcon(mimeType: string, name: string) {
  if (mimeType === "application/vnd.google-apps.folder") {
    return <FolderIcon size={18} style={{ verticalAlign: "middle", marginRight: 8, color: "#FFA726" }} />;
  }
  if (mimeType === "application/vnd.google-apps.document") {
    return <FileTextIcon size={18} style={{ verticalAlign: "middle", marginRight: 8, color: "#4285F4" }} />;
  }
  if (mimeType === "application/vnd.google-apps.spreadsheet") {
    return <SheetIcon size={18} style={{ verticalAlign: "middle", marginRight: 8, color: "#0F9D58" }} />;
  }
  const extension = name.toLowerCase().split(".").pop();
  switch (extension) {
    case "pdf":
      return <FileTextIcon size={18} style={{ verticalAlign: "middle", marginRight: 8, color: "#FF5252" }} />;
    case "jpg":
    case "jpeg":
    case "png":
    case "gif":
    case "bmp":
      return <ImageIcon size={18} style={{ verticalAlign: "middle", marginRight: 8, color: "#4CAF50" }} />;
    case "mp4":
    case "avi":
    case "mov":
    case "wmv":
      return <FilmIcon size={18} style={{ verticalAlign: "middle", marginRight: 8, color: "#FF4081" }} />;
    case "mp3":
    case "wav":
    case "ogg":
      return <MusicIcon size={18} style={{ verticalAlign: "middle", marginRight: 8, color: "#7C4DFF" }} />;
    case "js":
    case "ts":
    case "java":
    case "py":
    case "cpp":
    case "c":
    case "h":
    case "cs":
    case "php":
      return <CodeIcon size={18} style={{ verticalAlign: "middle", marginRight: 8, color: "#FFC107" }} />;
    default:
      return <FileTextIcon size={18} style={{ verticalAlign: "middle", marginRight: 8, color: "#757575" }} />;
  }
}

function extractFolderIdFromUrl(url: string): string | null {
  if (!url) return null;
  const match = url.match(/\/folders\/([^/?]+)/);
  return match ? match[1] : null;
}

export function GoogleDrive({ driveLocation }: { driveLocation: string }) {
  const [path, setPath] = useState<{ id: string; name: string }[]>([]);
  const rootFolderId = extractFolderIdFromUrl(driveLocation);
  const folderNotFound = !rootFolderId;
  const currentFolderId = path.length > 0 ? path[path.length - 1].id : (rootFolderId ?? "");

  const { data: files, isLoading: filesLoading, isError, refetch } = useGetDriveFiles(currentFolderId);

  const handleClick = (file: DriveFile) => {
    if (file.mimeType === "application/vnd.google-apps.folder") {
      if (!path.some((p) => p.id === file.id)) setPath([...path, { id: file.id, name: file.name }]);
    } else {
      openExternalUrl(`${FILE_VIEW_BASE_URL}${file.id}/view`);
    }
  };

  const handleBreadcrumbClick = (index: number) => {
    setPath(path.slice(0, index + 1));
  };

  const goToCurrentFolder = () => {
    if (currentFolderId) openExternalUrl(`${FOLDER_VIEW_BASE_URL}${currentFolderId}`);
  };

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Box sx={{ display: "flex", alignItems: "center" }}>
          <Button onClick={() => setPath([])} title="Back to root">
            {folderNotFound ? "No Folder Found" : filesLoading ? "Loading…" : "Root"}
          </Button>
          {path.map((p, index) => (
            <Box key={index} sx={{ display: "flex", alignItems: "center" }}>
              <ChevronRightIcon size={16} style={{ margin: "0 8px", opacity: 0.5 }} />
              <Button onClick={() => handleBreadcrumbClick(index)} title={p.name}>
                {p.name}
              </Button>
            </Box>
          ))}
        </Box>
        {!folderNotFound && path.length > 0 && (
          <Button variant="contained" onClick={goToCurrentFolder} sx={{ minWidth: 0, p: 0.5 }}>
            <ExternalLinkIcon size={20} />
          </Button>
        )}
      </Box>

      {filesLoading ? (
        <Box sx={{ py: 3, textAlign: "center", color: "text.secondary" }}>Loading folder contents…</Box>
      ) : isError ? (
        <Box sx={{ py: 3, textAlign: "center" }}>
          <Box sx={{ color: "text.secondary", mb: 1 }}>An error occurred while fetching data. Please try again.</Box>
          <Button variant="contained" onClick={() => void refetch()}>
            Retry
          </Button>
        </Box>
      ) : folderNotFound ? (
        <Box sx={{ py: 3, textAlign: "center" }}>
          <Box sx={{ color: "text.secondary", mb: 1 }}>No folder found at the specified drive location.</Box>
          <Button variant="contained" onClick={() => void refetch()}>
            Refresh
          </Button>
        </Box>
      ) : !files || files.length === 0 ? (
        <Box sx={{ py: 3, textAlign: "center", color: "text.secondary" }}>This folder is currently empty.</Box>
      ) : (
        <Box component="table" sx={{ width: "100%", borderCollapse: "collapse" }}>
          <Box component="thead">
            <Box component="tr">
              <Box
                component="th"
                sx={{ textAlign: "left", py: 1, borderBottomWidth: 1, borderBottomStyle: "solid", borderColor: "divider" }}
              >
                Name
              </Box>
            </Box>
          </Box>
          <Box component="tbody">
            {files
              .slice()
              .sort((a, b) => {
                const aIsFolder = a.mimeType === "application/vnd.google-apps.folder";
                const bIsFolder = b.mimeType === "application/vnd.google-apps.folder";
                if (aIsFolder && !bIsFolder) return -1;
                if (!aIsFolder && bIsFolder) return 1;
                return a.name.localeCompare(b.name, undefined, { numeric: true });
              })
              .map((file) => (
                <Box
                  component="tr"
                  key={file.id}
                  onClick={() => handleClick(file)}
                  sx={{ cursor: "pointer", "&:hover": { backgroundColor: "action.hover" } }}
                >
                  <Box component="td" sx={{ py: 1 }}>
                    {getFileIcon(file.mimeType, file.name)}
                    <Box component="span" sx={{ verticalAlign: "middle" }}>
                      {file.name}
                    </Box>
                  </Box>
                </Box>
              ))}
          </Box>
        </Box>
      )}
    </Box>
  );
}

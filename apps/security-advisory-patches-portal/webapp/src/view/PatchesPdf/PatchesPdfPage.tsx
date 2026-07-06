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

import React, { useEffect, useMemo, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { Box, CircularProgress, Typography } from '@mui/material';
import Header from '@src/layout/Header';
import { pathnameToPdfStoragePath } from '@src/utils/utils';
import { downloadSecurityAdvisory, getFileName } from '@src/utils/fileService';
import NotFoundPage from '@src/view/NotFound/NotFoundPage';

interface PatchesPdfPageProps {
  username?: string;
  onLogout: () => void;
}

const PatchesPdfPage: React.FC<PatchesPdfPageProps> = ({ username, onLogout }) => {
  const location = useLocation();
  const storagePath = useMemo(() => pathnameToPdfStoragePath(location.pathname), [location.pathname]);

  const [blobUrl, setBlobUrl] = useState<string | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!storagePath) {
      setNotFound(true);
      setLoading(false);
      return;
    }

    let cancelled = false;
    let objectUrl = '';

    const run = async () => {
      setLoading(true);
      setNotFound(false);
      setBlobUrl(null);

      try {
        const blob = await downloadSecurityAdvisory(storagePath);
        if (cancelled) {
          return;
        }
        objectUrl = URL.createObjectURL(blob);
        setBlobUrl(objectUrl);
      } catch {
        if (!cancelled) {
          setNotFound(true);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    run();

    return () => {
      cancelled = true;
      if (objectUrl) {
        URL.revokeObjectURL(objectUrl);
      }
    };
  }, [storagePath]);

  if (notFound) {
    return <NotFoundPage username={username} onLogout={onLogout} />;
  }

  const displayName = storagePath ? getFileName(storagePath) : '';

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#f5f5f5', display: 'flex', flexDirection: 'column' }}>
      <Header username={username} onLogout={onLogout} />
      <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0, p: { xs: 1, sm: 2 } }}>
        {displayName ? (
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1, px: 0.5 }} noWrap title={displayName}>
            {displayName}
          </Typography>
        ) : null}

        {loading && (
          <Box
            sx={{
              flex: 1,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              minHeight: 240,
            }}
          >
            <CircularProgress />
          </Box>
        )}

        {!loading && blobUrl && (
          <Box
            component="iframe"
            title={displayName || 'PDF preview'}
            src={blobUrl}
            sx={{
              flex: 1,
              width: '100%',
              minHeight: { xs: '70vh', md: 'calc(100vh - 120px)' },
              border: 'none',
              borderRadius: 1,
              bgcolor: '#525659',
            }}
          />
        )}
      </Box>
    </Box>
  );
};

export default PatchesPdfPage;

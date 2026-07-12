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

import { Suspense, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  Avatar,
  Button,
  Divider,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Skeleton,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMutation, useQueryClient, useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { users } from "@src/services/users";
import type { UserMeUpdateDto, UserProfile } from "@src/types";
import { useUserStore } from "@src/store/user";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { ErrorState } from "@components/support/ErrorState";
import { listSupportedTimeZones } from "@utils/dateTime";
import { initialsOf } from "@utils/initials";

export default function ProfilePage() {
  return (
    <Stack gap={2}>
      <Typography variant="h5">Profile</Typography>

      <ProfileErrorBoundary>
        <Suspense fallback={<ProfileSkeleton />}>
          <ProfileContent />
        </Suspense>
      </ProfileErrorBoundary>
    </Stack>
  );
}

function ProfileContent() {
  const { data: profile } = useSuspenseQuery(users.me());
  const queryClient = useQueryClient();
  const timeZoneOptions = useMemo(() => listSupportedTimeZones(), []);

  const [phoneNumber, setPhoneNumber] = useState(profile.phoneNumber ?? "");
  const [timeZone, setTimeZone] = useState(profile.timeZone ?? "");

  const isDirty = phoneNumber !== (profile.phoneNumber ?? "") || timeZone !== (profile.timeZone ?? "");

  // A background refetch (window refocus, cache invalidation elsewhere) must not clobber an
  // in-progress edit. Only re-seed from the server once the local draft matches it again, e.g.
  // right after this page's own save invalidates the query.
  useEffect(() => {
    if (isDirty) return;
    setPhoneNumber(profile.phoneNumber ?? "");
    setTimeZone(profile.timeZone ?? "");
  }, [profile, isDirty]);

  const mutation = useMutation({
    mutationFn: users.updateMe,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["users", "me"] });
    },
  });

  const handleSave = () => {
    const payload: UserMeUpdateDto = {};
    if (phoneNumber !== (profile.phoneNumber ?? "")) payload.phoneNumber = phoneNumber;
    if (timeZone !== (profile.timeZone ?? "")) payload.timeZone = timeZone;
    if (Object.keys(payload).length === 0) return;
    mutation.mutate(payload);
  };

  return (
    <Stack gap={2.5}>
      <ProfileHeader profile={profile} />
      <Divider />

      <Stack gap={2}>
        <TextField label="Email" value={profile.email} size="small" disabled fullWidth />

        <TextField
          label="Phone Number"
          value={phoneNumber}
          onChange={(event) => {
            if (mutation.isSuccess || mutation.isError) mutation.reset();
            setPhoneNumber(event.target.value);
          }}
          size="small"
          fullWidth
          disabled={mutation.isPending}
        />

        <FormControl size="small" fullWidth disabled={mutation.isPending}>
          <InputLabel id="profile-timezone-label" shrink>
            Time Zone
          </InputLabel>
          <Select
            labelId="profile-timezone-label"
            label="Time Zone"
            value={timeZone}
            displayEmpty
            notched
            onChange={(event) => {
              if (mutation.isSuccess || mutation.isError) mutation.reset();
              setTimeZone(event.target.value);
            }}
          >
            <MenuItem value="" disabled>
              <em>Select a time zone</em>
            </MenuItem>
            {timeZoneOptions.map((tz) => (
              <MenuItem key={tz} value={tz}>
                {tz}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <Button variant="contained" onClick={handleSave} disabled={!isDirty || mutation.isPending}>
          {mutation.isPending ? "Saving..." : "Save Changes"}
        </Button>

        {mutation.isSuccess && (
          <Typography variant="body2" color="success.main">
            Profile updated.
          </Typography>
        )}
        {mutation.isError && (
          <Typography variant="body2" color="error.main">
            Failed to update profile. Please try again.
          </Typography>
        )}
      </Stack>
    </Stack>
  );
}

function ProfileHeader({ profile }: { profile: UserProfile }) {
  // The photo isn't part of the backend /users/me profile — it only lives in the ID token
  // claims (decodeTokenAndStoreUser in auth.ts), so it's read from the user store directly here,
  // same as the TopBar's avatar (components/layout/TopBar.tsx).
  const avatarUrl = useUserStore((state) => state.user?.avatarUrl);
  const initials = initialsOf(profile.fullName);

  return (
    <Stack direction="row" alignItems="center" gap={2}>
      <Avatar src={avatarUrl} slotProps={{ img: { referrerPolicy: "no-referrer" } }} sx={{ width: 56, height: 56 }}>
        {initials}
      </Avatar>
      <Typography variant="h6">{profile.fullName}</Typography>
    </Stack>
  );
}

function ProfileSkeleton() {
  return (
    <Stack gap={2.5}>
      <Stack direction="row" alignItems="center" gap={2}>
        <Skeleton variant="circular" width={56} height={56} />
        <Skeleton variant="text" width={140} height={28} />
      </Stack>
      <Skeleton variant="rounded" height={40} />
      <Skeleton variant="rounded" height={40} />
      <Skeleton variant="rounded" height={40} />
    </Stack>
  );
}

function ProfileErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}

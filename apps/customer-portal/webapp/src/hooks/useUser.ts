// Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
//
// This software is the property of WSO2 LLC. and its suppliers, if any.
// Dissemination of any information or reproduction of any material contained
// herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
// You may not alter or remove any copyright or other notice from copies of this content.

import { useQuery } from "@tanstack/react-query";
import { useAuthContext } from "@asgardeo/auth-react";

export interface UserInfo {
  sub: string;
  email: string;
  name: string;
  given_name?: string;
  family_name?: string;
  picture?: string;
  email_verified?: boolean;
}

// Mock user info - replace with actual API call
const fetchUserInfo = async (idToken: string): Promise<UserInfo> => {
  // For now, we'll decode the ID token to get user info
  // In production, you would call your backend API here
  try {
    const payload = JSON.parse(atob(idToken.split(".")[1]));
    return {
      sub: payload.sub,
      email: payload.email || "",
      name: payload.name || payload.given_name || "User",
      given_name: payload.given_name,
      family_name: payload.family_name,
      picture: payload.picture,
      email_verified: payload.email_verified,
    };
  } catch (error) {
    console.error("Error decoding ID token:", error);
    throw new Error("Failed to fetch user information");
  }
};

export const useUserQuery = () => {
  const { getIDToken, isAuthenticated } = useAuthContext();

  return useQuery({
    queryKey: ["user"],
    queryFn: async () => {
      const isAuth = await isAuthenticated();
      if (!isAuth) {
        throw new Error("User is not authenticated");
      }
      const idToken = await getIDToken();
      return fetchUserInfo(idToken);
    },
    enabled: true,
    staleTime: 5 * 60 * 1000, // 5 minutes
    retry: 1,
  });
};

// Helper function to get initials from name
export const getInitialsFromName = (name: string): string => {
  if (!name) return "U";

  const parts = name.trim().split(" ");
  if (parts.length === 1) {
    return parts[0].charAt(0).toUpperCase();
  }

  return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase();
};

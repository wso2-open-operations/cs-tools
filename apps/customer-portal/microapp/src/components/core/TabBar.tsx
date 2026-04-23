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

import { Link } from "react-router-dom";
import { BottomNavigation, BottomNavigationAction, Box } from "@wso2/oxygen-ui";
import { House, MessageSquare, User, Users } from "@wso2/oxygen-ui-icons-react";
import { useLayout } from "@src/context/layout";

export function TabBar() {
  const { activeTabIndex } = useLayout();

  return (
    <Box position="fixed" bgcolor="background.paper" bottom={0} left={0} right={0} pt={1} pb={5}>
      <BottomNavigation value={activeTabIndex} showLabels>
        <BottomNavigationAction component={Link} to="/" label="Home" icon={<House />} disableRipple />
        <BottomNavigationAction component={Link} to="/support" label="Support" icon={<MessageSquare />} disableRipple />
        <BottomNavigationAction component={Link} to="/users" label="Users" icon={<Users />} disableRipple />
        <BottomNavigationAction component={Link} to="/profile" label="Profile" icon={<User />} disableRipple />
      </BottomNavigation>
    </Box>
  );
}

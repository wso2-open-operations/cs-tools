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

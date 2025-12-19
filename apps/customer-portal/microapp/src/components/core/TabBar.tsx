import { ChatBubble, Home, PeopleAlt, Person } from "@mui/icons-material";
import { BottomNavigation, BottomNavigationAction, Box } from "@mui/material";
import { Link } from "react-router-dom";

import { useLayout } from "@src/context/layout";

export function TabBar() {
  const { activeTabIndex } = useLayout();

  return (
    <Box position="fixed" bgcolor="background.paper" bottom={0} left={0} right={0} pt={1} pb={5}>
      <BottomNavigation value={activeTabIndex} showLabels>
        <BottomNavigationAction component={Link} to="/" label="Home" icon={<Home />} disableRipple />
        <BottomNavigationAction component={Link} to="/support" label="Support" icon={<ChatBubble />} disableRipple />
        <BottomNavigationAction component={Link} to="/users" label="Users" icon={<PeopleAlt />} disableRipple />
        <BottomNavigationAction component={Link} to="/profile" label="Profile" icon={<Person />} disableRipple />
      </BottomNavigation>
    </Box>
  );
}

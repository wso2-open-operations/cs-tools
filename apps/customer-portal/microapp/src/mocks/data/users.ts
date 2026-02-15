import type { UserListItemProps } from "@components/features/users";

export const MOCK_METRICS = [
  { label: "Total Users", value: 12 },
  { label: "Active", value: 4 },
  { label: "Admins", value: 1 },
];

export const MOCK_USERS: UserListItemProps[] = [
  {
    name: "Lithika Damnod",
    email: "user@example.com",
    role: "admin",
    lastActive: "2 hours ago",
  },
  {
    name: "Sarah Chan",
    email: "user@example.com",
    role: "developer",
    lastActive: "5 hours ago",
  },
  {
    name: "Mike Johnson",
    email: "user@example.com",
    role: "security",
    lastActive: "1 day ago",
  },
  {
    name: "Emily Rodriguez",
    email: "user@example.com",
    role: "procurement",
    lastActive: "3 days ago",
  },
  {
    name: "David Kim",
    email: "user@example.com",
    role: "manager",
    lastActive: "2 weeks ago",
  },
];

export const MOCK_ROLES = [
  {
    name: "Admin",
    description: "Full access to manage project, users, and settings",
  },
  {
    name: "Developer",
    description: "Create and manage cases, chats, and requests",
  },
  {
    name: "Security",
    description: "Security-focused access and monitoring",
  },
  {
    name: "Procurement",
    description: "Procurement and purchasing management",
  },
  {
    name: "Manager",
    description: "Team oversight and reporting access",
  },
];

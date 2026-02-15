import type { ActivityTimelineEntryProps } from "@components/features/detail";

export const MOCK_STAKEHOLDERS = [
  { name: "Sarah Chen", role: "Change Owner" },
  { name: "Security Team", role: "Requestor" },
];

export const MOCK_IMPLEMENTATION_STEPS = [
  {
    title: "Pre-change verification",
    description: "Verify current configuration and create backup",
    timestamp: "10 minutes ago",
  },
  {
    title: "Apply new security policies",
    description: "Deploy updated policy configuration to API Gateway",
    timestamp: "10 minutes ago",
  },
  {
    title: "Update rate limiting rules",
    description: "Configure enhanced rate limiting thresholds",
    timestamp: "10 minutes ago",
  },
  {
    title: "Testing and validation",
    description: "Run automated tests to verify policy enforcement",
    timestamp: "10 minutes ago",
  },
  {
    title: "Monitoring",
    description: "Monitor system for 5 minutes post-deployment",
    timestamp: "10 minutes ago",
  },
];

export const MOCK_ACTIVITY_TIMELINE: Omit<ActivityTimelineEntryProps, "variant">[] = [
  {
    author: "Change Advisory Board",
    description: "Change request approved and scheduled for Nov 25, 2025",
    timestamp: "10 minutes ago",
  },
  {
    author: "Sarah Chen",
    description: "Completed impact assessment. Risk level: Low.",
    timestamp: "10 minutes ago",
  },
  {
    author: "Security Team",
    description: "Added rollback plan documentation.",
    timestamp: "10 minutes ago",
  },
  {
    author: "System",
    description: "Change request assigned to Sarah Chen",
    timestamp: "10 minutes ago",
  },
  {
    author: "Security Team",
    description: "Change request created",
    timestamp: "10 minutes ago",
  },
];

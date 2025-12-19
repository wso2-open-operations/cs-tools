import { ServerIcon } from "../../../assets/icons/common-icons";
import { InfoCard } from "./InfoCard";

export const ServiceRequestCard = () => (
  <InfoCard
    icon={<ServerIcon width={20} height={20} />}
    title="Service Requests"
    subtitle="Manage deployment operations"
    colorTheme="blue"
    descriptionTitle="What are Service Requests?"
    descriptionText="Request operational changes to your managed cloud deployment:"
    listItems={[
      "Service restarts and upgrades",
      "Certificate management",
      "Infrastructure scaling",
      "Configuration changes",
      "Log and information requests",
      "Security updates",
    ]}
    primaryAction={{
      label: "New Service Request",
      icon: <ServerIcon width={16} height={16} />,
    }}
    secondaryAction={{
      label: "View All Service Requests",
    }}
  />
);

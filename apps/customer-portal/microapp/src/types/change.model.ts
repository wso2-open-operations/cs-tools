export interface ChangeRequestSummary {
  id: string;
  number: string;
  title: string;
  description: string;
  requestType?: string;
  impactId?: string;
  statusId?: string;
  owner?: string;
  scheduledOn?: Date;
  createdOn: Date;
  updatedOn: Date;
}

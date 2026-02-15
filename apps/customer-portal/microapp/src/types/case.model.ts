export interface Case {
  id: string;
  internalId: string;
  number: string;
  createdOn: Date;
  title: string;
  description?: string;
  assigned?: string;
  statusId?: string;
  severityId?: string;
  issueTypeId?: string;
}

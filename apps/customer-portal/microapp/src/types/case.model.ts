export interface Case {
  id: string;
  internalId: string;
  number: string;
  createdOn: Date;
  updatedOn: Date;
  title: string;
  description: string;
  slaResponseTime: string;
  assigned?: string;
  statusId?: string;
  severityId?: string;
  issueTypeId?: string;
  product?: string;
  productVersion?: string;
  reporter?: string;
  account?: string;
  parentCaseId?: string;
  conversationId?: string;
  deployment?: string;
}

export type CaseSummary = Omit<
  Case,
  | "updatedOn"
  | "productId"
  | "reporter"
  | "account"
  | "parentCaseId"
  | "conversationId"
  | "slaResponseTime"
  | "deployment"
>;

export interface Comment {
  id: string;
  content: string;
  createdOn: Date;
  createdBy: string;
  attachments: Attachment[];
}

export interface Attachment {
  id: string;
  fileName: string;
  downloadUrl: string;
  createdOn: Date;
  createdBy: string;
}

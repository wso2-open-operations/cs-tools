export interface MeDTO {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  timeZone: string;
}

export type UsersDTO = UserDTO[];

export interface UserDTO {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  isCsAdmin: boolean;
  isCsIntegrationUser: boolean;
  isSecurityContact: boolean;
  membershipStatus: string;
}

export interface CreateContactRequestDTO {
  contactEmail: string;
  contactFirstName: string;
  contactLastName: string;
  isCsIntegrationUser: boolean;
  isSecurityContact: boolean;
}

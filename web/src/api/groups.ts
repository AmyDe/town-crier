import type { ApiClient } from './client';
import type {
  GroupSummary,
  GroupDetail,
  CreateGroupRequest,
  InviteMemberRequest,
  GroupInvitation,
} from '../domain/types';

export function groupsApi(client: ApiClient) {
  return {
    list: () => client.get<readonly GroupSummary[]>('/v1/groups'),
    getById: (groupId: string) => client.get<GroupDetail>(`/v1/groups/${groupId}`),
    create: (data: CreateGroupRequest) => client.post<GroupDetail>('/v1/groups', data),
    delete: (groupId: string) => client.delete(`/v1/groups/${groupId}`),
    inviteMember: (groupId: string, data: InviteMemberRequest) =>
      client.post<GroupInvitation>(`/v1/groups/${groupId}/invitations`, data),
    removeMember: (groupId: string, memberUserId: string) =>
      client.delete(`/v1/groups/${groupId}/members/${memberUserId}`),
    acceptInvitation: (invitationId: string) =>
      client.post<void>(`/v1/invitations/${invitationId}/accept`),
  };
}

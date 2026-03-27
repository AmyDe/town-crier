import type {
  GroupSummary,
  GroupDetail,
  GroupId,
  InvitationId,
  CreateGroupRequest,
  InviteMemberRequest,
  GroupInvitation,
} from '../types';

export interface GroupsRepository {
  listGroups(): Promise<readonly GroupSummary[]>;
  getGroup(groupId: GroupId): Promise<GroupDetail>;
  createGroup(request: CreateGroupRequest): Promise<GroupDetail>;
  deleteGroup(groupId: GroupId): Promise<void>;
  inviteMember(groupId: GroupId, request: InviteMemberRequest): Promise<GroupInvitation>;
  removeMember(groupId: GroupId, memberUserId: string): Promise<void>;
  acceptInvitation(invitationId: InvitationId): Promise<void>;
}

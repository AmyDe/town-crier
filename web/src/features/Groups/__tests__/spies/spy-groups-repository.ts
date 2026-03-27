import type {
  GroupSummary,
  GroupDetail,
  GroupId,
  InvitationId,
  CreateGroupRequest,
  InviteMemberRequest,
  GroupInvitation,
} from '../../../../domain/types';
import type { GroupsRepository } from '../../../../domain/ports/groups-repository';

export class SpyGroupsRepository implements GroupsRepository {
  listGroupsCalls = 0;
  listGroupsResult: readonly GroupSummary[] = [];
  listGroupsError: Error | null = null;

  async listGroups(): Promise<readonly GroupSummary[]> {
    this.listGroupsCalls += 1;
    if (this.listGroupsError) throw this.listGroupsError;
    return this.listGroupsResult;
  }

  getGroupCalls: GroupId[] = [];
  getGroupResult: GroupDetail | null = null;
  getGroupError: Error | null = null;

  async getGroup(groupId: GroupId): Promise<GroupDetail> {
    this.getGroupCalls.push(groupId);
    if (this.getGroupError) throw this.getGroupError;
    if (!this.getGroupResult) throw new Error('getGroupResult not configured');
    return this.getGroupResult;
  }

  createGroupCalls: CreateGroupRequest[] = [];
  createGroupResult: GroupDetail | null = null;
  createGroupError: Error | null = null;

  async createGroup(request: CreateGroupRequest): Promise<GroupDetail> {
    this.createGroupCalls.push(request);
    if (this.createGroupError) throw this.createGroupError;
    if (!this.createGroupResult) throw new Error('createGroupResult not configured');
    return this.createGroupResult;
  }

  deleteGroupCalls: GroupId[] = [];
  deleteGroupError: Error | null = null;

  async deleteGroup(groupId: GroupId): Promise<void> {
    this.deleteGroupCalls.push(groupId);
    if (this.deleteGroupError) throw this.deleteGroupError;
  }

  inviteMemberCalls: Array<{ groupId: GroupId; request: InviteMemberRequest }> = [];
  inviteMemberResult: GroupInvitation | null = null;
  inviteMemberError: Error | null = null;

  async inviteMember(groupId: GroupId, request: InviteMemberRequest): Promise<GroupInvitation> {
    this.inviteMemberCalls.push({ groupId, request });
    if (this.inviteMemberError) throw this.inviteMemberError;
    if (!this.inviteMemberResult) throw new Error('inviteMemberResult not configured');
    return this.inviteMemberResult;
  }

  removeMemberCalls: Array<{ groupId: GroupId; memberUserId: string }> = [];
  removeMemberError: Error | null = null;

  async removeMember(groupId: GroupId, memberUserId: string): Promise<void> {
    this.removeMemberCalls.push({ groupId, memberUserId });
    if (this.removeMemberError) throw this.removeMemberError;
  }

  acceptInvitationCalls: InvitationId[] = [];
  acceptInvitationError: Error | null = null;

  async acceptInvitation(invitationId: InvitationId): Promise<void> {
    this.acceptInvitationCalls.push(invitationId);
    if (this.acceptInvitationError) throw this.acceptInvitationError;
  }
}

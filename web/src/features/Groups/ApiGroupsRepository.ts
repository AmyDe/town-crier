import type { ApiClient } from '../../api/client';
import type { GroupsRepository } from '../../domain/ports/groups-repository';
import type {
  GroupSummary,
  GroupDetail,
  GroupId,
  InvitationId,
  CreateGroupRequest,
  InviteMemberRequest,
  GroupInvitation,
} from '../../domain/types';
import { groupsApi } from '../../api/groups';

export class ApiGroupsRepository implements GroupsRepository {
  private readonly api: ReturnType<typeof groupsApi>;

  constructor(client: ApiClient) {
    this.api = groupsApi(client);
  }

  listGroups(): Promise<readonly GroupSummary[]> {
    return this.api.list();
  }

  getGroup(groupId: GroupId): Promise<GroupDetail> {
    return this.api.getById(groupId);
  }

  createGroup(request: CreateGroupRequest): Promise<GroupDetail> {
    return this.api.create(request);
  }

  deleteGroup(groupId: GroupId): Promise<void> {
    return this.api.delete(groupId);
  }

  inviteMember(groupId: GroupId, request: InviteMemberRequest): Promise<GroupInvitation> {
    return this.api.inviteMember(groupId, request);
  }

  removeMember(groupId: GroupId, memberUserId: string): Promise<void> {
    return this.api.removeMember(groupId, memberUserId);
  }

  acceptInvitation(invitationId: InvitationId): Promise<void> {
    return this.api.acceptInvitation(invitationId);
  }
}

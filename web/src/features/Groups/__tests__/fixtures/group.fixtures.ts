import type {
  GroupSummary,
  GroupDetail,
  GroupMember,
  GroupInvitation,
} from '../../../../domain/types';
import { asGroupId, asInvitationId, asAuthorityId } from '../../../../domain/types';

export function ownerGroupSummary(
  overrides?: Partial<GroupSummary>,
): GroupSummary {
  return {
    groupId: asGroupId('group-001'),
    name: 'Mill Road Residents',
    role: 'Owner',
    memberCount: 3,
    ...overrides,
  };
}

export function memberGroupSummary(
  overrides?: Partial<GroupSummary>,
): GroupSummary {
  return {
    groupId: asGroupId('group-002'),
    name: 'Castle Hill Watch',
    role: 'Member',
    memberCount: 7,
    ...overrides,
  };
}

export function ownerMember(
  overrides?: Partial<GroupMember>,
): GroupMember {
  return {
    userId: 'user-owner',
    role: 'Owner',
    joinedAt: '2026-01-15T10:00:00Z',
    ...overrides,
  };
}

export function regularMember(
  overrides?: Partial<GroupMember>,
): GroupMember {
  return {
    userId: 'user-member-1',
    role: 'Member',
    joinedAt: '2026-02-10T14:30:00Z',
    ...overrides,
  };
}

export function groupDetail(
  overrides?: Partial<GroupDetail>,
): GroupDetail {
  return {
    groupId: asGroupId('group-001'),
    name: 'Mill Road Residents',
    ownerId: 'user-owner',
    latitude: 52.2044,
    longitude: 0.1218,
    radiusMetres: 2000,
    authorityId: asAuthorityId(101),
    members: [ownerMember(), regularMember()],
    ...overrides,
  };
}

export function sampleInvitation(
  overrides?: Partial<GroupInvitation>,
): GroupInvitation {
  return {
    invitationId: asInvitationId('inv-001'),
    groupId: asGroupId('group-001'),
    inviteeEmail: 'newuser@example.com',
    ...overrides,
  };
}

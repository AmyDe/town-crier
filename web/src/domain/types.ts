// ---------------------------------------------------------------------------
// Branded types — prevent mixing up IDs that are structurally identical
// ---------------------------------------------------------------------------

type Brand<T, B extends string> = T & { readonly __brand: B };

export type ApplicationUid = Brand<string, "ApplicationUid">;
export type WatchZoneId = Brand<string, "WatchZoneId">;
export type GroupId = Brand<string, "GroupId">;
export type InvitationId = Brand<string, "InvitationId">;
export type AuthorityId = Brand<number, "AuthorityId">;

export function asApplicationUid(value: string): ApplicationUid {
  return value as ApplicationUid;
}

export function asWatchZoneId(value: string): WatchZoneId {
  return value as WatchZoneId;
}

export function asGroupId(value: string): GroupId {
  return value as GroupId;
}

export function asInvitationId(value: string): InvitationId {
  return value as InvitationId;
}

export function asAuthorityId(value: number): AuthorityId {
  return value as AuthorityId;
}

// ---------------------------------------------------------------------------
// Union types (no enums — erasableSyntaxOnly is enabled)
// ---------------------------------------------------------------------------

export type ApplicationStatus =
  | "Undecided"
  | "Approved"
  | "Refused"
  | "Withdrawn"
  | "Appealed"
  | "Not Available";

const APPLICATION_STATUSES: readonly string[] = [
  "Undecided",
  "Approved",
  "Refused",
  "Withdrawn",
  "Appealed",
  "Not Available",
];

export function isApplicationStatus(value: unknown): value is ApplicationStatus {
  return typeof value === "string" && APPLICATION_STATUSES.includes(value);
}

export type SubscriptionTier = "Free" | "Pro";

const SUBSCRIPTION_TIERS: readonly string[] = ["Free", "Pro"];

export function isSubscriptionTier(value: unknown): value is SubscriptionTier {
  return typeof value === "string" && SUBSCRIPTION_TIERS.includes(value);
}

export type GroupRole = "Owner" | "Member";

const GROUP_ROLES: readonly string[] = ["Owner", "Member"];

export function isGroupRole(value: unknown): value is GroupRole {
  return typeof value === "string" && GROUP_ROLES.includes(value);
}

export type InvitationStatus = "Pending" | "Accepted" | "Declined";

const INVITATION_STATUSES: readonly string[] = ["Pending", "Accepted", "Declined"];

export function isInvitationStatus(value: unknown): value is InvitationStatus {
  return typeof value === "string" && INVITATION_STATUSES.includes(value);
}

// ---------------------------------------------------------------------------
// Shared value types
// ---------------------------------------------------------------------------

export interface Coordinates {
  readonly latitude: number;
  readonly longitude: number;
}

// ---------------------------------------------------------------------------
// User profile
// ---------------------------------------------------------------------------

export interface UserProfile {
  readonly userId: string;
  readonly postcode: string | null;
  readonly pushEnabled: boolean;
  readonly tier: SubscriptionTier;
}

export interface ZoneNotificationPreferences {
  readonly zoneId: WatchZoneId;
  readonly newApplications: boolean;
  readonly statusChanges: boolean;
  readonly decisionUpdates: boolean;
}

// ---------------------------------------------------------------------------
// Watch zones
// ---------------------------------------------------------------------------

export interface WatchZoneSummary {
  readonly id: WatchZoneId;
  readonly name: string;
  readonly latitude: number;
  readonly longitude: number;
  readonly radiusMetres: number;
  readonly authorityId: AuthorityId;
}

// ---------------------------------------------------------------------------
// Planning applications
// ---------------------------------------------------------------------------

export interface PlanningApplication {
  readonly name: string;
  readonly uid: ApplicationUid;
  readonly areaName: string;
  readonly areaId: AuthorityId;
  readonly address: string;
  readonly postcode: string | null;
  readonly description: string;
  readonly appType: string;
  readonly appState: string;
  readonly appSize: string | null;
  readonly startDate: string | null;
  readonly decidedDate: string | null;
  readonly consultedDate: string | null;
  readonly longitude: number | null;
  readonly latitude: number | null;
  readonly url: string | null;
  readonly link: string | null;
  readonly lastDifferent: string;
}

export interface PlanningApplicationSummary {
  readonly uid: ApplicationUid;
  readonly name: string;
  readonly address: string;
  readonly postcode: string | null;
  readonly description: string;
  readonly appType: string;
  readonly appState: string;
  readonly areaName: string;
  readonly startDate: string | null;
  readonly url: string | null;
}

// ---------------------------------------------------------------------------
// Saved applications
// ---------------------------------------------------------------------------

export interface SavedApplication {
  readonly applicationUid: ApplicationUid;
  readonly savedAt: string;
  readonly application: PlanningApplicationSummary;
}

// ---------------------------------------------------------------------------
// Notifications
// ---------------------------------------------------------------------------

export interface NotificationItem {
  readonly applicationName: string;
  readonly applicationAddress: string;
  readonly applicationDescription: string;
  readonly applicationType: string;
  readonly authorityId: AuthorityId;
  readonly createdAt: string;
}

// ---------------------------------------------------------------------------
// Authorities
// ---------------------------------------------------------------------------

export interface AuthorityListItem {
  readonly id: AuthorityId;
  readonly name: string;
  readonly areaType: string;
}

export interface AuthorityDetail {
  readonly id: AuthorityId;
  readonly name: string;
  readonly areaType: string;
  readonly councilUrl: string | null;
  readonly planningUrl: string | null;
}

// ---------------------------------------------------------------------------
// Geocoding
// ---------------------------------------------------------------------------

export interface GeocodeResult {
  readonly latitude: number;
  readonly longitude: number;
}

// ---------------------------------------------------------------------------
// Designations
// ---------------------------------------------------------------------------

export interface DesignationContext {
  readonly isWithinConservationArea: boolean;
  readonly conservationAreaName: string | null;
  readonly isWithinListedBuildingCurtilage: boolean;
  readonly listedBuildingGrade: string | null;
  readonly isWithinArticle4Area: boolean;
}

// ---------------------------------------------------------------------------
// Groups
// ---------------------------------------------------------------------------

export interface GroupMember {
  readonly userId: string;
  readonly role: GroupRole;
  readonly joinedAt: string;
}

export interface GroupDetail {
  readonly groupId: GroupId;
  readonly name: string;
  readonly ownerId: string;
  readonly latitude: number;
  readonly longitude: number;
  readonly radiusMetres: number;
  readonly authorityId: AuthorityId;
  readonly members: readonly GroupMember[];
}

export interface GroupSummary {
  readonly groupId: GroupId;
  readonly name: string;
  readonly role: GroupRole;
  readonly memberCount: number;
}

export interface GroupInvitation {
  readonly invitationId: InvitationId;
  readonly groupId: GroupId;
  readonly inviteeEmail: string;
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

export interface CreateWatchZoneRequest {
  readonly name: string;
  readonly latitude: number;
  readonly longitude: number;
  readonly radiusMetres: number;
  readonly authorityId: number;
}

export interface UpdateProfileRequest {
  readonly postcode: string | null;
  readonly pushEnabled: boolean;
}

export interface UpdateZonePreferencesRequest {
  readonly newApplications: boolean;
  readonly statusChanges: boolean;
  readonly decisionUpdates: boolean;
}

export interface CreateGroupRequest {
  readonly name: string;
  readonly latitude: number;
  readonly longitude: number;
  readonly radiusMetres: number;
  readonly authorityId: number;
}

export interface InviteMemberRequest {
  readonly inviteeEmail: string;
}

// ---------------------------------------------------------------------------
// Paginated result types
// ---------------------------------------------------------------------------

export interface SearchResult {
  readonly applications: readonly PlanningApplicationSummary[];
  readonly total: number;
  readonly page: number;
}

export interface NotificationsResult {
  readonly notifications: readonly NotificationItem[];
  readonly total: number;
  readonly page: number;
}

export interface AuthoritiesResult {
  readonly authorities: readonly AuthorityListItem[];
  readonly total: number;
}

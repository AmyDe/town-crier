// ---------------------------------------------------------------------------
// Branded types — prevent mixing up IDs that are structurally identical
// ---------------------------------------------------------------------------

type Brand<T, B extends string> = T & { readonly __brand: B };

export type ApplicationUid = Brand<string, "ApplicationUid">;
export type WatchZoneId = Brand<string, "WatchZoneId">;
export type AuthorityId = Brand<number, "AuthorityId">;

export function asApplicationUid(value: string): ApplicationUid {
  return value as ApplicationUid;
}

export function asWatchZoneId(value: string): WatchZoneId {
  return value as WatchZoneId;
}

export function asAuthorityId(value: number): AuthorityId {
  return value as AuthorityId;
}

// ---------------------------------------------------------------------------
// Union types (no enums — erasableSyntaxOnly is enabled)
// ---------------------------------------------------------------------------

/**
 * PlanIt application state vocabulary. Identifiers are PascalCase and match the
 * exact wire string PlanIt sends in `app_state`.
 *
 * The four states that trigger decision alerts are:
 * Permitted, Conditions, Rejected, Appealed.
 */
export type ApplicationStatus =
  | "Undecided"
  | "Permitted"
  | "Conditions"
  | "Rejected"
  | "Withdrawn"
  | "Appealed"
  | "Unresolved"
  | "Referred"
  | "Not Available";

const APPLICATION_STATUSES: readonly string[] = [
  "Undecided",
  "Permitted",
  "Conditions",
  "Rejected",
  "Withdrawn",
  "Appealed",
  "Unresolved",
  "Referred",
  "Not Available",
];

export function isApplicationStatus(value: unknown): value is ApplicationStatus {
  return typeof value === "string" && APPLICATION_STATUSES.includes(value);
}

export type SubscriptionTier = "Free" | "Personal" | "Pro";

const SUBSCRIPTION_TIERS: readonly string[] = ["Free", "Personal", "Pro"];

export function isSubscriptionTier(value: unknown): value is SubscriptionTier {
  return typeof value === "string" && SUBSCRIPTION_TIERS.includes(value);
}

/**
 * Matches .NET System.DayOfWeek numeric enum.
 * 0 = Sunday, 1 = Monday, ..., 6 = Saturday.
 */
export type DayOfWeek = 0 | 1 | 2 | 3 | 4 | 5 | 6;

export const DAY_OF_WEEK_LABELS: Record<DayOfWeek, string> = {
  0: 'Sunday',
  1: 'Monday',
  2: 'Tuesday',
  3: 'Wednesday',
  4: 'Thursday',
  5: 'Friday',
  6: 'Saturday',
};

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
  readonly pushEnabled: boolean;
  readonly emailDigestEnabled: boolean;
  readonly emailInstantEnabled: boolean;
  readonly digestDay: DayOfWeek;
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
// Request types
// ---------------------------------------------------------------------------

export interface CreateWatchZoneRequest {
  readonly name: string;
  readonly latitude: number;
  readonly longitude: number;
  readonly radiusMetres: number;
  readonly authorityId?: number;
}

export interface UpdateProfileRequest {
  readonly pushEnabled: boolean;
  readonly emailDigestEnabled: boolean;
  readonly emailInstantEnabled: boolean;
  readonly digestDay: DayOfWeek;
}

export interface UpdateWatchZoneRequest {
  readonly name?: string;
  readonly radiusMetres?: number;
}

export interface UpdateZonePreferencesRequest {
  readonly newApplications: boolean;
  readonly statusChanges: boolean;
  readonly decisionUpdates: boolean;
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

// ---------------------------------------------------------------------------
// Legal documents
// ---------------------------------------------------------------------------

export interface LegalDocumentSection {
  readonly heading: string;
  readonly body: string;
}

export interface LegalDocument {
  readonly documentType: string;
  readonly title: string;
  readonly lastUpdated: string;
  readonly sections: readonly LegalDocumentSection[];
}

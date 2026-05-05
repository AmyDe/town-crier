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

/**
 * Wire-format tag distinguishing the lifecycle event a notification was raised
 * for. Mirrors the `.NET` `NotificationEventType` enum which serialises as a
 * string via `UseStringEnumConverter`.
 *
 * - `NewApplication` — a new planning application appeared in PlanIt.
 * - `DecisionUpdate` — a previously-tracked application transitioned to a
 *   decision state (Permitted, Conditions, Rejected, Appealed).
 */
export type NotificationEventType = "NewApplication" | "DecisionUpdate";

const NOTIFICATION_EVENT_TYPES: readonly string[] = [
  "NewApplication",
  "DecisionUpdate",
];

export function isNotificationEventType(
  value: unknown,
): value is NotificationEventType {
  return (
    typeof value === "string" && NOTIFICATION_EVENT_TYPES.includes(value)
  );
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
  readonly digestDay: DayOfWeek;
  /**
   * Profile-level toggle for push notifications when a saved application's
   * decision changes (Permitted, Conditions, Rejected, Appealed). Defaults
   * to true on the API for new and legacy users.
   */
  readonly savedDecisionPush: boolean;
  /**
   * Profile-level toggle for email notifications when a saved application's
   * decision changes. Defaults to true on the API for new and legacy users.
   */
  readonly savedDecisionEmail: boolean;
  readonly tier: SubscriptionTier;
}

export interface ZoneNotificationPreferences {
  readonly zoneId: WatchZoneId;
  readonly newApplicationPush: boolean;
  readonly newApplicationEmail: boolean;
  readonly decisionPush: boolean;
  readonly decisionEmail: boolean;
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
  readonly pushEnabled: boolean;
  readonly emailInstantEnabled: boolean;
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
  /**
   * Populated only by the per-zone applications endpoint — `null` for
   * applications fetched by uid or via paths that do not surface the
   * watermark-aware row data.
   */
  readonly latestUnreadEvent: LatestUnreadEvent | null;
}

/**
 * Per-row unread-notification descriptor surfaced by
 * `GET /v1/me/watch-zones/{zoneId}/applications`. `null` when no notification
 * exists strictly after the user's `lastReadAt` watermark for this row, or the
 * user has no watermark document yet (first-touch path; clients seed via
 * `GET /v1/me/notification-state`).
 *
 * Drives the saturated/muted styling of the application card status pill —
 * see spec `docs/specs/notifications-unread-watermark.md#api-augment-applications`.
 */
export interface LatestUnreadEvent {
  readonly type: NotificationEventType;
  /** Raw PlanIt decision string for `DecisionUpdate` events; `null` otherwise. */
  readonly decision: string | null;
  /** ISO-8601 instant the notification was raised. */
  readonly createdAt: string;
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
  /**
   * Geographic location, when the upstream PlanIt record carried one.
   * Drives the distance sort on the Applications list (tc-ge7j).
   */
  readonly latitude: number | null;
  readonly longitude: number | null;
  /**
   * The latest notification raised for this application that is strictly
   * newer than the user's read-state watermark. `null` for already-read rows
   * and first-touch users (no watermark yet).
   */
  readonly latestUnreadEvent: LatestUnreadEvent | null;
}

/**
 * Snapshot of the caller's notification read-state watermark, returned by
 * `GET /v1/me/notification-state`. A notification is "unread" iff its
 * `createdAt` is strictly after `lastReadAt`. The server returns
 * `totalUnreadCount` precomputed so the client can drive the Unread chip
 * without rescanning the notification list locally. `version` bumps on every
 * successful mark-all-read or advance — useful for detecting out-of-band
 * mutations across devices.
 */
export interface NotificationStateSnapshot {
  /** ISO-8601 instant; notifications strictly newer than this are unread. */
  readonly lastReadAt: string;
  readonly version: number;
  readonly totalUnreadCount: number;
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
  readonly pushEnabled?: boolean;
  readonly emailInstantEnabled?: boolean;
}

export interface UpdateProfileRequest {
  readonly pushEnabled: boolean;
  readonly emailDigestEnabled: boolean;
  readonly digestDay: DayOfWeek;
  readonly savedDecisionPush: boolean;
  readonly savedDecisionEmail: boolean;
}

export interface UpdateWatchZoneRequest {
  readonly name?: string;
  readonly radiusMetres?: number;
  readonly pushEnabled?: boolean;
  readonly emailInstantEnabled?: boolean;
}

export interface UpdateZonePreferencesRequest {
  readonly newApplicationPush: boolean;
  readonly newApplicationEmail: boolean;
  readonly decisionPush: boolean;
  readonly decisionEmail: boolean;
}

// ---------------------------------------------------------------------------
// Paginated result types
// ---------------------------------------------------------------------------

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

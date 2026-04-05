# Town Crier — Platform-Agnostic Feature List

**Date**: 2026-04-05  
**Source**: Derived from React web app and .NET 10 API  
**Purpose**: Authoritative feature reference for cross-platform comparison and new-platform specification

---

## Contents

1. [Authentication & Identity](#1-authentication--identity)
2. [Onboarding](#2-onboarding)
3. [User Profile & Account Management](#3-user-profile--account-management)
4. [Watch Zones](#4-watch-zones)
5. [Planning Applications](#5-planning-applications)
6. [Search](#6-search)
7. [Saved Applications](#7-saved-applications)
8. [Notifications](#8-notifications)
9. [Map Visualisation](#9-map-visualisation)
10. [Subscription Tiers](#10-subscription-tiers)
11. [Push & Email Notification Delivery](#11-push--email-notification-delivery)
12. [Geocoding & Location Services](#12-geocoding--location-services)
13. [Authorities](#13-authorities)
14. [Heritage & Conservation Designations](#14-heritage--conservation-designations)
15. [Data Export & Account Deletion](#15-data-export--account-deletion)
16. [Device Registration](#16-device-registration)
17. [Version & Health](#17-version--health)
18. [Demo Account](#18-demo-account)
19. [Navigation & App Structure](#19-navigation--app-structure)
20. [Appearance & Themes](#20-appearance--themes)
21. [Error & Loading States](#21-error--loading-states)

---

## 1. Authentication & Identity

### 1.1 Sign-in

- User can sign in via Auth0 (OAuth 2.0 / OIDC).
- After sign-in, the client receives a JWT bearer token used for all subsequent API calls.
- Tokens can be silently refreshed; the user does not need to re-authenticate unless the session fully expires.

### 1.2 Sign-out

- User can sign out, which discards the local session.

### 1.3 Post-authentication routing

- After a successful sign-in, the app checks whether the user has a profile.
- If no profile exists, the user is taken to the onboarding flow before anything else.
- If a profile exists, the user is taken to the main dashboard.

### 1.4 Identity endpoint

- API: `GET /api/me` — returns the authenticated user's ID (the JWT `sub` claim).

---

## 2. Onboarding

Onboarding is a first-run flow that creates the user's profile and their first watch zone in one sequence.

### 2.1 Profile creation

- The app creates a user profile automatically on first sign-in (idempotent — calling it twice is safe).
- If the user's email address is verified and belongs to a pre-configured allow-list of domains, the account is automatically granted Pro tier.

### 2.2 First watch zone creation (guided)

- User enters a UK postcode.
- The app geocodes the postcode to coordinates and resolves the local planning authority.
- User selects a monitoring radius (choices: 1 km, 2 km, 5 km, 10 km).
- The app shows a map preview with the zone boundary before the user confirms.
- On confirmation, the first watch zone (named "Home") is created and the user enters the main app.

### 2.3 Onboarding errors

- Geocoding failure (invalid or unresolvable postcode) is surfaced inline before the user can proceed.

---

## 3. User Profile & Account Management

### 3.1 View profile

- User can view their profile, including:
  - User ID
  - Postcode (if set)
  - Subscription tier (Free / Pro)
  - Subscription expiry date (Pro only)

### 3.2 Update profile

- User can update:
  - Home postcode
  - Whether push notifications are globally enabled

### 3.3 Notification preferences (global)

The global notification preferences stored on the profile are:

| Preference | Type | Default | Tier |
|---|---|---|---|
| Push notifications enabled | Boolean | true | Free |
| Digest send day | Day of week | Monday | Free |
| Email digest enabled | Boolean | true | Pro |
| Email instant notifications enabled | Boolean | false | Pro |

### 3.4 Appearance

- User can toggle between light and dark theme.
- The app respects the device/OS system colour scheme preference as the default.

### 3.5 Legal

- User can navigate to the Privacy Policy.
- User can navigate to the Terms of Service.

### 3.6 Data export

- User can request a full export of their data (GDPR compliance).
- Export is delivered as a downloadable JSON file containing profile, watch zones, saved applications, and notification history.

### 3.7 Account deletion

- User can delete their account.
- A confirmation step is required before deletion proceeds.
- Deletion is a hard cascade: profile, watch zones, saved applications, notifications, and device registrations are all removed.
- The user is signed out immediately after deletion.

---

## 4. Watch Zones

A watch zone is a circular geographic area the user wants to monitor for planning activity.

### 4.1 Watch zone data

| Field | Type | Notes |
|---|---|---|
| ID | String (UUID) | System-generated |
| Name | String | User-defined, e.g. "Home", "Office" |
| Centre | Coordinates (lat/lon) | Derived from postcode geocoding |
| Radius | Double (metres) | User selects from 1000/2000/5000/10000 |
| Authority ID | Integer | Planning authority for the area |

### 4.2 List watch zones

- User can see all their watch zones.
- Each zone shows its name and radius.

### 4.3 Create watch zone

Multi-step flow:

1. Enter postcode → geocode to coordinates + resolve authority.
2. Enter a name for the zone and select a monitoring radius.
3. Preview the zone on a map and confirm.

On creation:
- For Pro users, the last 90 days of planning applications for the authority are backfilled immediately.
- The response includes nearby applications already in the system.

### 4.4 Delete watch zone

- User can delete any of their watch zones.
- A confirmation step is required.

### 4.5 Watch zone notification preferences

Each watch zone has independent notification preferences:

| Preference | Default | Tier required |
|---|---|---|
| New applications | Enabled | Free |
| Status changes | Disabled | Pro |
| Decision updates | Disabled | Pro |

- User can toggle each preference per zone independently.
- Attempting to enable status changes or decision updates on a Free account returns a 403 error.

### 4.6 Linked authorities

- User can see the distinct list of planning authorities covered by their watch zones (for use in browsing and search).

---

## 5. Planning Applications

### 5.1 Application data

| Field | Type | Notes |
|---|---|---|
| UID | String | Unique identifier (may contain slashes) |
| Name | String | Reference number from the authority |
| Address | String | Street address of the application site |
| Postcode | String? | Optional |
| Description | String | Description of works proposed |
| Application type | String | e.g. "Full" |
| Application state | Enum | See statuses below |
| Application size | String? | Optional |
| Area name | String | Authority name |
| Authority ID | Integer | |
| Start date | Date? | Date received |
| Consultation date | Date? | |
| Decided date | Date? | |
| Latitude / Longitude | Double? | Optional (not all records have coordinates) |
| Council portal URL | String? | External link to the authority's planning portal |
| Data source link | String? | Direct link on PlanIt |
| Last changed | Timestamp | When the PlanIt record last differed |

### 5.2 Application statuses

- Undecided
- Approved
- Refused
- Withdrawn
- Appealed
- Not Available

### 5.3 Browse applications by authority

- User can see the planning authorities linked to their watch zones.
- User can select an authority and browse all ingested applications for it.
- Each application in the list shows: name, address, description snippet, type, status, area name, and start date.

### 5.4 View application detail

- User can view the full detail of a single application.
- Detail includes all fields listed in 5.1.
- If the application has coordinates, heritage/conservation designations are fetched and shown (see §14).
- User can open the council portal in an external browser if a URL is available.

### 5.5 Save / unsave an application

- From the detail view, user can save an application to their saved list.
- User can unsave it from the detail view or from the saved applications list.
- The save state is shown with immediate (optimistic) UI feedback.

### 5.6 Data source

- Applications are ingested from PlanIt (planit.org.uk) via a background polling job.
- The job polls each active authority since the last successful poll time (default lookback: 30 days on first run).
- Applications are stored in Cosmos DB and served from there; the client never calls PlanIt directly.

---

## 6. Search

### 6.1 Free-text search

- User can search planning applications by keyword within a specific planning authority.
- Results are paginated (20 per page).
- Each result shows the same summary fields as the browse list.

### 6.2 Authority selection for search

- User must select an authority before searching.
- Authority selection is via an autocomplete/typeahead (minimum 2 characters, debounced).
- Any UK authority can be searched — not limited to the user's watch zones.

### 6.3 Tier restriction

- Search is a **Pro-only** feature.
- A Free-tier user who attempts to search receives a 403 response.
- The app surfaces an upgrade prompt in this case.

---

## 7. Saved Applications

### 7.1 List saved applications

- User can see all applications they have saved.
- Each item shows: name, address, description, type, status, area name, start date.

### 7.2 Remove saved application

- User can remove an application from their saved list.
- A confirmation step is required.

---

## 8. Notifications

### 8.1 What triggers a notification

A notification is created when:

- A new planning application is found by the polling job within the boundary of a user's watch zone AND the zone has "New applications" enabled.
- *(Pro)* An application's status changes, and the zone has "Status changes" enabled.
- *(Pro)* A planning decision is issued, and the zone has "Decision updates" enabled.

### 8.2 Notification data

| Field | Notes |
|---|---|
| Application name | Reference number |
| Application address | |
| Application description | |
| Application type | |
| Authority ID | |
| Watch zone ID | The zone that matched |
| Push sent flag | Whether a push notification was delivered |
| Created at | Timestamp |

### 8.3 Notification list

- User can see a paginated list of their notifications (20 per page).
- Each item shows: application name, type, address, description, and timestamp.
- Pagination shows current page and total pages.

### 8.4 Notification suppression rules

The system applies these rules before delivering a push notification (in order):

1. **Duplicate suppression**: If the same user already has a notification for the same application name, skip delivery (but still record it).
2. **Global push disabled**: If `PushEnabled = false` on the user profile, record only.
3. **Zone preference disabled**: If the relevant zone preference (NewApplications / StatusChanges / DecisionUpdates) is off, record only.
4. **Free-tier monthly cap**: Free users are capped at 5 push notifications per calendar month. Further notifications are recorded but not delivered.
5. **No registered device**: If the user has no registered device tokens, record only.

### 8.5 Weekly digest

- Pro users receive a weekly digest notification summarising activity across all their watch zones.
- Digest day is configurable per user (default: Monday).
- Digest is sent via push (if push enabled) and/or email (if email digest enabled).
- The digest groups notifications by watch zone.

---

## 9. Map Visualisation

### 9.1 Map view

- The app provides a map view showing planning applications overlaid on a geographic map.
- Map uses OpenStreetMap tiles.
- The map centres on the user's first watch zone by default.

### 9.2 Application markers

- Each application with coordinates is shown as a marker.
- Tapping/clicking a marker shows a popup with: description, address, and a link to the full detail view.

### 9.3 Watch zone boundaries

- Watch zones can be visualised on the map as circles centred on the zone's coordinates.

---

## 10. Subscription Tiers

### 10.1 Tiers

| Tier | Description |
|---|---|
| Free | Default for new users |
| Pro | Paid; unlocks additional features |
| Personal | Defined but not actively used |

### 10.2 Feature matrix

| Feature | Free | Pro |
|---|---|---|
| Watch zones | Limited | Unlimited |
| New application notifications | Yes (capped at 5/month) | Yes (unlimited) |
| Status change notifications | No | Yes |
| Decision update notifications | No | Yes |
| Weekly digest (push) | No | Yes |
| Weekly digest (email) | No | Yes |
| Instant email notifications | No | Yes |
| Full-text search | No | Yes |
| 90-day backfill on zone creation | No | Yes |
| Data export | Yes | Yes |
| Account deletion | Yes | Yes |

### 10.3 Subscription activation

- Subscriptions are activated via Apple in-app purchases (App Store).
- Apple sends a signed server notification; the API validates the signature and updates the user's tier.
- Events handled: Subscribed, Renewed, Expired, Refund, Grace Period.
- An admin endpoint allows manual tier grant/revoke (for internal use).

### 10.4 Auto-grant

- Users whose verified email address belongs to a configured domain allowlist are automatically granted Pro tier on registration (expiry: 2099-12-31).

### 10.5 Grace period

- After expiry, users may enter a grace period before being downgraded to Free.

---

## 11. Push & Email Notification Delivery

### 11.1 Push notifications

- Push notifications are delivered via Firebase Cloud Messaging (FCM).
- Platform-specific tokens (iOS, Android) are registered against the user's account.
- Multiple devices can be registered per user.
- Invalid tokens are removed when FCM reports them as invalid.

### 11.2 Email notifications — instant

- Pro users with instant email enabled receive an email for each matched application.
- Requires a verified email address on the account.

### 11.3 Email notifications — weekly digest

- Pro users with email digest enabled receive a weekly summary email.
- Grouped by watch zone with all matching notifications from the past 7 days.
- Delivered on the user's configured digest day.

### 11.4 Notification infrastructure

- Email is delivered via Azure Communication Services.
- Push is delivered via Firebase Cloud Messaging.
- No-op implementations are available for development/test environments.

---

## 12. Geocoding & Location Services

### 12.1 Postcode geocoding

- Any postcode can be geocoded to coordinates (latitude/longitude).
- The geocoder also resolves the associated planning authority.
- Endpoint is public (no authentication required).

### 12.2 Authority resolution from coordinates

- When a watch zone is created without an explicit authority ID, the API resolves the authority automatically from the zone's coordinates.
- Resolution is via PostcodesIo.

### 12.3 Error handling

- Invalid postcode format → 400 Bad Request.
- Valid format but unresolvable → 404 Not Found.

---

## 13. Authorities

### 13.1 List authorities

- Any client (authenticated or not) can retrieve the full list of UK planning authorities.
- Supports optional text search/filter by authority name.

### 13.2 Get authority by ID

- Any client can retrieve a single authority's details by its numeric ID.
- Returns: name, area type, council URL, planning portal URL.

### 13.3 Authority data source

- Authorities are sourced from PlanIt and cached in memory with a TTL.

---

## 14. Heritage & Conservation Designations

### 14.1 Designation lookup

- For any coordinates, the API returns the heritage designations that apply.
- Requires authentication.

### 14.2 Designation types

| Designation | Data returned |
|---|---|
| Conservation area | Boolean + area name |
| Listed building curtilage | Boolean + listing grade |
| Article 4 direction | Boolean |

### 14.3 Data source

- Gov.uk Planning Data API.
- On failure, the API returns "none" (all false) rather than surfacing an error to the user.

### 14.4 Application detail integration

- When viewing a planning application with coordinates, designations are fetched and shown alongside the application data.
- The designations section is only shown if at least one designation applies.

---

## 15. Data Export & Account Deletion

### 15.1 Data export (GDPR)

- Authenticated user can request a full export of all their data.
- Response is a JSON document containing:
  - User profile (ID, email, postcode)
  - All watch zones
  - All saved applications
  - All notification history

### 15.2 Account deletion (GDPR right to erasure)

- Authenticated user can delete their account.
- All associated data is deleted in a cascade (profile, zones, notifications, saved apps, device registrations).
- Returns 204 on success.

---

## 16. Device Registration

### 16.1 Register device

- Authenticated user can register a device for push notifications.
- Requires: push token (string) and platform (iOS or Android).
- If the same token is registered again, the registration timestamp is refreshed.

### 16.2 Unregister device

- Authenticated user (or the system) can remove a device token.
- Used when the user signs out on a device, or when FCM reports an invalid token.

---

## 17. Version & Health

### 17.1 Health check

- Public endpoint: returns a status string.
- Used as a liveness probe and by the web app to warm up cold-started backend instances.

### 17.2 Version config

- Public endpoint: returns the current API version and the minimum client version the API supports.
- Clients can use this to prompt the user to upgrade if their app version is below the minimum.

---

## 18. Demo Account

### 18.1 Demo mode

- The API provides a public endpoint that returns a pre-configured demo account.
- Includes a demo watch zone and a set of sample planning applications.
- Intended for marketing/onboarding use — does not require sign-in.
- The demo account and its data are created on first call and reused thereafter.

---

## 19. Navigation & App Structure

### 19.1 Main navigation areas

| Destination | Description |
|---|---|
| Dashboard | Overview: watch zones summary, recent applications, quick links |
| Applications | Browse applications by authority |
| Watch Zones | List, create, edit, and delete zones |
| Map | Geographic map of applications |
| Search | Free-text application search (Pro) |
| Saved | Saved applications list |
| Notifications | Notification history |
| Settings | Profile, preferences, account, legal |

### 19.2 Landing / marketing surface

- A public landing page (no authentication required) with:
  - Hero section describing the product
  - App Store download link
  - "Try the web app" CTA
  - Stats about service coverage
  - How it works explanation
  - Pricing section
  - FAQ section
  - Legal links

### 19.3 Legal pages

- Privacy Policy and Terms of Service are accessible without signing in.

### 19.4 Auth callback

- A dedicated route/screen handles the OAuth callback from Auth0 after sign-in.

---

## 20. Appearance & Themes

### 20.1 Colour schemes

- Light theme
- Dark theme
- (Design system also specifies an OLED dark variant)

### 20.2 Theme selection

- User can manually override the theme in settings.
- Default is the device/OS system preference.
- Selection persists locally on the device.

---

## 21. Error & Loading States

### 21.1 Loading states

Every asynchronous operation must show a loading indicator. Common patterns:

- Full-screen loading (auth, initial profile load)
- Section-level loading (list data)
- Button-level loading (form submissions, save/delete actions)
- Skeleton placeholders for list content

### 21.2 Empty states

Every list view has a defined empty state:

| Screen | Empty state behaviour |
|---|---|
| Watch Zones | Prompt to create first zone |
| Applications | "No applications" message |
| Saved Applications | "No saved applications" message |
| Notifications | Explanation of when notifications will appear |
| Dashboard | Prompt to create a watch zone |

### 21.3 Error states

- Inline validation errors on form fields (e.g. invalid postcode).
- Section-level errors with a retry action where appropriate.
- Global error boundary prevents a rendering error from crashing the entire app.

### 21.4 Confirmation dialogs

Destructive or irreversible actions require explicit user confirmation before proceeding:

- Delete watch zone
- Remove saved application
- Delete account

### 21.5 Pro feature gate

- Attempting to use a Pro feature as a Free user surfaces an upgrade prompt rather than a generic error.

---

## API Summary: Complete Endpoint Reference

| Domain | Method | Path | Auth | Tier |
|---|---|---|---|---|
| Identity | GET | /api/me | Required | Any |
| Profile | POST | /v1/me | Required | Any |
| Profile | GET | /v1/me | Required | Any |
| Profile | PATCH | /v1/me | Required | Any |
| Profile | GET | /v1/me/data | Required | Any |
| Profile | DELETE | /v1/me | Required | Any |
| Watch Zones | POST | /v1/me/watch-zones | Required | Any |
| Watch Zones | GET | /v1/me/watch-zones | Required | Any |
| Watch Zones | DELETE | /v1/me/watch-zones/{zoneId} | Required | Any |
| Zone Preferences | GET | /v1/me/watch-zones/{zoneId}/preferences | Required | Any |
| Zone Preferences | PUT | /v1/me/watch-zones/{zoneId}/preferences | Required | Any (Pro for status/decision) |
| Applications | GET | /v1/applications?authorityId={id} | Required | Any |
| Applications | GET | /v1/applications/{**uid} | Required | Any |
| Authorities (user) | GET | /v1/me/application-authorities | Required | Any |
| Search | GET | /v1/search?q={}&authorityId={}&page={} | Required | Pro |
| Saved | PUT | /v1/me/saved-applications/{**uid} | Required | Any |
| Saved | DELETE | /v1/me/saved-applications/{**uid} | Required | Any |
| Saved | GET | /v1/me/saved-applications | Required | Any |
| Notifications | GET | /v1/notifications?page={}&pageSize={} | Required | Any |
| Device Token | PUT | /v1/me/device-token | Required | Any |
| Device Token | DELETE | /v1/me/device-token/{token} | Required | Any |
| Geocoding | GET | /v1/geocode/{postcode} | Anonymous | — |
| Authorities | GET | /v1/authorities?search={} | Anonymous | — |
| Authorities | GET | /v1/authorities/{id} | Anonymous | — |
| Designations | GET | /v1/designations?latitude={}&longitude={} | Required | Any |
| Demo | GET | /v1/demo-account | Anonymous | — |
| Version | GET | /v1/version-config | Anonymous | — |
| Health | GET | /health or /v1/health | Anonymous | — |
| Admin: Subscriptions | PUT | /v1/admin/subscriptions | API Key | — |
| App Store Webhook | POST | (App Store server notification webhook) | Signed JWS | — |

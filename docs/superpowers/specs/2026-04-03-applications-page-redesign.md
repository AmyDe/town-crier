# Applications Page Redesign

Date: 2026-04-03

## Problem

The Applications page lets users search all ~500 PlanIt authorities, but only authorities with active watch zones have data in Cosmos DB. Selecting an authority with no watch zones returns an empty list with no explanation. This is confusing and misleading.

## Decision

Replace the authority search box with a list of the current user's watch zone authorities. This scopes the page to authorities that are guaranteed to have data, and creates a meaningful differentiation between personal and pro tiers.

## Design

### Backend

**New endpoint**: `GET /v1/applications/authorities` (authenticated)

- Reads the authenticated user's watch zones to get their distinct authority IDs
- Cross-references with `IAuthorityProvider` to resolve names and area types
- Returns `{ authorities: AuthorityListItem[], count: int }`
- `AuthorityListItem` reuses the existing record: `{ id, name, areaType }`

**New query**: `GetUserApplicationAuthoritiesQuery` / `GetUserApplicationAuthoritiesQueryHandler`
- Dependencies: `IWatchZoneRepository` (to get user's zones), `IAuthorityProvider` (to resolve details)
- Filters watch zones by the authenticated user's ID
- Maps distinct authority IDs to `AuthorityListItem` via the cached authority provider
- Returns sorted by name

### Frontend

**Applications page** has two view states managed in the `useApplications` hook:

1. **Authority list** (default)
   - Fetches the user's active authorities on mount via new `GET /v1/applications/authorities`
   - Renders a clickable card per authority showing name and area type
   - Empty state: "Set up a watch zone to start browsing applications" with link to `/watch-zones/new`

2. **Application list** (after selecting an authority)
   - Breadcrumb: `Authorities > {authority name}` at top of page; clicking "Authorities" returns to state 1
   - Fetches and displays applications for the selected authority (existing behaviour)
   - Existing `ApplicationCard` component unchanged

**Removed from this page**: `AuthoritySelector` component and `AuthoritySearchPort` dependency. The `AuthoritySelector` remains available for other pages (e.g., Search) that still need free-text authority search.

### New domain port

```typescript
interface UserAuthoritiesPort {
  fetchMyAuthorities(): Promise<readonly AuthorityListItem[]>;
}
```

Replaces `AuthoritySearchPort` in the Applications feature.

### What stays the same

- `ApplicationCard` component and its link to `/applications/{uid}`
- Application detail page and route (`/applications/*`)
- `GET /v1/applications?authorityId=` endpoint for fetching applications by authority
- The `AuthoritySelector` component (still used elsewhere)

## Consequences

- Users can only browse applications in authorities where they have watch zones
- No PlanIt calls on the Applications page — all data comes from Cosmos and the cached authority list
- The page becomes a meaningful entry point for personal-tier users to explore their watched areas
- Pro-tier differentiation: broader authority access can be gated by subscription tier in future

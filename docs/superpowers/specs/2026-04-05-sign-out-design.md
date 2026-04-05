# Sign Out Feature — Design Spec

Date: 2026-04-05

## Overview

Add a "Sign out" button to the Settings page that ends the user's Auth0 session and redirects to the landing page with a confirmation toast.

## Decisions

### Placement: Settings page only

The sign-out action lives in a new "Session" section on the Settings page, placed above the danger zone (Delete Account). No sidebar, header, or avatar-menu placement.

**Why:** Settings already contains account-level actions (theme, profile, account deletion). Sign out fits naturally here without adding new UI chrome.

### Button style: secondary/outlined

A secondary/outlined button visually distinguishes sign out from the destructive "Delete Account" button while remaining clearly actionable.

### Post-logout flow: landing page with toast

After sign out, Auth0 redirects to `/?signed_out=true`. The landing page detects the query parameter and displays a brief auto-dismissing toast: "You've been signed out."

### No explicit local storage clearing

Auth0's `logout()` clears its own tokens from localStorage (used because `cacheLocation="localstorage"` is configured in App.tsx). React Query's in-memory cache is lost on the page redirect. The only app-specific localStorage key is `tc-theme` (light/dark preference), which is a device-level preference and should persist across sign-out.

## Architecture

### 1. AuthPort interface expansion

**File:** `web/src/domain/ports/auth-port.ts`

Add `logout(): Promise<void>` to the `AuthPort` interface. This keeps the domain layer decoupled from Auth0.

### 2. Auth0AuthAdapter implementation

**File:** `web/src/auth/Auth0AuthAdapter.tsx`

Implement `logout()` by calling Auth0's `logout()` with:

```typescript
logout({
  logoutParams: {
    returnTo: `${window.location.origin}?signed_out=true`,
  },
});
```

This ends the Auth0 session server-side and redirects to the landing page with the toast trigger parameter.

### 3. Settings page — Session section

**File:** `web/src/features/Settings/SettingsPage.tsx`

Add a "Session" section above the existing danger zone section containing a single secondary/outlined button labelled "Sign out". On click, it calls `logout()` from the auth context.

No confirmation dialog — sign out is not destructive. The user can immediately sign back in from the landing page.

### 4. Landing page — signed-out toast

**File:** `web/src/features/LandingPage/LandingPage.tsx`

On mount, check for `?signed_out=true` in the URL search params. If present:

1. Display a toast notification: "You've been signed out"
2. Remove the query parameter from the URL (using `history.replaceState`) to prevent the toast from reappearing on refresh
3. Auto-dismiss the toast after 4 seconds

The toast component should be a simple styled div positioned at the top of the page — no need for a toast library.

## Out of Scope

- User avatar or profile indicator in the sidebar
- Sign out from sidebar, header, or other locations
- "Sign out of all devices" functionality
- Confirmation dialog before sign out
- Clearing device-level preferences (theme) on sign out

## Testing Strategy

- **AuthPort/adapter:** Unit test that `logout()` calls Auth0's logout with the correct `returnTo` URL
- **Settings page:** Test that the Session section renders with a "Sign out" button and that clicking it invokes `logout()`
- **Landing page toast:** Test that the toast appears when `?signed_out=true` is in the URL, and that the parameter is removed from the URL after rendering

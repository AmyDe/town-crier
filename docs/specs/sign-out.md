# Sign Out

## Context

The web app has no way for users to manually sign out. The Auth0 `logout()` function is only called internally when an account is deleted. Users need a visible, intentional way to end their session.

## Design

Add `logout()` to the `AuthPort` domain interface so sign-out flows through the same port abstraction as login. `Auth0AuthAdapter` implements it by calling Auth0's `logout()` with `returnTo` set to the landing page with a `?signed_out=true` query parameter.

The Settings page gets a new "Session" section (above the danger zone) with a secondary/outlined button. No confirmation dialog — sign out is not destructive.

The landing page detects the query parameter, shows an auto-dismissing toast ("You've been signed out"), and strips the parameter from the URL via `history.replaceState`.

Auth0 handles all token cleanup (it uses `cacheLocation="localstorage"`). The only app-specific localStorage key (`tc-theme`) is a device preference and persists across sign-out. No manual clearing needed.

See also: `docs/superpowers/specs/2026-04-05-sign-out-design.md` (full brainstorm output), `docs/superpowers/plans/2026-04-05-sign-out.md` (implementation plan with code).

## Scope

**In:** AuthPort interface, Auth0AuthAdapter, Settings page Session section, landing page toast.

**Out:** Sidebar/header sign-out, user avatar, "sign out everywhere", confirmation dialog.

## Steps

### AuthPort + adapter
Add `logout(): Promise<void>` to `AuthPort`. Implement in `Auth0AuthAdapter` with `returnTo` pointing to `/?signed_out=true`. Update `SpyAuthPort` test double.

### Settings page sign-out button
New "Session" section with secondary button. Replace direct `useAuth0()` import with `useAuth()` from context. Migrate existing tests from Auth0 mock to `AuthProvider` + `SpyAuthPort`.

### Toast component
Create `Toast` component — renders a message, auto-dismisses after configurable duration (default 4s), uses `role="status"` for accessibility.

### Landing page toast
Detect `?signed_out=true`, show toast, strip query param. No toast without param.

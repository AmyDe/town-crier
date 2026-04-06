# Email Notification Settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire email notification preferences (digest, instant, digest day) from the API to the web settings page so users can configure them.

**Architecture:** Extend GET /v1/me to return email fields already stored in the domain. On the frontend, add `updateProfile` to the SettingsRepository port, build a reusable Toggle component, extend the useUserProfile hook with optimistic update support, and render a Notifications section in SettingsPage.

**Tech Stack:** .NET 10 (TUnit), React 18, TypeScript, CSS Modules, Vitest + Testing Library

**Spec:** `docs/superpowers/specs/2026-04-06-email-notification-settings-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `api/src/town-crier.application/UserProfiles/GetUserProfileResult.cs` | Add email fields to GET result |
| Modify | `api/src/town-crier.application/UserProfiles/GetUserProfileQueryHandler.cs` | Map email fields from domain |
| Modify | `api/tests/town-crier.application.tests/UserProfiles/GetUserProfileQueryHandlerTests.cs` | Assert email fields returned |
| Modify | `web/src/domain/types.ts` | Add DayOfWeek, update UserProfile & UpdateProfileRequest |
| Modify | `web/src/domain/ports/settings-repository.ts` | Add updateProfile method |
| Modify | `web/src/features/Settings/ApiSettingsRepository.ts` | Implement updateProfile |
| Modify | `web/src/features/Settings/__tests__/spies/spy-settings-repository.ts` | Add updateProfile spy |
| Modify | `web/src/features/Settings/__tests__/fixtures/user-profile.fixtures.ts` | Add email field defaults |
| Create | `web/src/components/Toggle/Toggle.tsx` | Accessible switch component |
| Create | `web/src/components/Toggle/Toggle.module.css` | Toggle styling |
| Modify | `web/src/features/Settings/useUserProfile.ts` | Add optimistic updatePreferences |
| Modify | `web/src/features/Settings/__tests__/useUserProfile.test.ts` | Test updatePreferences |
| Modify | `web/src/features/Settings/SettingsPage.tsx` | Notifications section UI |
| Modify | `web/src/features/Settings/SettingsPage.module.css` | Styles for toggle rows and select |
| Modify | `web/src/features/Settings/__tests__/SettingsPage.test.tsx` | Test notification controls |

---

### Task 1: API — Expose email fields on GET /v1/me

**Files:**
- Modify: `api/src/town-crier.application/UserProfiles/GetUserProfileResult.cs`
- Modify: `api/src/town-crier.application/UserProfiles/GetUserProfileQueryHandler.cs`
- Modify: `api/tests/town-crier.application.tests/UserProfiles/GetUserProfileQueryHandlerTests.cs`

- [ ] **Step 1: Write failing test for email fields on GET result**

In `api/tests/town-crier.application.tests/UserProfiles/GetUserProfileQueryHandlerTests.cs`, add:

```csharp
[Test]
public async Task Should_ReturnDefaultEmailPreferences_When_ProfileExists()
{
    // Arrange
    var repository = new FakeUserProfileRepository();
    var profile = UserProfile.Register("auth0|email-user");
    await repository.SaveAsync(profile, CancellationToken.None);

    var handler = new GetUserProfileQueryHandler(repository);
    var query = new GetUserProfileQuery("auth0|email-user");

    // Act
    var result = await handler.HandleAsync(query, CancellationToken.None);

    // Assert
    await Assert.That(result).IsNotNull();
    await Assert.That(result!.EmailDigestEnabled).IsTrue();
    await Assert.That(result.EmailInstantEnabled).IsFalse();
    await Assert.That(result.DigestDay).IsEqualTo(DayOfWeek.Monday);
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && dotnet test --filter "Should_ReturnDefaultEmailPreferences"`
Expected: Build failure — `GetUserProfileResult` does not contain `EmailDigestEnabled`, `EmailInstantEnabled`, `DigestDay`.

- [ ] **Step 3: Update GetUserProfileResult to include email fields**

Replace the full record in `api/src/town-crier.application/UserProfiles/GetUserProfileResult.cs`:

```csharp
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record GetUserProfileResult(
    string UserId,
    bool PushEnabled,
    DayOfWeek DigestDay,
    bool EmailDigestEnabled,
    bool EmailInstantEnabled,
    SubscriptionTier Tier);
```

- [ ] **Step 4: Update GetUserProfileQueryHandler to map the new fields**

In `api/src/town-crier.application/UserProfiles/GetUserProfileQueryHandler.cs`, replace the return statement (lines 22-25):

```csharp
return new GetUserProfileResult(
    profile.UserId,
    profile.NotificationPreferences.PushEnabled,
    profile.NotificationPreferences.DigestDay,
    profile.NotificationPreferences.EmailDigestEnabled,
    profile.NotificationPreferences.EmailInstantEnabled,
    profile.Tier);
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd api && dotnet test --filter "GetUserProfileQueryHandlerTests"`
Expected: All 3 tests PASS (the existing `Should_ReturnProfile_When_ProfileExists` test should also still pass since it doesn't assert against the new fields).

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.application/UserProfiles/GetUserProfileResult.cs api/src/town-crier.application/UserProfiles/GetUserProfileQueryHandler.cs api/tests/town-crier.application.tests/UserProfiles/GetUserProfileQueryHandlerTests.cs
git commit -m "feat(api): expose email notification fields on GET /v1/me"
```

---

### Task 2: Web — Update domain types and test infrastructure

**Files:**
- Modify: `web/src/domain/types.ts`
- Modify: `web/src/features/Settings/__tests__/fixtures/user-profile.fixtures.ts`
- Modify: `web/src/features/Settings/__tests__/spies/spy-settings-repository.ts`
- Modify: `web/src/domain/ports/settings-repository.ts`
- Modify: `web/src/features/Settings/ApiSettingsRepository.ts`

- [ ] **Step 1: Add DayOfWeek type and update UserProfile and UpdateProfileRequest**

In `web/src/domain/types.ts`, add the `DayOfWeek` type after the `SubscriptionTier` block (after line 54), and update `UserProfile` and `UpdateProfileRequest`:

Add after the `isSubscriptionTier` function (after line 54):

```typescript
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
```

Replace the `UserProfile` interface (lines 69-73):

```typescript
export interface UserProfile {
  readonly userId: string;
  readonly pushEnabled: boolean;
  readonly emailDigestEnabled: boolean;
  readonly emailInstantEnabled: boolean;
  readonly digestDay: DayOfWeek;
  readonly tier: SubscriptionTier;
}
```

Replace the `UpdateProfileRequest` interface (lines 207-209):

```typescript
export interface UpdateProfileRequest {
  readonly pushEnabled: boolean;
  readonly emailDigestEnabled: boolean;
  readonly emailInstantEnabled: boolean;
  readonly digestDay: DayOfWeek;
}
```

- [ ] **Step 2: Update test fixtures with default email field values**

Replace the full file `web/src/features/Settings/__tests__/fixtures/user-profile.fixtures.ts`:

```typescript
import type { UserProfile, SubscriptionTier } from '../../../../domain/types';

export function freeUserProfile(
  overrides?: Partial<UserProfile>,
): UserProfile {
  return {
    userId: 'auth0|abc123',
    pushEnabled: true,
    emailDigestEnabled: true,
    emailInstantEnabled: false,
    digestDay: 1,
    tier: 'Free' as SubscriptionTier,
    ...overrides,
  };
}

export function proUserProfile(
  overrides?: Partial<UserProfile>,
): UserProfile {
  return {
    ...freeUserProfile(),
    userId: 'auth0|pro456',
    tier: 'Pro' as SubscriptionTier,
    ...overrides,
  };
}
```

- [ ] **Step 3: Add updateProfile to SettingsRepository port**

Replace the full file `web/src/domain/ports/settings-repository.ts`:

```typescript
import type { UpdateProfileRequest, UserProfile } from '../types';

export interface SettingsRepository {
  fetchProfile(): Promise<UserProfile>;
  updateProfile(request: UpdateProfileRequest): Promise<UserProfile>;
  exportData(): Promise<Blob>;
  deleteAccount(): Promise<void>;
}
```

- [ ] **Step 4: Implement updateProfile in ApiSettingsRepository**

In `web/src/features/Settings/ApiSettingsRepository.ts`, add the method after `fetchProfile()` (after line 22):

```typescript
  async updateProfile(request: UpdateProfileRequest): Promise<UserProfile> {
    return this.api.update(request);
  }
```

Also add the import at the top — replace line 3:

```typescript
import type { UpdateProfileRequest, UserProfile } from '../../domain/types';
```

- [ ] **Step 5: Add updateProfile to SpySettingsRepository**

In `web/src/features/Settings/__tests__/spies/spy-settings-repository.ts`, add the spy method and update the import:

Replace line 1:

```typescript
import type { SettingsRepository } from '../../../../domain/ports/settings-repository';
import type { UpdateProfileRequest, UserProfile } from '../../../../domain/types';
```

Add after the `fetchProfile` block (after line 14):

```typescript
  updateProfileCalls = 0;
  updateProfileLastRequest: UpdateProfileRequest | null = null;
  updateProfileResult: UserProfile | null = null;
  updateProfileError: Error | null = null;

  async updateProfile(request: UpdateProfileRequest): Promise<UserProfile> {
    this.updateProfileCalls++;
    this.updateProfileLastRequest = request;
    if (this.updateProfileError) {
      throw this.updateProfileError;
    }
    return this.updateProfileResult ?? this.fetchProfileResult;
  }
```

- [ ] **Step 6: Run type check to verify no compile errors**

Run: `cd web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 7: Commit**

```bash
git add web/src/domain/types.ts web/src/domain/ports/settings-repository.ts web/src/features/Settings/ApiSettingsRepository.ts web/src/features/Settings/__tests__/spies/spy-settings-repository.ts web/src/features/Settings/__tests__/fixtures/user-profile.fixtures.ts
git commit -m "feat(web): add email notification types and repository support"
```

---

### Task 3: Web — Create Toggle component

**Files:**
- Create: `web/src/components/Toggle/Toggle.tsx`
- Create: `web/src/components/Toggle/Toggle.module.css`

- [ ] **Step 1: Create Toggle component**

Create `web/src/components/Toggle/Toggle.tsx`:

```tsx
import styles from './Toggle.module.css';

interface Props {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
}

export function Toggle({ checked, onChange, label }: Props) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      className={styles.toggle}
      data-checked={checked || undefined}
      onClick={() => onChange(!checked)}
    >
      <span className={styles.thumb} />
    </button>
  );
}
```

- [ ] **Step 2: Create Toggle styles**

Create `web/src/components/Toggle/Toggle.module.css`:

```css
.toggle {
  position: relative;
  display: inline-flex;
  align-items: center;
  width: 44px;
  height: 24px;
  padding: 2px;
  border: 1px solid var(--tc-border);
  border-radius: 12px;
  background: var(--tc-surface);
  cursor: pointer;
  transition: background-color var(--tc-duration-fast) ease,
              border-color var(--tc-duration-fast) ease;
}

.toggle[data-checked] {
  background: var(--tc-amber);
  border-color: var(--tc-amber);
}

.thumb {
  display: block;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: var(--tc-text-secondary);
  transition: transform var(--tc-duration-fast) ease,
              background-color var(--tc-duration-fast) ease;
}

.toggle[data-checked] .thumb {
  transform: translateX(20px);
  background: var(--tc-text-on-accent);
}
```

- [ ] **Step 3: Verify types compile**

Run: `cd web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/Toggle/Toggle.tsx web/src/components/Toggle/Toggle.module.css
git commit -m "feat(web): add accessible Toggle switch component"
```

---

### Task 4: Web — Extend useUserProfile hook with updatePreferences

**Files:**
- Modify: `web/src/features/Settings/useUserProfile.ts`
- Modify: `web/src/features/Settings/__tests__/useUserProfile.test.ts`

- [ ] **Step 1: Write failing tests for updatePreferences**

Add the following tests to the end of the `describe` block in `web/src/features/Settings/__tests__/useUserProfile.test.ts`:

```typescript
  it('updates preferences and calls repository', async () => {
    spy.fetchProfileResult = freeUserProfile();

    const { result } = renderHook(() => useUserProfile(spy, logout));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.updatePreferences({ emailDigestEnabled: false });
    });

    expect(spy.updateProfileCalls).toBe(1);
    expect(spy.updateProfileLastRequest).toEqual({
      pushEnabled: true,
      emailDigestEnabled: false,
      emailInstantEnabled: false,
      digestDay: 1,
    });
  });

  it('optimistically updates profile on preference change', async () => {
    spy.fetchProfileResult = freeUserProfile();

    const { result } = renderHook(() => useUserProfile(spy, logout));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.updatePreferences({ emailInstantEnabled: true });
    });

    expect(result.current.profile?.emailInstantEnabled).toBe(true);
  });

  it('reverts profile on preference update failure', async () => {
    spy.fetchProfileResult = freeUserProfile({ emailDigestEnabled: true });
    spy.updateProfileError = new Error('Network error');

    const { result } = renderHook(() => useUserProfile(spy, logout));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.updatePreferences({ emailDigestEnabled: false });
    });

    expect(result.current.profile?.emailDigestEnabled).toBe(true);
    expect(result.current.error).toBe('Network error');
  });
```

Also add the `freeUserProfile` import — update the import on line 5:

```typescript
import { freeUserProfile, proUserProfile } from './fixtures/user-profile.fixtures';
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && npx vitest run src/features/Settings/__tests__/useUserProfile.test.ts`
Expected: FAIL — `updatePreferences` does not exist on the hook return value.

- [ ] **Step 3: Implement updatePreferences in useUserProfile**

Replace the full file `web/src/features/Settings/useUserProfile.ts`:

```typescript
import { useState, useCallback, useEffect } from 'react';
import type { SettingsRepository } from '../../domain/ports/settings-repository';
import type { UpdateProfileRequest, UserProfile } from '../../domain/types';
import { useFetchData } from '../../hooks/useFetchData';

export function useUserProfile(
  repository: SettingsRepository,
  logout: () => void,
) {
  const [isExporting, setIsExporting] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [localProfile, setLocalProfile] = useState<UserProfile | null>(null);

  const { data: fetchedProfile, isLoading, error: fetchError } = useFetchData<UserProfile>(
    () => repository.fetchProfile(),
    [repository],
  );

  useEffect(() => {
    if (fetchedProfile) {
      setLocalProfile(fetchedProfile);
    }
  }, [fetchedProfile]);

  const profile = localProfile;
  const error = actionError ?? fetchError;

  const updatePreferences = useCallback(async (changes: Partial<UpdateProfileRequest>) => {
    if (!localProfile) return;

    const request: UpdateProfileRequest = {
      pushEnabled: localProfile.pushEnabled,
      emailDigestEnabled: localProfile.emailDigestEnabled,
      emailInstantEnabled: localProfile.emailInstantEnabled,
      digestDay: localProfile.digestDay,
      ...changes,
    };

    const previous = localProfile;
    setLocalProfile({ ...localProfile, ...changes });
    setActionError(null);

    try {
      const updated = await repository.updateProfile(request);
      setLocalProfile(updated);
    } catch (err) {
      setLocalProfile(previous);
      const message = err instanceof Error ? err.message : 'Failed to update preferences';
      setActionError(message);
    }
  }, [localProfile, repository]);

  const exportData = useCallback(async () => {
    setIsExporting(true);
    setActionError(null);
    try {
      const blob = await repository.exportData();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = 'town-crier-data.json';
      link.click();
      URL.revokeObjectURL(url);
      setIsExporting(false);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to export data';
      setIsExporting(false);
      setActionError(message);
    }
  }, [repository]);

  const deleteAccount = useCallback(async () => {
    setIsDeleting(true);
    setActionError(null);
    try {
      await repository.deleteAccount();
      setIsDeleting(false);
      logout();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to delete account';
      setIsDeleting(false);
      setActionError(message);
    }
  }, [repository, logout]);

  return {
    profile,
    isLoading,
    isExporting,
    isDeleting,
    error,
    exportData,
    deleteAccount,
    updatePreferences,
  };
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && npx vitest run src/features/Settings/__tests__/useUserProfile.test.ts`
Expected: All tests PASS (8 total — 5 existing + 3 new).

- [ ] **Step 5: Commit**

```bash
git add web/src/features/Settings/useUserProfile.ts web/src/features/Settings/__tests__/useUserProfile.test.ts
git commit -m "feat(web): add optimistic updatePreferences to useUserProfile hook"
```

---

### Task 5: Web — Add Notifications section to SettingsPage

**Files:**
- Modify: `web/src/features/Settings/SettingsPage.tsx`
- Modify: `web/src/features/Settings/SettingsPage.module.css`
- Modify: `web/src/features/Settings/__tests__/SettingsPage.test.tsx`

- [ ] **Step 1: Write failing tests for the Notifications section**

Add the following tests to the end of the `describe` block in `web/src/features/Settings/__tests__/SettingsPage.test.tsx`:

```typescript
  it('renders email digest toggle', async () => {
    spy.fetchProfileResult = freeUserProfile({ emailDigestEnabled: true });

    renderSettingsPage(spy);

    const toggle = await screen.findByRole('switch', { name: /email digest/i });
    expect(toggle).toHaveAttribute('aria-checked', 'true');
  });

  it('renders instant emails toggle', async () => {
    spy.fetchProfileResult = freeUserProfile({ emailInstantEnabled: false });

    renderSettingsPage(spy);

    const toggle = await screen.findByRole('switch', { name: /instant email/i });
    expect(toggle).toHaveAttribute('aria-checked', 'false');
  });

  it('renders digest day picker when digest is enabled', async () => {
    spy.fetchProfileResult = freeUserProfile({ emailDigestEnabled: true, digestDay: 1 });

    renderSettingsPage(spy);

    const select = await screen.findByLabelText(/digest day/i);
    expect(select).toHaveValue('1');
  });

  it('hides digest day picker when digest is disabled', async () => {
    spy.fetchProfileResult = freeUserProfile({ emailDigestEnabled: false });

    renderSettingsPage(spy);

    await screen.findByRole('heading', { name: /notifications/i });
    expect(screen.queryByLabelText(/digest day/i)).not.toBeInTheDocument();
  });

  it('calls updateProfile when email digest is toggled off', async () => {
    const user = userEvent.setup();
    spy.fetchProfileResult = freeUserProfile({ emailDigestEnabled: true });

    renderSettingsPage(spy);

    const toggle = await screen.findByRole('switch', { name: /email digest/i });
    await user.click(toggle);

    expect(spy.updateProfileCalls).toBe(1);
    expect(spy.updateProfileLastRequest?.emailDigestEnabled).toBe(false);
  });

  it('calls updateProfile when digest day is changed', async () => {
    const user = userEvent.setup();
    spy.fetchProfileResult = freeUserProfile({ emailDigestEnabled: true, digestDay: 1 });

    renderSettingsPage(spy);

    const select = await screen.findByLabelText(/digest day/i);
    await user.selectOptions(select, '5');

    expect(spy.updateProfileCalls).toBe(1);
    expect(spy.updateProfileLastRequest?.digestDay).toBe(5);
  });
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && npx vitest run src/features/Settings/__tests__/SettingsPage.test.tsx`
Expected: FAIL — no switch role or Notifications heading found.

- [ ] **Step 3: Add notification CSS styles**

In `web/src/features/Settings/SettingsPage.module.css`, add at the end of the file (after the `.legalLink:hover` block):

```css
.toggleRow {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--tc-space-sm) 0;
}

.toggleRow + .toggleRow,
.toggleRow + .selectRow,
.selectRow + .toggleRow {
  border-top: 1px solid var(--tc-border);
}

.selectRow {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--tc-space-sm) 0;
}

.select {
  padding: var(--tc-space-xs) var(--tc-space-sm);
  font-size: var(--tc-text-body);
  color: var(--tc-text-primary);
  background: var(--tc-surface);
  border: 1px solid var(--tc-border);
  border-radius: var(--tc-radius-md);
  cursor: pointer;
  transition: border-color var(--tc-duration-fast);
}

.select:hover {
  border-color: var(--tc-amber);
}
```

- [ ] **Step 4: Add Notifications section to SettingsPage**

In `web/src/features/Settings/SettingsPage.tsx`, add the Toggle import at line 7 (after the ThemeToggle import):

```typescript
import { Toggle } from '../../components/Toggle/Toggle';
import { DAY_OF_WEEK_LABELS, type DayOfWeek } from '../../domain/types';
```

Destructure `updatePreferences` from the hook — replace lines 18-27:

```typescript
  const {
    profile,
    isLoading,
    isExporting,
    isDeleting,
    error,
    exportData,
    deleteAccount,
    updatePreferences,
  } = useUserProfile(repository, () => logout());
```

Insert the Notifications section between the Profile section and the Appearance section. After the closing `</section>` of the Profile section (after line 64) and before the `{/* Appearance section */}` comment, add:

```tsx
      {/* Notifications section */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Notifications</h2>
        <div className={styles.card}>
          <div className={styles.toggleRow}>
            <span className={styles.label}>Email digest</span>
            <Toggle
              checked={profile?.emailDigestEnabled ?? true}
              onChange={(checked) => updatePreferences({ emailDigestEnabled: checked })}
              label="Email digest"
            />
          </div>
          {profile?.emailDigestEnabled && (
            <div className={styles.selectRow}>
              <label htmlFor="digest-day" className={styles.label}>Digest day</label>
              <select
                id="digest-day"
                className={styles.select}
                value={profile.digestDay}
                onChange={(e) => updatePreferences({ digestDay: Number(e.target.value) as DayOfWeek })}
              >
                {([1, 2, 3, 4, 5, 6, 0] as const).map((day) => (
                  <option key={day} value={day}>
                    {DAY_OF_WEEK_LABELS[day]}
                  </option>
                ))}
              </select>
            </div>
          )}
          <div className={styles.toggleRow}>
            <span className={styles.label}>Instant emails</span>
            <Toggle
              checked={profile?.emailInstantEnabled ?? false}
              onChange={(checked) => updatePreferences({ emailInstantEnabled: checked })}
              label="Instant emails"
            />
          </div>
        </div>
      </section>
```

- [ ] **Step 5: Run all tests to verify they pass**

Run: `cd web && npx vitest run src/features/Settings/__tests__/SettingsPage.test.tsx`
Expected: All tests PASS (17 total — 11 existing + 6 new).

- [ ] **Step 6: Run full type check and web test suite**

Run: `cd web && npx tsc --noEmit && npx vitest run`
Expected: No type errors. All tests PASS.

- [ ] **Step 7: Commit**

```bash
git add web/src/features/Settings/SettingsPage.tsx web/src/features/Settings/SettingsPage.module.css web/src/features/Settings/__tests__/SettingsPage.test.tsx
git commit -m "feat(web): add Notifications section to settings page"
```

---

### Task 6: Full build verification

- [ ] **Step 1: Run .NET build and tests**

Run: `cd api && dotnet build && dotnet test`
Expected: Build succeeds. All tests pass.

- [ ] **Step 2: Run web build and tests**

Run: `cd web && npm run build && npx vitest run`
Expected: Build succeeds. All tests pass.

- [ ] **Step 3: Push**

```bash
git push
```

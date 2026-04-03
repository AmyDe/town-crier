# Map Confirm Step Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace raw lat/long on watch zone confirmation with an interactive Leaflet map showing a pin and radius circle, in both the onboarding and watch zone creation flows.

**Architecture:** Shared `ConfirmMap` component wraps `react-leaflet` (`MapContainer`, `TileLayer`, `Marker`, `Circle`). Both `useOnboarding` and `useCreateWatchZone` hooks gain a `postcode` state field and thread it from `PostcodeInput`. The watch zone creation flow gains a new `confirm` step.

**Tech Stack:** React, TypeScript, react-leaflet (already installed), Leaflet (already installed), CSS Modules with design tokens, Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-04-03-map-confirm-step-design.md`

---

### File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `web/src/components/ConfirmMap/ConfirmMap.tsx` | Shared map preview with marker + radius circle |
| Create | `web/src/components/ConfirmMap/ConfirmMap.module.css` | Map container styling |
| Create | `web/src/components/ConfirmMap/__tests__/ConfirmMap.test.tsx` | Tests for map rendering |
| Modify | `web/src/components/PostcodeInput/PostcodeInput.tsx` | Pass postcode string to `onGeocode` callback |
| Modify | `web/src/features/onboarding/useOnboarding.ts` | Add `postcode` state, update `handleGeocode` signature |
| Modify | `web/src/features/onboarding/OnboardingPage.tsx` | Use `ConfirmMap` + postcode label on confirm step |
| Modify | `web/src/features/onboarding/OnboardingPage.module.css` | Add `.mapWrapper` style |
| Modify | `web/src/features/onboarding/__tests__/OnboardingPage.test.tsx` | Add Leaflet mocks, verify postcode display |
| Modify | `web/src/features/WatchZones/useCreateWatchZone.ts` | Add `postcode` state, add `confirm` step |
| Modify | `web/src/features/WatchZones/WatchZoneCreatePage.tsx` | Add confirm step with `ConfirmMap` |
| Modify | `web/src/features/WatchZones/WatchZoneCreatePage.module.css` | Add `.mapWrapper`, `.confirmRow`, `.confirmLabel`, `.confirmValue` styles |
| Modify | `web/src/features/WatchZones/__tests__/WatchZoneCreatePage.test.tsx` | Add Leaflet mocks, test confirm step in flow |

---

### Task 1: ConfirmMap Component

**Files:**
- Create: `web/src/components/ConfirmMap/__tests__/ConfirmMap.test.tsx`
- Create: `web/src/components/ConfirmMap/ConfirmMap.tsx`
- Create: `web/src/components/ConfirmMap/ConfirmMap.module.css`

- [ ] **Step 1: Write the failing test**

Create `web/src/components/ConfirmMap/__tests__/ConfirmMap.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { ConfirmMap } from '../ConfirmMap';

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: () => <div data-testid="map-marker" />,
  Circle: () => <div data-testid="map-circle" />,
  useMap: () => ({
    fitBounds: vi.fn(),
  }),
}));

vi.mock('leaflet', () => ({
  default: {
    icon: () => ({}),
    Icon: { Default: { mergeOptions: () => {} } },
    latLng: (lat: number, lng: number) => ({ lat, lng }),
    latLngBounds: () => ({
      pad: () => ({ lat: 0, lng: 0 }),
    }),
  },
  icon: () => ({}),
  Icon: { Default: { mergeOptions: () => {} } },
  latLng: (lat: number, lng: number) => ({ lat, lng }),
  latLngBounds: () => ({
    pad: () => ({ lat: 0, lng: 0 }),
  }),
}));

describe('ConfirmMap', () => {
  it('renders map container with marker and circle', () => {
    render(
      <ConfirmMap latitude={51.5074} longitude={-0.1278} radiusMetres={2000} />,
    );

    expect(screen.getByTestId('map-container')).toBeInTheDocument();
    expect(screen.getByTestId('tile-layer')).toBeInTheDocument();
    expect(screen.getByTestId('map-marker')).toBeInTheDocument();
    expect(screen.getByTestId('map-circle')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run src/components/ConfirmMap/__tests__/ConfirmMap.test.tsx`
Expected: FAIL — cannot resolve `../ConfirmMap`

- [ ] **Step 3: Write the ConfirmMap component**

Create `web/src/components/ConfirmMap/ConfirmMap.module.css`:

```css
.container {
  border-radius: var(--tc-radius-md);
  overflow: hidden;
  height: 250px;
  margin-bottom: var(--tc-space-md);
}
```

Create `web/src/components/ConfirmMap/ConfirmMap.tsx`:

```tsx
import { useEffect } from 'react';
import { MapContainer, TileLayer, Marker, Circle, useMap } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import styles from './ConfirmMap.module.css';

const OSM_TILE_URL = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
const OSM_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

const CIRCLE_OPTIONS = {
  color: 'rgba(74, 108, 247, 0.8)',
  fillColor: 'rgba(74, 108, 247, 0.15)',
  fillOpacity: 1,
  weight: 2,
};

interface Props {
  latitude: number;
  longitude: number;
  radiusMetres: number;
}

function FitToCircle({ latitude, longitude, radiusMetres }: Props) {
  const map = useMap();

  useEffect(() => {
    const centre = L.latLng(latitude, longitude);
    const circle = L.circle(centre, { radius: radiusMetres });
    map.fitBounds(circle.getBounds().pad(0.1));
  }, [map, latitude, longitude, radiusMetres]);

  return null;
}

export function ConfirmMap({ latitude, longitude, radiusMetres }: Props) {
  const centre: [number, number] = [latitude, longitude];

  return (
    <div className={styles.container}>
      <MapContainer
        center={centre}
        zoom={13}
        style={{ height: '100%', width: '100%' }}
        zoomControl={false}
        attributionControl={true}
      >
        <TileLayer url={OSM_TILE_URL} attribution={OSM_ATTRIBUTION} />
        <Marker position={centre} />
        <Circle center={centre} radius={radiusMetres} pathOptions={CIRCLE_OPTIONS} />
        <FitToCircle latitude={latitude} longitude={longitude} radiusMetres={radiusMetres} />
      </MapContainer>
    </div>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run src/components/ConfirmMap/__tests__/ConfirmMap.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/ConfirmMap/
git commit -m "feat(web): add ConfirmMap component with marker and radius circle"
```

---

### Task 2: Thread Postcode Through PostcodeInput Callback

**Files:**
- Modify: `web/src/components/PostcodeInput/PostcodeInput.tsx:9,16-19`
- Modify: `web/src/components/PostcodeInput/__tests__/PostcodeInput.test.tsx`

- [ ] **Step 1: Update PostcodeInput test to verify postcode is passed**

In `web/src/components/PostcodeInput/__tests__/PostcodeInput.test.tsx`, find the test that verifies the `onGeocode` callback is called. Update the assertion to check the second argument is the postcode string. In the test for successful geocode:

```tsx
// Existing assertion (update):
expect(onGeocode).toHaveBeenCalledWith(geocodingSpy.geocodeResult, 'SW1A 1AA');
```

The `onGeocode` spy is `vi.fn()`. After the user types `'SW1A 1AA'` and clicks "Look up", the callback should now receive both the geocode result and the postcode string.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run src/components/PostcodeInput/__tests__/PostcodeInput.test.tsx`
Expected: FAIL — `onGeocode` called with only one argument

- [ ] **Step 3: Update PostcodeInput to pass postcode**

In `web/src/components/PostcodeInput/PostcodeInput.tsx`:

Change the `Props` interface:
```tsx
interface Props {
  geocodingPort: GeocodingPort;
  onGeocode: (result: GeocodeResult, postcode: string) => void;
}
```

Change `handleLookup`:
```tsx
async function handleLookup() {
  const result = await lookup();
  if (result) {
    onGeocode(result, postcode);
  }
}
```

Note: `postcode` is already destructured from `usePostcodeGeocode` on line 12.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run src/components/PostcodeInput/__tests__/PostcodeInput.test.tsx`
Expected: PASS

- [ ] **Step 5: Run full test suite to check for breakage**

Run: `cd web && npx vitest run`
Expected: Some tests may fail if existing `onGeocode` handlers don't accept the second argument — this is expected and will be fixed in Tasks 3 and 4.

- [ ] **Step 6: Commit**

```bash
git add web/src/components/PostcodeInput/PostcodeInput.tsx web/src/components/PostcodeInput/__tests__/PostcodeInput.test.tsx
git commit -m "feat(web): pass postcode string through PostcodeInput onGeocode callback"
```

---

### Task 3: Update Onboarding Flow

**Files:**
- Modify: `web/src/features/onboarding/useOnboarding.ts`
- Modify: `web/src/features/onboarding/OnboardingPage.tsx`
- Modify: `web/src/features/onboarding/__tests__/OnboardingPage.test.tsx`

- [ ] **Step 1: Update onboarding test to verify postcode display**

In `web/src/features/onboarding/__tests__/OnboardingPage.test.tsx`, add the Leaflet mocks at the top of the file (after imports, before `describe`):

```tsx
vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: () => <div data-testid="map-marker" />,
  Circle: () => <div data-testid="map-circle" />,
  useMap: () => ({
    fitBounds: vi.fn(),
  }),
}));

vi.mock('leaflet', () => ({
  default: {
    icon: () => ({}),
    Icon: { Default: { mergeOptions: () => {} } },
    latLng: (lat: number, lng: number) => ({ lat, lng }),
    circle: () => ({ getBounds: () => ({ pad: () => ({}) }) }),
  },
  icon: () => ({}),
  Icon: { Default: { mergeOptions: () => {} } },
  latLng: (lat: number, lng: number) => ({ lat, lng }),
  circle: () => ({ getBounds: () => ({ pad: () => ({}) }) }),
}));
```

Add `vi` to the imports (if not already imported).

Add a new test case:

```tsx
it('shows postcode and map on confirm step', async () => {
  const user = userEvent.setup();
  const geocodingSpy = new SpyGeocodingPort();
  renderOnboarding(new SpyOnboardingPort(), geocodingSpy);

  await user.click(screen.getByRole('button', { name: /get started/i }));
  await user.type(screen.getByLabelText(/postcode/i), 'SW1A 1AA');
  await user.click(screen.getByRole('button', { name: /look up/i }));

  await waitFor(() => {
    expect(screen.getByRole('radiogroup', { name: /radius/i })).toBeInTheDocument();
  });

  await user.click(screen.getByLabelText('2 km'));
  await user.click(screen.getByRole('button', { name: /next/i }));

  expect(screen.getByTestId('map-container')).toBeInTheDocument();
  expect(screen.getByText('SW1A 1AA')).toBeInTheDocument();
});
```

- [ ] **Step 2: Run tests to verify the new test fails**

Run: `cd web && npx vitest run src/features/onboarding/__tests__/OnboardingPage.test.tsx`
Expected: FAIL — no `map-container` or `SW1A 1AA` text on confirm step

- [ ] **Step 3: Update useOnboarding hook**

In `web/src/features/onboarding/useOnboarding.ts`:

Add `postcode` state:
```tsx
const [postcode, setPostcode] = useState('');
```

Update `handleGeocode` to accept and store postcode:
```tsx
const handleGeocode = useCallback((result: GeocodeResult, enteredPostcode: string) => {
  setGeocode(result);
  setPostcode(enteredPostcode);
  setStep('radius');
}, []);
```

Add `postcode` to the return object:
```tsx
return {
  step,
  geocode,
  postcode,
  radiusMetres,
  // ... rest unchanged
};
```

- [ ] **Step 4: Update OnboardingPage confirm step**

In `web/src/features/onboarding/OnboardingPage.tsx`:

Add `ConfirmMap` import:
```tsx
import { ConfirmMap } from '../../components/ConfirmMap/ConfirmMap';
```

Add `postcode` to the destructured hook values:
```tsx
const {
  step,
  geocode,
  postcode,
  radiusMetres,
  // ... rest unchanged
} = useOnboarding(onboardingPort);
```

Replace the confirm step JSX (lines 79–110) with:
```tsx
{step === 'confirm' && (
  <>
    <h2 className={styles.stepLabel}>Confirm your watch zone</h2>
    {geocode && (
      <ConfirmMap
        latitude={geocode.latitude}
        longitude={geocode.longitude}
        radiusMetres={radiusMetres}
      />
    )}
    <div className={styles.confirmDetails}>
      <div className={styles.confirmRow}>
        <span className={styles.confirmLabel}>Postcode</span>
        <span className={styles.confirmValue}>{postcode}</span>
      </div>
      <div className={styles.confirmRow}>
        <span className={styles.confirmLabel}>Radius</span>
        <span className={styles.confirmValue}>
          {radiusMetres >= 1000 ? `${radiusMetres / 1000} km` : `${radiusMetres} m`}
        </span>
      </div>
    </div>
    <button
      type="button"
      className={styles.primaryButton}
      onClick={finish}
      disabled={isSubmitting}
    >
      {isSubmitting ? 'Setting up...' : 'Confirm'}
    </button>
    {error && (
      <p className={styles.error} role="alert">
        {error}
      </p>
    )}
  </>
)}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd web && npx vitest run src/features/onboarding/__tests__/OnboardingPage.test.tsx`
Expected: PASS (all tests including the new one)

- [ ] **Step 6: Commit**

```bash
git add web/src/features/onboarding/ web/src/features/onboarding/__tests__/
git commit -m "feat(web): replace lat/long with map preview on onboarding confirm step"
```

---

### Task 4: Add Confirm Step to Watch Zone Creation

**Files:**
- Modify: `web/src/features/WatchZones/useCreateWatchZone.ts`
- Modify: `web/src/features/WatchZones/WatchZoneCreatePage.tsx`
- Modify: `web/src/features/WatchZones/WatchZoneCreatePage.module.css`
- Modify: `web/src/features/WatchZones/__tests__/WatchZoneCreatePage.test.tsx`

- [ ] **Step 1: Update WatchZoneCreatePage test for new confirm step**

In `web/src/features/WatchZones/__tests__/WatchZoneCreatePage.test.tsx`, add the Leaflet mocks at the top (after imports, before `describe`):

```tsx
vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: () => <div data-testid="map-marker" />,
  Circle: () => <div data-testid="map-circle" />,
  useMap: () => ({
    fitBounds: vi.fn(),
  }),
}));

vi.mock('leaflet', () => ({
  default: {
    icon: () => ({}),
    Icon: { Default: { mergeOptions: () => {} } },
    latLng: (lat: number, lng: number) => ({ lat, lng }),
    circle: () => ({ getBounds: () => ({ pad: () => ({}) }) }),
  },
  icon: () => ({}),
  Icon: { Default: { mergeOptions: () => {} } },
  latLng: (lat: number, lng: number) => ({ lat, lng }),
  circle: () => ({ getBounds: () => ({ pad: () => ({}) }) }),
}));
```

Add `vi` to the vitest imports if not already present.

Update the "saves zone with form data" test — change the "Save" click to "Next", then add a confirm click:

```tsx
it('saves zone with form data', async () => {
  const user = userEvent.setup();
  repoSpy.createResult = aWatchZone();

  renderWithRouter(
    <WatchZoneCreatePage
      repository={repoSpy}
      geocodingPort={geocodingSpy}
      navigate={navigate}
    />,
  );

  // Step 1: Look up postcode
  await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'CB1 2AD');
  await user.click(screen.getByRole('button', { name: /look up/i }));

  // Step 2: Fill in details
  const nameInput = await screen.findByLabelText(/zone name/i);
  await user.type(nameInput, 'Home');
  await user.click(screen.getByLabelText('5 km'));

  // Advance to confirm step
  await user.click(screen.getByRole('button', { name: /next/i }));

  // Step 3: Confirm — should see map and postcode
  expect(screen.getByTestId('map-container')).toBeInTheDocument();
  expect(screen.getByText('CB1 2AD')).toBeInTheDocument();

  // Confirm
  await user.click(screen.getByRole('button', { name: /confirm/i }));

  expect(repoSpy.createCalls).toHaveLength(1);
  expect(repoSpy.createCalls[0]?.name).toBe('Home');
  expect(repoSpy.createCalls[0]?.radiusMetres).toBe(5000);
  expect(navigatedTo).toBe('/watch-zones');
});
```

Update the "shows error when save fails" test similarly — advance through the confirm step:

```tsx
it('shows error when save fails', async () => {
  const user = userEvent.setup();
  repoSpy.createError = new Error('Create failed');

  renderWithRouter(
    <WatchZoneCreatePage
      repository={repoSpy}
      geocodingPort={geocodingSpy}
      navigate={navigate}
    />,
  );

  await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'CB1 2AD');
  await user.click(screen.getByRole('button', { name: /look up/i }));

  const nameInput = await screen.findByLabelText(/zone name/i);
  await user.type(nameInput, 'Home');

  await user.click(screen.getByRole('button', { name: /next/i }));
  await user.click(screen.getByRole('button', { name: /confirm/i }));

  expect(await screen.findByText('Create failed')).toBeInTheDocument();
  expect(navigatedTo).toBeNull();
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && npx vitest run src/features/WatchZones/__tests__/WatchZoneCreatePage.test.tsx`
Expected: FAIL — no "Next" button (currently "Save"), no confirm step

- [ ] **Step 3: Update useCreateWatchZone hook**

In `web/src/features/WatchZones/useCreateWatchZone.ts`:

Change the step type:
```tsx
type CreateStep = 'postcode' | 'details' | 'confirm';
```

Add `postcode` to state interface:
```tsx
interface CreateWatchZoneState {
  step: CreateStep;
  name: string;
  postcode: string;
  coordinates: GeocodeResult | null;
  radiusMetres: number;
  authorityId: AuthorityId | null;
  isSaving: boolean;
  error: string | null;
}
```

Add `postcode: ''` to the initial state object.

Update `setGeocode` to accept and store the postcode:
```tsx
const setGeocode = useCallback((result: GeocodeResult, enteredPostcode: string) => {
  setState(prev => ({
    ...prev,
    coordinates: result,
    postcode: enteredPostcode,
    step: 'details',
    error: null,
  }));
}, []);
```

Add a `confirmDetails` callback:
```tsx
const confirmDetails = useCallback(() => {
  if (!state.name.trim()) {
    setState(prev => ({ ...prev, error: 'Please enter a name for this watch zone' }));
    return;
  }
  setState(prev => ({ ...prev, step: 'confirm', error: null }));
}, [state.name]);
```

Remove the name validation from `save` (it's now in `confirmDetails`):
```tsx
const save = useCallback(async () => {
  if (!state.coordinates) {
    setState(prev => ({ ...prev, error: 'Please look up a postcode first' }));
    return;
  }

  setState(prev => ({ ...prev, isSaving: true, error: null }));
  try {
    await repository.create({
      name: state.name.trim(),
      latitude: state.coordinates.latitude,
      longitude: state.coordinates.longitude,
      radiusMetres: state.radiusMetres,
      authorityId: state.authorityId ?? undefined,
    });
    navigate('/watch-zones');
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'An error occurred';
    setState(prev => ({ ...prev, isSaving: false, error: message }));
  }
}, [state.coordinates, state.name, state.radiusMetres, state.authorityId, repository, navigate]);
```

Add `postcode` and `confirmDetails` to the return object:
```tsx
return {
  step: state.step,
  name: state.name,
  postcode: state.postcode,
  coordinates: state.coordinates,
  radiusMetres: state.radiusMetres,
  isSaving: state.isSaving,
  error: state.error,
  setGeocode,
  setName,
  setRadiusMetres,
  setAuthorityId,
  confirmDetails,
  save,
};
```

- [ ] **Step 4: Update WatchZoneCreatePage with confirm step**

In `web/src/features/WatchZones/WatchZoneCreatePage.tsx`:

Add `ConfirmMap` import:
```tsx
import { ConfirmMap } from '../../components/ConfirmMap/ConfirmMap';
```

Add `postcode`, `confirmDetails`, and `isSaving` to destructured hook values:
```tsx
const {
  step,
  name,
  postcode,
  coordinates,
  radiusMetres,
  isSaving,
  error,
  setGeocode,
  setName,
  setRadiusMetres,
  confirmDetails,
  save,
} = useCreateWatchZone(repository, navigate);
```

In the details step, change the Save button to Next and call `confirmDetails`:
```tsx
<div className={styles.actions}>
  <button
    type="button"
    className={styles.saveButton}
    onClick={confirmDetails}
  >
    Next
  </button>
</div>
```

Add the new confirm step after the details step:
```tsx
{step === 'confirm' && (
  <section className={styles.section}>
    {coordinates && (
      <ConfirmMap
        latitude={coordinates.latitude}
        longitude={coordinates.longitude}
        radiusMetres={radiusMetres}
      />
    )}
    <div className={styles.confirmDetails}>
      <div className={styles.confirmRow}>
        <span className={styles.confirmLabel}>Postcode</span>
        <span className={styles.confirmValue}>{postcode}</span>
      </div>
      <div className={styles.confirmRow}>
        <span className={styles.confirmLabel}>Name</span>
        <span className={styles.confirmValue}>{name}</span>
      </div>
      <div className={styles.confirmRow}>
        <span className={styles.confirmLabel}>Radius</span>
        <span className={styles.confirmValue}>
          {radiusMetres >= 1000 ? `${radiusMetres / 1000} km` : `${radiusMetres} m`}
        </span>
      </div>
    </div>
    {error && (
      <p className={styles.error} role="alert">
        {error}
      </p>
    )}
    <div className={styles.actions}>
      <button
        type="button"
        className={styles.saveButton}
        onClick={save}
        disabled={isSaving}
      >
        {isSaving ? 'Saving...' : 'Confirm'}
      </button>
    </div>
  </section>
)}
```

- [ ] **Step 5: Add confirm styles to WatchZoneCreatePage CSS**

In `web/src/features/WatchZones/WatchZoneCreatePage.module.css`, add at the end:

```css
.confirmDetails {
  background: var(--tc-surface-elevated);
  border-radius: var(--tc-radius-sm);
  padding: var(--tc-space-md);
  margin-bottom: var(--tc-space-lg);
}

.confirmRow {
  display: flex;
  justify-content: space-between;
  padding: var(--tc-space-xs) 0;
}

.confirmLabel {
  font-size: var(--tc-text-body);
  color: var(--tc-text-secondary);
}

.confirmValue {
  font-size: var(--tc-text-body);
  font-weight: var(--tc-font-semibold);
  color: var(--tc-text-primary);
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd web && npx vitest run src/features/WatchZones/__tests__/WatchZoneCreatePage.test.tsx`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add web/src/features/WatchZones/
git commit -m "feat(web): add map confirm step to watch zone creation flow"
```

---

### Task 5: Full Test Suite + Type Check

**Files:** None (verification only)

- [ ] **Step 1: Run type checker**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 2: Run full test suite**

Run: `cd web && npx vitest run`
Expected: All tests pass

- [ ] **Step 3: Run dev server smoke test**

Run: `cd web && npx vite build`
Expected: Build succeeds with no errors

- [ ] **Step 4: Commit any remaining fixes**

If type check or tests revealed issues, fix and commit:
```bash
git add -A && git commit -m "fix(web): resolve type/test issues from map confirm step"
```

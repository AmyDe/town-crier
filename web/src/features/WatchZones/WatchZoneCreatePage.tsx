import { Link } from 'react-router-dom';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import type { GeocodingPort } from '../../domain/ports/geocoding-port';
import { useCreateWatchZone } from './useCreateWatchZone';
import { PostcodeInput } from '../../components/PostcodeInput/PostcodeInput';
import { RadiusPicker } from '../../components/RadiusPicker/RadiusPicker';
import styles from './WatchZoneCreatePage.module.css';

interface Props {
  repository: WatchZoneRepository;
  geocodingPort: GeocodingPort;
  navigate: (path: string) => void;
}

export function WatchZoneCreatePage({ repository, geocodingPort, navigate }: Props) {
  const {
    step,
    name,
    radiusMetres,
    isSaving,
    error,
    setGeocode,
    setName,
    setRadiusMetres,
    save,
  } = useCreateWatchZone(repository, navigate);

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Create Watch Zone</h1>
        <Link to="/watch-zones" className={styles.cancelLink}>
          Cancel
        </Link>
      </div>

      {step === 'postcode' && (
        <section className={styles.section}>
          <p className={styles.instruction}>
            Enter a postcode to set the centre of your watch zone.
          </p>
          <PostcodeInput geocodingPort={geocodingPort} onGeocode={setGeocode} />
        </section>
      )}

      {step === 'details' && (
        <section className={styles.section}>
          <div className={styles.field}>
            <label htmlFor="zone-name" className={styles.label}>
              Zone name
            </label>
            <input
              id="zone-name"
              type="text"
              className={styles.input}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Home, Office"
            />
          </div>

          <RadiusPicker selectedMetres={radiusMetres} onSelect={setRadiusMetres} />

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
              {isSaving ? 'Saving...' : 'Save'}
            </button>
          </div>
        </section>
      )}

      {step === 'postcode' && error && (
        <p className={styles.error} role="alert">
          {error}
        </p>
      )}
    </div>
  );
}

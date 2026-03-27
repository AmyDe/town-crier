import styles from './RadiusPicker.module.css';

const RADIUS_OPTIONS = [
  { label: '1 km', metres: 1000 },
  { label: '2 km', metres: 2000 },
  { label: '5 km', metres: 5000 },
  { label: '10 km', metres: 10000 },
] as const;

interface Props {
  selectedMetres: number;
  onSelect: (metres: number) => void;
}

export function RadiusPicker({ selectedMetres, onSelect }: Props) {
  return (
    <fieldset className={styles.fieldset} role="radiogroup" aria-label="Radius">
      <legend className={styles.legend}>Radius</legend>
      <div className={styles.options}>
        {RADIUS_OPTIONS.map((option) => (
          <label key={option.metres} className={styles.option}>
            <input
              type="radio"
              name="radius"
              value={option.metres}
              checked={selectedMetres === option.metres}
              onChange={() => onSelect(option.metres)}
              className={styles.radio}
            />
            <span className={styles.label}>{option.label}</span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}

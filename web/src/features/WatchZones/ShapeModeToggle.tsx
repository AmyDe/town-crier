import styles from './ShapeModeToggle.module.css';

/**
 * Whether a zone being created/edited is a plain circle (centre + radius) or
 * a hand-drawn custom shape. Personal/Pro only — see GH#1031.
 */
export type ShapeMode = 'circle' | 'custom';

interface Props {
  mode: ShapeMode;
  onSelect: (mode: ShapeMode) => void;
}

const OPTIONS: ReadonlyArray<{ mode: ShapeMode; label: string }> = [
  { mode: 'circle', label: 'Circle' },
  { mode: 'custom', label: 'Custom shape' },
];

export function ShapeModeToggle({ mode, onSelect }: Props) {
  return (
    <fieldset className={styles.fieldset} role="radiogroup" aria-label="Shape">
      <legend className={styles.legend}>Shape</legend>
      <div className={styles.options}>
        {OPTIONS.map((option) => (
          <label key={option.mode} className={styles.option}>
            <input
              type="radio"
              name="shape-mode"
              value={option.mode}
              checked={mode === option.mode}
              onChange={() => onSelect(option.mode)}
              className={styles.radio}
            />
            <span className={styles.label}>{option.label}</span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}

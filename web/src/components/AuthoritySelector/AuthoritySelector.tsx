import type { AuthorityListItem } from '../../domain/types';
import type { AuthoritySearchPort } from '../../domain/ports/authority-search-port';
import { useAuthoritySearch } from './useAuthoritySearch';
import styles from './AuthoritySelector.module.css';

interface Props {
  searchPort: AuthoritySearchPort;
  onSelect: (authority: AuthorityListItem) => void;
}

export function AuthoritySelector({ searchPort, onSelect }: Props) {
  const { query, results, isSearching, setQuery, clearResults } =
    useAuthoritySearch(searchPort);

  function handleSelect(authority: AuthorityListItem) {
    setQuery(authority.name);
    clearResults();
    onSelect(authority);
  }

  const showDropdown = results.length > 0;

  return (
    <div className={styles.container}>
      <input
        type="text"
        role="combobox"
        aria-label="Search authorities"
        aria-expanded={showDropdown}
        aria-autocomplete="list"
        aria-controls="authority-listbox"
        className={styles.input}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search for a local authority..."
      />
      {isSearching && (
        <div className={styles.loading} aria-live="polite">
          Searching...
        </div>
      )}
      {showDropdown && (
        <ul
          id="authority-listbox"
          role="listbox"
          className={styles.dropdown}
        >
          {results.map((authority) => (
            <li key={authority.id} role="option" className={styles.option}>
              <button
                type="button"
                className={styles.optionButton}
                onClick={() => handleSelect(authority)}
              >
                <span className={styles.authorityName}>{authority.name}</span>
                <span className={styles.areaType}>{authority.areaType}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

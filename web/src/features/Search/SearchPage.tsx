import { useEffect } from 'react';
import type { SearchPort } from '../../domain/ports/search-port';
import { useSearch } from './useSearch';
import { SearchResultCard } from './components/SearchResultCard';
import styles from './SearchPage.module.css';

interface Props {
  port: SearchPort;
}

const PAGE_TITLE = 'Search UK planning applications | Town Crier';
const PAGE_DESCRIPTION =
  'Search UK council planning applications by reference, address, or description, and copy a shareable link to any result.';

/**
 * Public, anonymous `/search` page (#821 Phase 4) — outside `AuthGuard`, works
 * fully logged out. Sets its own `<title>`/meta description on mount so the
 * page is indexable in its own right, distinct from the landing page; results
 * are client-rendered and deliberately never added to `sitemap.xml`.
 */
export function SearchPage({ port }: Props) {
  const {
    query,
    setQuery,
    authority,
    setAuthority,
    results,
    isLoading,
    error,
    refineQuery,
    hasSearched,
  } = useSearch(port);

  useEffect(() => {
    const previousTitle = document.title;
    document.title = PAGE_TITLE;

    const metaDescription = document.querySelector('meta[name="description"]');
    const previousDescription = metaDescription?.getAttribute('content') ?? null;
    metaDescription?.setAttribute('content', PAGE_DESCRIPTION);

    return () => {
      document.title = previousTitle;
      if (metaDescription && previousDescription !== null) {
        metaDescription.setAttribute('content', previousDescription);
      }
    };
  }, []);

  const showEmptyState = hasSearched && !isLoading && error === null && results.length === 0;

  return (
    <main className={styles.container}>
      <h1 className={styles.heading}>Search planning applications</h1>
      <p className={styles.intro}>
        Find a UK council planning application by reference, address, or description, then copy a
        link to share it.
      </p>

      <form className={styles.form} onSubmit={(event) => event.preventDefault()}>
        <div className={styles.field}>
          <label className={styles.label} htmlFor="search-query">
            Search
          </label>
          <input
            id="search-query"
            type="search"
            className={styles.input}
            placeholder="Reference, address, or description"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>

        <div className={styles.field}>
          <label className={styles.label} htmlFor="search-authority">
            Council (optional)
          </label>
          <input
            id="search-authority"
            type="text"
            className={styles.input}
            placeholder="e.g. cambridge"
            value={authority}
            onChange={(event) => setAuthority(event.target.value)}
          />
        </div>
      </form>

      {isLoading && <p className={styles.status}>Searching…</p>}
      {error !== null && <p className={styles.error}>{error}</p>}
      {refineQuery && (
        <p className={styles.notice}>
          Showing the first 20 matches — try a more specific search to narrow the results.
        </p>
      )}
      {showEmptyState && <p className={styles.status}>No applications matched your search.</p>}

      {results.length > 0 && (
        <ul className={styles.results}>
          {results.map((result) => (
            <SearchResultCard key={`${result.authoritySlug}/${result.reference}`} result={result} />
          ))}
        </ul>
      )}
    </main>
  );
}

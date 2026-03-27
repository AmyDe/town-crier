import { useState, type FormEvent } from 'react';
import type { AuthorityListItem } from '../../domain/types';
import type { AuthoritySearchPort } from '../../domain/ports/authority-search-port';
import type { SearchRepository } from '../../domain/ports/search-repository';
import { useSearch } from './useSearch';
import { AuthoritySelector } from '../../components/AuthoritySelector/AuthoritySelector';
import { ApplicationCard } from '../../components/ApplicationCard/ApplicationCard';
import { Pagination } from '../../components/Pagination/Pagination';
import { ProGate } from '../../components/ProGate/ProGate';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import styles from './SearchPage.module.css';

interface Props {
  searchRepository: SearchRepository;
  authoritySearchPort: AuthoritySearchPort;
}

export function SearchPage({ searchRepository, authoritySearchPort }: Props) {
  const [query, setQuery] = useState('');
  const [selectedAuthority, setSelectedAuthority] = useState<AuthorityListItem | null>(null);
  const [hasSearched, setHasSearched] = useState(false);

  const {
    applications,
    page,
    totalPages,
    isLoading,
    error,
    proGateRequired,
    performSearch,
    goToNextPage,
    goToPreviousPage,
  } = useSearch(searchRepository);

  const canSearch = query.trim().length > 0 && selectedAuthority !== null && !isLoading;

  function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!canSearch || selectedAuthority === null) return;
    setHasSearched(true);
    performSearch(query.trim(), selectedAuthority.id);
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Search</h1>

      <form className={styles.form} onSubmit={handleSubmit}>
        <input
          type="text"
          className={styles.searchInput}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search for planning applications..."
          aria-label="Search query"
        />
        <AuthoritySelector
          searchPort={authoritySearchPort}
          onSelect={setSelectedAuthority}
        />
        <button
          type="submit"
          className={styles.searchButton}
          disabled={!canSearch}
        >
          Search
        </button>
      </form>

      {proGateRequired && <ProGate featureName="Search" />}

      {error !== null && (
        <p className={styles.error} role="alert">{error}</p>
      )}

      {isLoading && (
        <p className={styles.loading} aria-live="polite">Searching...</p>
      )}

      {!isLoading && !proGateRequired && error === null && hasSearched && applications.length === 0 && (
        <EmptyState
          icon="&#x1F50D;"
          title="No results"
          message="No planning applications matched your search. Try different keywords or another authority."
        />
      )}

      {!isLoading && applications.length > 0 && (
        <div className={styles.results}>
          {applications.map((app) => (
            <ApplicationCard key={app.uid} application={app} />
          ))}
          <Pagination
            page={page}
            totalPages={totalPages}
            onNext={goToNextPage}
            onPrevious={goToPreviousPage}
          />
        </div>
      )}
    </div>
  );
}

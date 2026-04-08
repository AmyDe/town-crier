import { useState, useCallback } from 'react';

interface UsePaginationOptions {
  loadPage: (page: number) => void;
  pageSize: number;
}

interface UsePaginationResult {
  page: number;
  totalPages: number;
  setPage: (page: number) => void;
  setTotal: (total: number) => void;
  goToNextPage: () => void;
  goToPreviousPage: () => void;
}

export function usePagination({ loadPage, pageSize }: UsePaginationOptions): UsePaginationResult {
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);

  const totalPages = total > 0 ? Math.ceil(total / pageSize) : 0;

  const goToNextPage = useCallback(() => {
    const next = page + 1;
    if (next <= totalPages) {
      loadPage(next);
    }
  }, [page, totalPages, loadPage]);

  const goToPreviousPage = useCallback(() => {
    const prev = page - 1;
    if (prev >= 1) {
      loadPage(prev);
    }
  }, [page, loadPage]);

  return {
    page,
    totalPages,
    setPage,
    setTotal,
    goToNextPage,
    goToPreviousPage,
  };
}

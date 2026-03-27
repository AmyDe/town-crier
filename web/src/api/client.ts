export class ApiRequestError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiRequestError';
    this.status = status;
  }
}

export interface ApiClient {
  get<T>(path: string, params?: Record<string, string>): Promise<T>;
  post<T>(path: string, body?: unknown): Promise<T>;
  put(path: string, body?: unknown): Promise<void>;
  patch<T>(path: string, body?: unknown): Promise<T>;
  delete(path: string): Promise<void>;
}

export function createApiClient(
  baseUrl: string,
  getToken: () => Promise<string>,
  fetchFn: typeof globalThis.fetch = globalThis.fetch,
): ApiClient {
  async function request<T>(method: string, path: string, body?: unknown, params?: Record<string, string>): Promise<T> {
    const token = await getToken();
    let url = `${baseUrl}${path}`;
    if (params) {
      const search = new URLSearchParams(params);
      url = `${url}?${search.toString()}`;
    }

    const headers: Record<string, string> = {
      Authorization: `Bearer ${token}`,
    };
    if (body !== undefined) {
      headers['Content-Type'] = 'application/json';
    }

    const response = await fetchFn(url, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });

    if (!response.ok) {
      let message = `Request failed with status ${response.status}`;
      try {
        const errorBody = await response.json() as { error?: string };
        if (errorBody.error) {
          message = errorBody.error;
        }
      } catch {
        // Use default message if body isn't JSON
      }
      throw new ApiRequestError(response.status, message);
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return response.json() as Promise<T>;
  }

  return {
    get: <T>(path: string, params?: Record<string, string>) => request<T>('GET', path, undefined, params),
    post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
    put: (path: string, body?: unknown) => request<void>('PUT', path, body),
    patch: <T>(path: string, body?: unknown) => request<T>('PATCH', path, body),
    delete: (path: string) => request<void>('DELETE', path),
  };
}

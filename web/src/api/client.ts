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
  /**
   * Like `get`, but also surfaces the raw response headers so callers can read
   * out-of-band metadata such as the `X-Next-Cursor` keyset-pagination header
   * exposed by the watch-zone applications list (GH#711). `get` stays the
   * header-blind fast path for the common case.
   */
  getWithHeaders<T>(
    path: string,
    params?: Record<string, string>,
  ): Promise<{ body: T; headers: Headers }>;
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
  async function requestRaw(
    method: string,
    path: string,
    body?: unknown,
    params?: Record<string, string>,
  ): Promise<{ body: unknown; response: Response }> {
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
      return { body: undefined, response };
    }

    return { body: await response.json(), response };
  }

  async function request<T>(method: string, path: string, body?: unknown, params?: Record<string, string>): Promise<T> {
    const { body: parsed } = await requestRaw(method, path, body, params);
    return parsed as T;
  }

  return {
    get: <T>(path: string, params?: Record<string, string>) => request<T>('GET', path, undefined, params),
    getWithHeaders: async <T>(path: string, params?: Record<string, string>) => {
      const { body, response } = await requestRaw('GET', path, undefined, params);
      return { body: body as T, headers: response.headers };
    },
    post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
    put: (path: string, body?: unknown) => request<void>('PUT', path, body),
    patch: <T>(path: string, body?: unknown) => request<T>('PATCH', path, body),
    delete: (path: string) => request<void>('DELETE', path),
  };
}

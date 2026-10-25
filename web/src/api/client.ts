export class ApiRequestError extends Error {
  readonly status: number;
  readonly code: string | undefined;

  constructor(status: number, message: string, code?: string) {
    super(message);
    this.name = 'ApiRequestError';
    this.status = status;
    this.code = code;
  }
}

export interface ApiClient {
  get<T>(path: string, params?: Record<string, string>): Promise<T>;
  /** Like `get`, but also returns the response headers, e.g. `X-Next-Cursor` for keyset pagination. */
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
      let code: string | undefined;
      try {
        const errorBody = await response.json() as { error?: string; message?: string };
        if (errorBody.message) {
          message = errorBody.message;
          code = errorBody.error;
        } else if (errorBody.error) {
          message = errorBody.error;
        }
      } catch {
        // Use default message if body isn't JSON
      }
      throw new ApiRequestError(response.status, message, code);
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

import type { Deps, Env } from "./route";

export const FRESH_SECONDS = 3_600;
export const STALE_SECONDS = 604_800;
export const ORIGIN_TIMEOUT_MS = 10_000;

const CACHED_AT = "x-tc-cached-at";
const ORIGIN_CACHE_CONTROL = "x-tc-origin-cache-control";
const CACHEABLE_STATUSES = new Set([200, 301, 404]);

/**
 * Fetches an SEO path from the API through the edge cache. `x-tc-cached-at` holds the
 * store time in epoch milliseconds; the stored copy keeps the origin's Cache-Control in
 * `x-tc-origin-cache-control` so clients never see the 7-day edge max-age.
 */
export async function fetchSeo(url: URL, env: Env, deps: Deps): Promise<Response> {
  const key = url.toString();
  const cached = await deps.cache.match(key);
  const ageSeconds = cached ? cachedAgeSeconds(cached, deps.now()) : Infinity;
  if (cached && ageSeconds < FRESH_SECONDS) {
    return fromCache(cached, false);
  }

  const origin = await fetchOrigin(new URL(env.API_ORIGIN).origin + url.pathname + url.search, env, deps);
  if (origin && origin.status < 500) {
    if (CACHEABLE_STATUSES.has(origin.status)) {
      deps.waitUntil(deps.cache.put(key, toStoredCopy(origin, deps.now())));
    }
    return toClient(origin);
  }

  if (cached && ageSeconds <= STALE_SECONDS) {
    return fromCache(cached, true);
  }
  return new Response("Service Unavailable", { status: 503, headers: { "Retry-After": "60" } });
}

interface OriginResponse {
  status: number;
  statusText: string;
  headers: Headers;
  body: ArrayBuffer;
}

async function fetchOrigin(target: string, env: Env, deps: Deps): Promise<OriginResponse | null> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), deps.originTimeoutMs ?? ORIGIN_TIMEOUT_MS);
  try {
    const response = await deps.fetch(target, {
      method: "GET",
      headers: { "X-Build-Key": env.SITE_BUILD_KEY },
      redirect: "manual",
      signal: controller.signal,
    });
    const body = await response.arrayBuffer();
    return { status: response.status, statusText: response.statusText, headers: response.headers, body };
  } catch {
    return null;
  } finally {
    clearTimeout(timer);
  }
}

function cachedAgeSeconds(cached: Response, now: number): number {
  const storedAt = Number(cached.headers.get(CACHED_AT));
  return Number.isFinite(storedAt) && cached.headers.has(CACHED_AT) ? (now - storedAt) / 1000 : Infinity;
}

function toStoredCopy(origin: OriginResponse, now: number): Response {
  const headers = clientHeaders(origin.headers);
  const originCacheControl = headers.get("Cache-Control");
  if (originCacheControl !== null) {
    headers.set(ORIGIN_CACHE_CONTROL, originCacheControl);
  }
  headers.set("Cache-Control", `public, max-age=${STALE_SECONDS}`);
  headers.set(CACHED_AT, String(now));
  return new Response(origin.body, { status: origin.status, statusText: origin.statusText, headers });
}

function toClient(origin: OriginResponse): Response {
  return new Response(origin.body, {
    status: origin.status,
    statusText: origin.statusText,
    headers: clientHeaders(origin.headers),
  });
}

function fromCache(cached: Response, stale: boolean): Response {
  const headers = new Headers(cached.headers);
  const originCacheControl = headers.get(ORIGIN_CACHE_CONTROL);
  if (originCacheControl !== null) {
    headers.set("Cache-Control", originCacheControl);
  } else {
    headers.delete("Cache-Control");
  }
  headers.delete(ORIGIN_CACHE_CONTROL);
  headers.delete(CACHED_AT);
  if (stale) {
    headers.set("x-tc-stale", "1");
  }
  return new Response(cached.body, { status: cached.status, statusText: cached.statusText, headers });
}

function clientHeaders(source: Headers): Headers {
  const headers = new Headers(source);
  headers.delete("X-Build-Key");
  headers.delete("Content-Length");
  return headers;
}

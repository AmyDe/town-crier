import { fetchSeo } from "./cache";

export interface Env {
  SWA_ORIGIN: string;
  API_ORIGIN: string;
  INDEXABLE_HOSTS: string;
  SITE_BUILD_KEY: string;
}

export interface EdgeCache {
  match(key: string): Promise<Response | undefined>;
  put(key: string, response: Response): Promise<void>;
}

export interface Deps {
  fetch: (input: string, init?: RequestInit) => Promise<Response>;
  cache: EdgeCache;
  now: () => number;
  waitUntil: (promise: Promise<unknown>) => void;
  originTimeoutMs?: number;
}

export function isSeoPath(pathname: string): boolean {
  return pathname === "/planning" || pathname.startsWith("/planning/") || pathname === "/sitemap.xml";
}

export function isIndexableHost(hostname: string, indexableHosts: string): boolean {
  return indexableHosts
    .split(",")
    .map((h) => h.trim().toLowerCase())
    .filter((h) => h !== "")
    .includes(hostname.toLowerCase());
}

/**
 * Routes a request: SEO paths to the Go API through the edge cache, everything else
 * to the Static Web App. Adds `X-Robots-Tag: noindex, nofollow` on non-indexable hosts.
 */
export async function handle(request: Request, env: Env, deps: Deps): Promise<Response> {
  const url = new URL(request.url);
  const response = isSeoPath(url.pathname)
    ? await handleSeo(request, url, env, deps)
    : await passToSwa(request, url, env, deps);
  if (!isIndexableHost(url.hostname, env.INDEXABLE_HOSTS)) {
    response.headers.set("X-Robots-Tag", "noindex, nofollow");
  }
  return response;
}

async function handleSeo(request: Request, url: URL, env: Env, deps: Deps): Promise<Response> {
  if (request.method !== "GET" && request.method !== "HEAD") {
    return new Response("Method Not Allowed", { status: 405, headers: { Allow: "GET, HEAD" } });
  }
  const response = await fetchSeo(url, env, deps);
  if (request.method === "HEAD") {
    return new Response(null, { status: response.status, headers: response.headers });
  }
  return response;
}

async function passToSwa(request: Request, url: URL, env: Env, deps: Deps): Promise<Response> {
  const swaOrigin = new URL(env.SWA_ORIGIN);
  const hasBody = request.method !== "GET" && request.method !== "HEAD";
  const upstream = await deps.fetch(swaOrigin.origin + url.pathname + url.search, {
    method: request.method,
    headers: request.headers,
    body: hasBody ? request.body : null,
    redirect: "manual",
  });
  const response = new Response(upstream.body, upstream);
  const location = response.headers.get("Location");
  if (location !== null) {
    const target = new URL(location, swaOrigin);
    if (target.host === swaOrigin.host) {
      target.protocol = url.protocol;
      target.host = url.host;
      response.headers.set("Location", target.toString());
    }
  }
  return response;
}

import { describe, expect, it } from "vitest";
import { handle } from "../src/route";
import { env, FakeFetch, makeDeps } from "./fakes";

const PAGE = "https://towncrierapp.uk/planning/croydon";

function page(body: string, status = 200): Response {
  return new Response(body, {
    status,
    headers: { "Content-Type": "text/html; charset=utf-8", "Cache-Control": "public, max-age=300" },
  });
}

async function primeCache(body = "cached page") {
  const fetch = new FakeFetch(async () => page(body));
  const harness = makeDeps({ fetch });
  await handle(new Request(PAGE), env, harness.deps);
  await harness.ctx.settle();
  fetch.calls = [];
  return harness;
}

describe("edge cache", () => {
  it("stores 200 responses with the edge max-age and cached-at stamp", async () => {
    const { cache, clock } = await primeCache();

    const stored = cache.store.get(PAGE)!;
    expect(stored.headers.get("Cache-Control")).toBe("public, max-age=604800");
    expect(stored.headers.get("x-tc-cached-at")).toBe(String(clock.nowMs));
  });

  it("serves fresh cache without origin call", async () => {
    const { fetch, clock, deps } = await primeCache();
    clock.advanceSeconds(3_599);

    const response = await handle(new Request(PAGE), env, deps);

    expect(fetch.calls).toHaveLength(0);
    expect(await response.text()).toBe("cached page");
    expect(response.headers.get("Cache-Control")).toBe("public, max-age=300");
    expect(response.headers.get("x-tc-cached-at")).toBeNull();
    expect(response.headers.get("x-tc-stale")).toBeNull();
  });

  it("refetches origin once the copy is an hour old", async () => {
    const { fetch, clock, ctx, cache, deps } = await primeCache();
    clock.advanceSeconds(3_600);
    fetch.respondWith(async () => page("new page"));

    const response = await handle(new Request(PAGE), env, deps);
    await ctx.settle();

    expect(fetch.calls).toHaveLength(1);
    expect(await response.text()).toBe("new page");
    expect(await cache.store.get(PAGE)!.clone().text()).toBe("new page");
  });

  it("stores 301 and 404 but not other statuses", async () => {
    const fetch = new FakeFetch(async (url) => {
      if (url.endsWith("/wrexham/wrexham")) {
        return new Response(null, { status: 301, headers: { Location: "https://towncrierapp.uk/planning/wrexham" } });
      }
      if (url.endsWith("/missing")) return new Response("nope", { status: 404 });
      return new Response("gone", { status: 410 });
    });
    const { cache, ctx, deps } = makeDeps({ fetch });

    const redirect = await handle(new Request("https://towncrierapp.uk/planning/wrexham/wrexham"), env, deps);
    await handle(new Request("https://towncrierapp.uk/planning/missing"), env, deps);
    await handle(new Request("https://towncrierapp.uk/planning/gone"), env, deps);
    await ctx.settle();

    expect(redirect.status).toBe(301);
    expect(redirect.headers.get("Location")).toBe("https://towncrierapp.uk/planning/wrexham");
    expect(cache.puts).toEqual([
      "https://towncrierapp.uk/planning/wrexham/wrexham",
      "https://towncrierapp.uk/planning/missing",
    ]);
  });

  it("serves stale copy on origin 5xx", async () => {
    const { fetch, clock, deps } = await primeCache();
    clock.advanceSeconds(6 * 86_400);
    fetch.respondWith(async () => new Response("boom", { status: 502 }));

    const response = await handle(new Request(PAGE), env, deps);

    expect(fetch.calls).toHaveLength(1);
    expect(response.status).toBe(200);
    expect(response.headers.get("x-tc-stale")).toBe("1");
    expect(await response.text()).toBe("cached page");
  });

  it("serves stale copy on origin timeout", async () => {
    const fetch = new FakeFetch(async () => page("cached page"));
    const harness = makeDeps({ fetch, originTimeoutMs: 20 });
    await handle(new Request(PAGE), env, harness.deps);
    await harness.ctx.settle();
    harness.clock.advanceSeconds(7_200);
    fetch.respondWith(
      (_url, init) =>
        new Promise((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")));
        }),
    );

    const response = await handle(new Request(PAGE), env, harness.deps);

    expect(response.status).toBe(200);
    expect(response.headers.get("x-tc-stale")).toBe("1");
    expect(await response.text()).toBe("cached page");
  });

  it("serves stale copy on network error", async () => {
    const { fetch, clock, deps } = await primeCache();
    clock.advanceSeconds(7_200);
    fetch.respondWith(async () => {
      throw new TypeError("network down");
    });

    const response = await handle(new Request(PAGE), env, deps);

    expect(response.headers.get("x-tc-stale")).toBe("1");
  });

  it("returns 503 when origin fails and no copy", async () => {
    const fetch = new FakeFetch(async () => new Response("boom", { status: 500 }));
    const { cache, ctx, deps } = makeDeps({ fetch });

    const response = await handle(new Request("https://preview.towncrierapp.uk/planning"), env, deps);
    await ctx.settle();

    expect(response.status).toBe(503);
    expect(response.headers.get("Retry-After")).toBe("60");
    expect(response.headers.get("X-Robots-Tag")).toBe("noindex, nofollow");
    expect(cache.puts).toHaveLength(0);
  });

  it("returns 503 when the only copy is older than seven days", async () => {
    const { fetch, clock, deps } = await primeCache();
    clock.advanceSeconds(7 * 86_400 + 1);
    fetch.respondWith(async () => new Response("boom", { status: 503 }));

    const response = await handle(new Request(PAGE), env, deps);

    expect(response.status).toBe(503);
    expect(response.headers.get("Retry-After")).toBe("60");
  });

  it("answers HEAD from the GET cache entry without a body", async () => {
    const { fetch, deps } = await primeCache();

    const response = await handle(new Request(PAGE, { method: "HEAD" }), env, deps);

    expect(fetch.calls).toHaveLength(0);
    expect(response.status).toBe(200);
    expect(response.body).toBeNull();
    expect(response.headers.get("Content-Type")).toBe("text/html; charset=utf-8");
  });

  it("does not cache SWA paths", async () => {
    const { fetch, cache, ctx, deps } = makeDeps();

    await handle(new Request("https://towncrierapp.uk/"), env, deps);
    await handle(new Request("https://towncrierapp.uk/"), env, deps);
    await ctx.settle();

    expect(fetch.calls).toHaveLength(2);
    expect(cache.puts).toHaveLength(0);
    expect(cache.store.size).toBe(0);
  });
});

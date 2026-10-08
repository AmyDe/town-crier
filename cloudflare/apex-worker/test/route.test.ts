import { describe, expect, it } from "vitest";
import { handle, isIndexableHost, isSeoPath } from "../src/route";
import { env, FakeFetch, headerOf, makeDeps } from "./fakes";

describe("routing", () => {
  it("routes non-SEO paths to SWA origin", async () => {
    const { fetch, deps } = makeDeps();
    const request = new Request("https://towncrierapp.uk/watch-zones?x=1", {
      method: "POST",
      headers: { "Content-Type": "application/json", Cookie: "a=b" },
      body: '{"k":1}',
    });

    const response = await handle(request, env, deps);

    expect(response.status).toBe(200);
    expect(fetch.calls).toHaveLength(1);
    const call = fetch.calls[0]!;
    expect(call.url).toBe("https://swa-default.azurestaticapps.net/watch-zones?x=1");
    expect(call.init?.method).toBe("POST");
    expect(call.init?.redirect).toBe("manual");
    expect(headerOf(call.init, "Cookie")).toBe("a=b");
    expect(headerOf(call.init, "X-Build-Key")).toBeNull();
    expect(await new Response(call.init?.body).text()).toBe('{"k":1}');
  });

  it("rewrites SWA Location host to request host", async () => {
    const fetch = new FakeFetch(
      async () =>
        new Response(null, {
          status: 302,
          headers: { Location: "https://swa-default.azurestaticapps.net/login?next=%2F" },
        }),
    );
    const { deps } = makeDeps({ fetch });

    const response = await handle(new Request("https://preview.towncrierapp.uk/account"), env, deps);

    expect(response.status).toBe(302);
    expect(response.headers.get("Location")).toBe("https://preview.towncrierapp.uk/login?next=%2F");
  });

  it("leaves a foreign Location host untouched", async () => {
    const fetch = new FakeFetch(
      async () => new Response(null, { status: 302, headers: { Location: "https://auth.example.com/authorize" } }),
    );
    const { deps } = makeDeps({ fetch });

    const response = await handle(new Request("https://towncrierapp.uk/login"), env, deps);

    expect(response.headers.get("Location")).toBe("https://auth.example.com/authorize");
  });

  it("routes /planning, /planning/*, /sitemap.xml to API with X-Build-Key", async () => {
    const { fetch, deps } = makeDeps();

    for (const path of ["/planning", "/planning/croydon/", "/planning/croydon/purley?page=2", "/sitemap.xml"]) {
      await handle(new Request(`https://towncrierapp.uk${path}`), env, deps);
    }

    expect(fetch.calls.map((c) => c.url)).toEqual([
      "https://api.example.test/planning",
      "https://api.example.test/planning/croydon/",
      "https://api.example.test/planning/croydon/purley?page=2",
      "https://api.example.test/sitemap.xml",
    ]);
    for (const call of fetch.calls) {
      expect(headerOf(call.init, "X-Build-Key")).toBe("build-key-secret");
    }
  });

  it("does not treat /planningfoo as an SEO path", async () => {
    const { fetch, deps } = makeDeps();

    await handle(new Request("https://towncrierapp.uk/planningfoo"), env, deps);

    expect(fetch.calls[0]!.url).toBe("https://swa-default.azurestaticapps.net/planningfoo");
    expect(isSeoPath("/planningfoo")).toBe(false);
    expect(isSeoPath("/sitemap.xml.gz")).toBe(false);
  });

  it("returns 405 for POST on /planning", async () => {
    const { fetch, deps } = makeDeps();

    const response = await handle(new Request("https://towncrierapp.uk/planning", { method: "POST", body: "x" }), env, deps);

    expect(response.status).toBe(405);
    expect(response.headers.get("Allow")).toBe("GET, HEAD");
    expect(fetch.calls).toHaveLength(0);
  });

  it("does not forward the build key back to clients", async () => {
    const fetch = new FakeFetch(async () => new Response("page", { headers: { "X-Build-Key": "echoed" } }));
    const { deps } = makeDeps({ fetch });

    const response = await handle(new Request("https://towncrierapp.uk/planning"), env, deps);

    expect(response.headers.get("X-Build-Key")).toBeNull();
  });
});

describe("robots", () => {
  it("adds X-Robots-Tag noindex on non-indexable host", async () => {
    const { deps } = makeDeps();

    const swa = await handle(new Request("https://preview.towncrierapp.uk/"), env, deps);
    const seo = await handle(new Request("https://preview.towncrierapp.uk/planning"), env, deps);
    const notAllowed = await handle(new Request("https://preview.towncrierapp.uk/planning", { method: "DELETE" }), env, deps);

    for (const response of [swa, seo, notAllowed]) {
      expect(response.headers.get("X-Robots-Tag")).toBe("noindex, nofollow");
    }
  });

  it("omits X-Robots-Tag on indexable host", async () => {
    const { deps } = makeDeps();

    const swa = await handle(new Request("https://towncrierapp.uk/"), env, deps);
    const seo = await handle(new Request("https://towncrierapp.uk/planning"), env, deps);

    expect(swa.headers.get("X-Robots-Tag")).toBeNull();
    expect(seo.headers.get("X-Robots-Tag")).toBeNull();
  });

  it("treats an empty INDEXABLE_HOSTS as no indexable host and trims entries", () => {
    expect(isIndexableHost("towncrierapp.uk", "")).toBe(false);
    expect(isIndexableHost("towncrierapp.uk", " www.towncrierapp.uk , towncrierapp.uk ")).toBe(true);
    expect(isIndexableHost("preview.towncrierapp.uk", "towncrierapp.uk")).toBe(false);
  });
});

import { describe, expect, it } from "vitest";
import { errorOf } from "./useGitHub";

const res = (status: number, body: string, url = "http://localhost:5173/api/github/repos") => {
  const r = new Response(body, { status });
  Object.defineProperty(r, "url", { value: url });
  return r;
};

describe("errorOf", () => {
  it("uses the server's own message", async () => {
    expect(await errorOf(res(403, JSON.stringify({ error: "not allowed" })))).toBe("not allowed");
  });
  it("names the failing call when the answer is not JSON", async () => {
    expect(await errorOf(res(404, "404 page not found"))).toBe("Request failed (404) /api/github/repos");
  });
  it("still works when the response has no url", async () => {
    expect(await errorOf(res(502, "bad gateway", ""))).toBe("Request failed (502)");
  });
});

import { afterEach, describe, expect, it, vi } from "vitest";

// GitHub sends the browser back to #/projects/new?...; the app clears the hash on start,
// so the return must be read when the module loads (this was the "nothing happens" bug).
async function load(hash: string) {
  vi.resetModules();
  vi.stubGlobal("window", { location: { hash } });
  return import("./useGitHub");
}

afterEach(() => vi.unstubAllGlobals());

describe("return from GitHub", () => {
  it("is kept from the hash at load, even if the hash is cleared later", async () => {
    const g = await load("#/projects/new?connected=1");
    (window as unknown as { location: { hash: string } }).location.hash = "";
    expect(g.peekGitHubReturn()?.get("connected")).toBe("1");
  });
  it("carries an error code", async () => {
    const g = await load("#/projects/new?error=no_installation");
    expect(g.peekGitHubReturn()?.get("error")).toBe("no_installation");
  });
  it("is empty on any other page and after it was used", async () => {
    expect((await load("#/p/abc")).peekGitHubReturn()).toBeNull();
    const g = await load("#/projects/new");
    expect(g.peekGitHubReturn()).not.toBeNull();
    g.clearGitHubReturn();
    expect(g.peekGitHubReturn()).toBeNull();
  });
});

import { describe, expect, it } from "vitest";
import { EXAMPLE_SCHEMA } from "./helpContent";
import { suggestName } from "./ModulesPanel";
import { sortUsers } from "./PeoplePanel";
import { filterProjects, needsAttention, type ProjectOverview } from "./ProjectsPanel";

describe("sign-up list order", () => {
  it("puts people waiting for approval first, then active, suspended and rejected, each by name", () => {
    const u = (name: string, status?: "pending" | "active" | "rejected" | "suspended") => ({
      oid: name, email: "", name, project_count: 0, status,
    });
    const sorted = sortUsers([u("Zed", "rejected"), u("Bo", "active"), u("Al", "pending"), u("Cy", "suspended"), u("Ann", undefined), u("Di", "pending")]);
    expect(sorted.map((x) => x.name)).toEqual(["Al", "Di", "Ann", "Bo", "Cy", "Zed"]);
  });
});

describe("module import helpers", () => {
  it("suggests the catalog name from the schema's module source", () => {
    expect(suggestName(JSON.stringify({ "x-tf-module": { source: "modules/Storage-Account" } }))).toBe("storage-account");
    expect(suggestName("not json", "fallback")).toBe("fallback");
    expect(suggestName("{}", "")).toBe("");
  });
  it("the built-in example names itself and has what a schema needs", () => {
    expect(suggestName(JSON.stringify(EXAMPLE_SCHEMA))).toBe("my-resource-group");
    expect(EXAMPLE_SCHEMA.type).toBe("object");
    expect(EXAMPLE_SCHEMA.required).toContain("name");
  });
});

describe("project overview filters", () => {
  const p = (over: Partial<ProjectOverview>): ProjectOverview => ({
    id: over.name ?? "x", name: "x", source_type: "github", iac_path: "iac", created_at: "", members: 1, pending_changesets: 0, ...over,
  });
  const list = [
    p({ name: "Platform", repository: { full_name: "alice/infra", folder: "iac", default_branch: "main", second_reviewer: false, connection_status: "active" }, owner: { oid: "u1", name: "Alice", email: "a@x.test" } }),
    p({ name: "Broken", repository: { full_name: "bob/net", folder: "", default_branch: "main", second_reviewer: false, connection_status: "needs_attention" }, pending_changesets: 2 }),
    p({ name: "Sample", source_type: "local" }),
  ];
  it("finds what needs an admin", () => {
    expect(list.filter(needsAttention).map((x) => x.name)).toEqual(["Broken"]);
    expect(filterProjects(list, "attention", "").map((x) => x.name)).toEqual(["Broken"]);
    expect(filterProjects(list, "review", "").map((x) => x.name)).toEqual(["Broken"]);
    expect(filterProjects(list, "local", "").map((x) => x.name)).toEqual(["Sample"]);
    expect(filterProjects(list, "github", "").length).toBe(2);
  });
  it("searches name, repository and owner", () => {
    expect(filterProjects(list, "all", "alice").map((x) => x.name)).toEqual(["Platform"]);
    expect(filterProjects(list, "all", "bob/net").map((x) => x.name)).toEqual(["Broken"]);
  });
});

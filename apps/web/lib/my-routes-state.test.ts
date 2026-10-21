import assert from "node:assert/strict";
import test from "node:test";
import { appendRoutePage, defaultRouteFilters, routeListQuery, readRouteListState, saveRouteListState, type RouteListResult } from "./my-routes-state.ts";

test("route filters use literal additive booleans and restart without a cursor", () => {
  assert.equal(routeListQuery(defaultRouteFilters), "view=active&owned=true&joined=true&left=false&limit=20");
  for (let mask = 0; mask < 8; mask++) {
    const filters = { view: "archived" as const, owned: !!(mask & 1), joined: !!(mask & 2), left: !!(mask & 4) };
    const query = new URLSearchParams(routeListQuery(filters, "signed+cursor"));
    assert.equal(query.get("owned"), String(filters.owned));
    assert.equal(query.get("joined"), String(filters.joined));
    assert.equal(query.get("left"), String(filters.left));
    assert.equal(query.get("cursor"), "signed+cursor");
    assert.equal(new URLSearchParams(routeListQuery(filters)).has("cursor"), false);
  }
});

test("appending a changed page deduplicates by route ID and uses full server totals", () => {
  const first = { items: [{ id: "a", name: "old" }, { id: "b", name: "second" }], counts: { active: 42, archived: 3 }, nextCursor: "first" } as RouteListResult;
  const next = { items: [{ id: "a", name: "updated" }, { id: "c", name: "third" }], counts: { active: 43, archived: 4 }, nextCursor: null } as RouteListResult;
  const result = appendRoutePage(first, next);
  assert.deepEqual(result.items.map(item => [item.id, item.name]), [["a", "updated"], ["b", "second"], ["c", "third"]]);
  assert.equal(result.counts.active, 43);
  assert.equal(result.nextCursor, null);
});

test("retained summaries and position cannot cross accounts", () => {
  const storage = new Map<string, string>();
  Object.defineProperty(globalThis, "sessionStorage", { configurable: true, value: { getItem: (key: string) => storage.get(key), setItem: (key: string, value: string) => storage.set(key, value), removeItem: (key: string) => storage.delete(key) } });
  saveRouteListState({ userId: "first", filters: defaultRouteFilters, result: null, scroll: 400 });
  assert.equal(readRouteListState("first")?.scroll, 400);
  assert.equal(readRouteListState("second"), null);
  assert.equal(readRouteListState("first"), null);
});

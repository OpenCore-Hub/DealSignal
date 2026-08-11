/**
 * Deal Radar contract against a real API (no Vite / MSW).
 *
 * Seeds an access-request → diligence_gate via production SyncWorkspace + Compile,
 * then asserts evidence + PATCH done clear + missing PATCH 404.
 *
 *   REAL_API_BASE_URL=http://localhost:8090 pnpm test:e2e:radar-real
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedRadarDiligenceGate,
  waitForRadarProduct,
  fetchRadar,
  fetchRadarEvidence,
  patchRadarItem,
} from "./real-helpers";

test.describe.configure({ mode: "serial" });

test.describe("Deal Radar contract (real backend)", () => {
  let workspaceSlug = "";
  let gateItemId = "";

  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
    await seedRadarDiligenceGate(workspaceSlug);
    const item = await waitForRadarProduct(workspaceSlug, "diligence_gate");
    gateItemId = item.id;
  });

  test("GET /radar surfaces diligence_gate after access-request sync", async () => {
    const feed = await fetchRadar(workspaceSlug);
    expect(feed.items.some((i) => i.id === gateItemId)).toBe(true);
    expect(feed.counts?.all ?? feed.items.length).toBeGreaterThanOrEqual(1);
    const gate = feed.items.find((i) => i.id === gateItemId);
    expect(gate?.product).toBe("diligence_gate");
  });

  test("GET evidence returns pack for the radar item", async () => {
    const { res, body } = await fetchRadarEvidence(workspaceSlug, gateItemId);
    expect(res.status).toBe(200);
    expect(body).toMatchObject({
      itemId: gateItemId,
      product: "diligence_gate",
    });
    // Happy-path seed should not mark ranking/evidence as degraded.
    const degraded = (body as { degradedSections?: string[] }).degradedSections;
    expect(degraded ?? []).toEqual([]);
  });

  test("PATCH done clears item and increments clearedToday", async () => {
    const before = await fetchRadar(workspaceSlug);
    const clearedBefore = before.clearedToday ?? 0;

    const { res } = await patchRadarItem(workspaceSlug, gateItemId, {
      status: "done",
      outcome: "acted",
    });
    expect(res.status).toBe(200);

    // SyncWorkspace must not reopen host "done" while the access request is
    // still pending (inbox remains; radar card stays cleared).
    const after = await fetchRadar(workspaceSlug);
    expect(after.items.some((i) => i.id === gateItemId)).toBe(false);
    expect(after.clearedToday).toBeGreaterThanOrEqual(clearedBefore + 1);
  });

  test("PATCH missing item returns not_found 404", async () => {
    const missing = "00000000-0000-4000-8000-000000000099";
    const { res, body } = await patchRadarItem(workspaceSlug, missing, {
      status: "done",
      outcome: "acted",
    });
    expect(res.status).toBe(404);
    expect(body.code).toBe("not_found");

    const evidence = await fetchRadarEvidence(workspaceSlug, missing);
    expect(evidence.res.status).toBe(404);
    expect((evidence.body as { code?: string }).code).toBe("not_found");
  });
});

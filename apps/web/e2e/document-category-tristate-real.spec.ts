/**
 * Document category tri-state model — API contract on real backend.
 * general ↔ deal_room via room membership; agreement blocked from rooms.
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedAgreementDocument,
  seedDealRoom,
  fetchDocument,
  listDocumentsByCategory,
  attachDocumentToRoom,
  detachDocumentFromRoom,
  apiFetch,
  uploadDocumentRaw,
} from "./real-helpers";

let workspaceSlug: string;

test.describe("Document category tri-state (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
  });

  test("library upload is general and lists under category=general only", async () => {
    const doc = await seedDocument(workspaceSlug);
    const fetched = await fetchDocument(workspaceSlug, doc.id);
    expect(fetched.category).toBe("general");

    const general = await listDocumentsByCategory(workspaceSlug, "general");
    expect(general.some((d) => d.id === doc.id)).toBe(true);

    const dealRoom = await listDocumentsByCategory(workspaceSlug, "deal_room");
    expect(dealRoom.some((d) => d.id === doc.id)).toBe(false);
  });

  test("adding a general doc to a deal room promotes category to deal_room", async () => {
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, { name: "Category Promote Room", templateType: "seed" });

    const attachRes = await attachDocumentToRoom(workspaceSlug, room.id, doc.id);
    expect([200, 201]).toContain(attachRes.status);

    const fetched = await fetchDocument(workspaceSlug, doc.id);
    expect(fetched.category).toBe("deal_room");

    const library = await listDocumentsByCategory(workspaceSlug, "general");
    expect(library.some((d) => d.id === doc.id)).toBe(false);
  });

  test("removing the last room membership demotes deal_room back to general", async () => {
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, { name: "Category Demote Room", templateType: "seed" });

    const attachRes = await attachDocumentToRoom(workspaceSlug, room.id, doc.id);
    expect([200, 201]).toContain(attachRes.status);

    const detachRes = await detachDocumentFromRoom(workspaceSlug, room.id, doc.id);
    expect([200, 204]).toContain(detachRes.status);

    const fetched = await fetchDocument(workspaceSlug, doc.id);
    expect(fetched.category).toBe("general");
  });

  test("agreement documents cannot be added to a deal room", async () => {
    const agreement = await seedAgreementDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, { name: "Agreement Block Room", templateType: "seed" });

    const res = await attachDocumentToRoom(workspaceSlug, room.id, agreement.id);
    expect(res.status).toBe(400);
    const body = (await res.json()) as { code?: string };
    expect(body.code).toBe("agreement_not_allowed_in_deal_room");

    const fetched = await fetchDocument(workspaceSlug, agreement.id);
    expect(fetched.category).toBe("agreement");
  });

  test("deal_room category cannot be manually changed to agreement", async () => {
    const doc = await seedDocument(workspaceSlug);
    const room = await seedDealRoom(workspaceSlug, { name: "Immutable Category Room", templateType: "seed" });
    await attachDocumentToRoom(workspaceSlug, room.id, doc.id);

    const patchRes = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${doc.id}/category`, {
      method: "PATCH",
      body: JSON.stringify({ category: "agreement" }),
    });
    expect(patchRes.status).toBe(409);
    const body = (await patchRes.json()) as { code?: string };
    expect(body.code).toBe("category_immutable");
  });

  test("general PDF can be marked agreement via PATCH", async () => {
    const doc = await seedDocument(workspaceSlug);

    const patchRes = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${doc.id}/category`, {
      method: "PATCH",
      body: JSON.stringify({ category: "agreement" }),
    });
    expect(patchRes.ok).toBe(true);

    const fetched = await fetchDocument(workspaceSlug, doc.id);
    expect(fetched.category).toBe("agreement");

    const agreements = await listDocumentsByCategory(workspaceSlug, "agreement");
    expect(agreements.some((d) => d.id === doc.id)).toBe(true);
  });

  test("reject invalid category values on PATCH", async () => {
    const doc = await seedDocument(workspaceSlug);
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${doc.id}/category`, {
      method: "PATCH",
      body: JSON.stringify({ category: "deal_room" }),
    });
    expect(res.status).toBe(400);
  });

  test("reject deal_room category on POST upload", async () => {
    const res = await uploadDocumentRaw(workspaceSlug, { category: "deal_room" });
    expect(res.status).toBe(400);
    const body = (await res.json()) as { code?: string };
    expect(body.code).toBe("category_deal_room_via_api");
  });

  test("multi-room attach keeps deal_room until last detach", async () => {
    const doc = await seedDocument(workspaceSlug);
    const roomA = await seedDealRoom(workspaceSlug, { name: "Multi A", templateType: "seed" });
    const roomB = await seedDealRoom(workspaceSlug, { name: "Multi B", templateType: "seed" });

    expect([200, 201]).toContain((await attachDocumentToRoom(workspaceSlug, roomA.id, doc.id)).status);
    expect((await fetchDocument(workspaceSlug, doc.id)).category).toBe("deal_room");

    expect([200, 201]).toContain((await attachDocumentToRoom(workspaceSlug, roomB.id, doc.id)).status);
    expect((await fetchDocument(workspaceSlug, doc.id)).category).toBe("deal_room");

    expect([200, 204]).toContain((await detachDocumentFromRoom(workspaceSlug, roomA.id, doc.id)).status);
    expect((await fetchDocument(workspaceSlug, doc.id)).category).toBe("deal_room");

    expect([200, 204]).toContain((await detachDocumentFromRoom(workspaceSlug, roomB.id, doc.id)).status);
    expect((await fetchDocument(workspaceSlug, doc.id)).category).toBe("general");
  });

  test("deal room upload path promotes via attach", async () => {
    const room = await seedDealRoom(workspaceSlug, { name: "Upload Promote Room", templateType: "seed" });
    const doc = await seedDocument(workspaceSlug);
    expect(doc.category).toBe("general");

    const attachRes = await attachDocumentToRoom(workspaceSlug, room.id, doc.id);
    expect([200, 201]).toContain(attachRes.status);
    expect((await fetchDocument(workspaceSlug, doc.id)).category).toBe("deal_room");

    const library = await listDocumentsByCategory(workspaceSlug, "general");
    expect(library.some((d) => d.id === doc.id)).toBe(false);
  });
});

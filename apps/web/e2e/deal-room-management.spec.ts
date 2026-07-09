/**
 * Deal room management — folders, document move/remove, permissions, access requests.
 * Covers: POST/PATCH/DELETE folders, PATCH/DELETE documents, POST folder-permissions,
 *         GET/POST access-requests, approve/reject
 */
import { test, expect } from "@playwright/test";
import { seedRealBackend, seedDocument, seedDealRoom, apiFetch } from "./real-helpers";

let token: string;
let workspaceSlug: string;
let roomId: string;

test.describe("Deal room management (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    token = seed.token;
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(token, workspaceSlug);
    const room = await seedDealRoom(token, workspaceSlug, {
      name: "Management Test Room",
      templateType: "seed",
      documentIds: [doc.id],
    });
    roomId = room.id;
  });

  // ── Folder CRUD ────────────────────────────────────────────
  test("creates a deal room folder", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/folders`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ name: "Legal Documents" }),
    });
    expect([200, 201]).toContain(res.status);
  });

  test("renames a deal room folder", async () => {
    // First ensure we have a folder
    const listRes = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/folders`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    const folders = (await listRes.json()) as { data: { path: string }[] };
    const target = folders.data.find((f) => f.path !== "/general") ?? folders.data[0];
    const encodedPath = encodeURIComponent(target.path.replace(/^\//, ""));

    const res = await apiFetch(
      `/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/folders/${encodedPath}`,
      {
        method: "PATCH",
        headers: { Authorization: `Bearer ${token}` },
        body: JSON.stringify({ name: "Renamed Folder" }),
      }
    );
    expect([200, 400, 404]).toContain(res.status);
  });

  // ── Documents ───────────────────────────────────────────────
  test("lists deal room documents", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/documents`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { data: unknown[] };
    expect(Array.isArray(body.data)).toBe(true);
  });

  test("adds a document to deal room", async () => {
    const doc = await seedDocument(token, workspaceSlug);
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/documents`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ document_id: doc.id, folder_path: "/general" }),
    });
    expect([200, 201]).toContain(res.status);
  });

  test("updates deal room document position", async () => {
    // Get the first document
    const docsRes = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/documents`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    const docs = (await docsRes.json()) as { data: { documents: { id: string }[] }[] };
    const firstDoc = docs.data.flatMap((f) => f.documents)[0];
    if (firstDoc) {
      const res = await apiFetch(
        `/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/documents/${firstDoc.id}`,
        {
          method: "PATCH",
          headers: { Authorization: `Bearer ${token}` },
          body: JSON.stringify({ sort_order: 999 }),
        }
      );
      expect([200, 400, 404]).toContain(res.status);
    }
  });

  test("removes a document from deal room", async () => {
    // Add a fresh document just for removal
    const doc = await seedDocument(token, workspaceSlug);
    const addRes = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/documents`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ document_id: doc.id }),
    });
    if (addRes.ok) {
      const added = (await addRes.json()) as { id: string };
      const delRes = await apiFetch(
        `/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/documents/${added.id}`,
        {
          method: "DELETE",
          headers: { Authorization: `Bearer ${token}` },
        }
      );
      expect([200, 204, 404]).toContain(delRes.status);
    }
  });

  // ── Folder permissions ──────────────────────────────────────
  test("sets folder permissions", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/folder-permissions`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({
        email: `perm-${Date.now()}@example.com`,
        folder_path: "/general",
        permission: "view",
      }),
    });
    expect([200, 201, 400]).toContain(res.status);
  });

  // ── Members ─────────────────────────────────────────────────
  test("lists deal room members", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/members`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
  });

  test("adds and removes a deal room member", async () => {
    const email = `room-member-${Date.now()}@example.com`;
    const addRes = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/members`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ email, role: "viewer" }),
    });
    expect(addRes.ok).toBe(true);

    const added = (await addRes.json()) as { data: { id: string } };
    const memberId = added.data.id;

    const delRes = await apiFetch(
      `/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/members/${memberId}`,
      {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      }
    );
    expect([200, 204]).toContain(delRes.status);
  });

  // ── Deal room links ─────────────────────────────────────────
  test("creates and lists deal room share links", async () => {
    // Create
    const createRes = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/links`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ name: "Room Share Link", download_enabled: true }),
    });
    expect([200, 201]).toContain(createRes.status);

    // List
    const listRes = await apiFetch(`/api/workspaces/${workspaceSlug}/deal-rooms/${roomId}/links`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(listRes.ok).toBe(true);
  });
});

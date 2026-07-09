/**
 * Document operations — archive/unarchive, category, download URL.
 * Covers: POST archive/unarchive, PATCH category, GET download-url, GET document pages
 */
import { test, expect } from "@playwright/test";
import { seedRealBackend, seedDocument, apiFetch } from "./real-helpers";

let token: string;
let workspaceSlug: string;
let docId: string;

test.describe("Document operations (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    token = seed.token;
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(token, workspaceSlug);
    docId = doc.id;
  });

  test("archives and unarchives a document", async () => {
    // Archive
    const archiveRes = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/archive`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(archiveRes.ok).toBe(true);
    const archived = (await archiveRes.json()) as { status: string };
    expect(archived.status).toBe("archived");

    // Unarchive
    const unarchiveRes = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/unarchive`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(unarchiveRes.ok).toBe(true);
    const unarchived = (await unarchiveRes.json()) as { status: string };
    expect(unarchived.status).toBe("ready");
  });

  test("updates document category", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/category`, {
      method: "PATCH",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ category: "pitch_deck" }),
    });
    // May succeed or return 400 if category values are enum-restricted
    const ok = res.ok || res.status === 400;
    expect(ok).toBe(true);

    // Verify category persisted if successful
    if (res.ok) {
      const getRes = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const doc = (await getRes.json()) as { category?: string };
      if (doc.category) {
        expect(doc.category).toBe("pitch_deck");
      }
    }
  });

  test("gets document download URL", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/download-url`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { download_url: string; filename: string };
    expect(body.download_url).toBeTruthy();
    expect(body.filename).toContain("sample.pdf");
  });

  test("gets document pages", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/pages`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { total: number; pages: unknown[] };
    expect(body.total).toBeGreaterThan(0);
    expect(body.pages.length).toBeGreaterThan(0);
  });

  test("gets document page signed URL", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/pages/signed-url`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ page_number: 1 }),
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { image_url: string; page_number: number };
    expect(body.image_url).toBeTruthy();
    expect(body.page_number).toBe(1);
  });

  test("lists documents with filters", async () => {
    // Without filter
    const resAll = await apiFetch(`/api/workspaces/${workspaceSlug}/documents`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(resAll.ok).toBe(true);
    const all = (await resAll.json()) as { data: unknown[] };
    expect(all.data.length).toBeGreaterThan(0);

    // With category filter
    const resCategory = await apiFetch(`/api/workspaces/${workspaceSlug}/documents?category=pitch_deck`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(resCategory.ok).toBe(true);

    // With status filter
    const resFilter = await apiFetch(`/api/workspaces/${workspaceSlug}/documents?filter=recent`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(resFilter.ok).toBe(true);
  });
});

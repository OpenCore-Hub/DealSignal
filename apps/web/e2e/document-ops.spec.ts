/**
 * Document operations — archive/unarchive, category tri-state, download URL.
 * Trust gate: archive revokes visitor Access / signed-url for that document.
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedLink,
  fetchDocument,
  apiFetch,
} from "./real-helpers";

let workspaceSlug: string;
let docId: string;

test.describe("Document operations (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
    const doc = await seedDocument(workspaceSlug);
    docId = doc.id;
  });

  test("archives and unarchives a document", async () => {
    const archiveRes = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/archive`, {
      method: "POST",
    });
    expect(archiveRes.ok).toBe(true);
    const archived = (await archiveRes.json()) as { status: string };
    expect(archived.status).toBe("archived");

    const unarchiveRes = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/unarchive`, {
      method: "POST",
    });
    expect(unarchiveRes.ok).toBe(true);
    const unarchived = (await unarchiveRes.json()) as { status: string };
    expect(unarchived.status).toBe("ready");
  });

  test("archive revokes visitor document access on public link", async () => {
    const doc = await seedDocument(workspaceSlug);
    const link = await seedLink(workspaceSlug, doc.id, {
      name: "Archive revoke gate",
      permissionType: "public",
    });

    const beforeAccess = await apiFetch(`/api/v1/public/links/${link.publicToken}`, {
      method: "POST",
      body: JSON.stringify({}),
    });
    expect(beforeAccess.ok).toBe(true);
    const beforeBody = (await beforeAccess.json()) as {
      documents?: { id: string }[];
      document?: { id: string };
    };
    const authorizedIds = new Set(
      (beforeBody.documents ?? [])
        .map((d) => d.id)
        .concat(beforeBody.document?.id ? [beforeBody.document.id] : []),
    );
    expect(authorizedIds.has(doc.id)).toBe(true);

    const beforeSigned = await apiFetch(
      `/api/v1/public/documents/${doc.id}/pages/signed-url?token=${link.publicToken}&page_number=1`,
    );
    expect(beforeSigned.ok).toBe(true);

    const archiveRes = await apiFetch(
      `/api/workspaces/${workspaceSlug}/documents/${doc.id}/archive`,
      { method: "POST" },
    );
    expect(archiveRes.ok).toBe(true);

    const afterAccess = await apiFetch(`/api/v1/public/links/${link.publicToken}`, {
      method: "POST",
      body: JSON.stringify({}),
    });
    expect(afterAccess.ok).toBe(true);
    const afterBody = (await afterAccess.json()) as {
      documents?: { id: string }[];
      document?: { id: string };
    };
    const afterIds = new Set(
      (afterBody.documents ?? [])
        .map((d) => d.id)
        .concat(afterBody.document?.id ? [afterBody.document.id] : []),
    );
    expect(afterIds.has(doc.id)).toBe(false);

    const afterSigned = await apiFetch(
      `/api/v1/public/documents/${doc.id}/pages/signed-url?token=${link.publicToken}&page_number=1`,
    );
    expect(afterSigned.status).toBe(403);
  });

  test("rejects invalid category on PATCH", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/category`, {
      method: "PATCH",
      body: JSON.stringify({ category: "deal_room" }),
    });
    expect(res.status).toBe(400);
  });

  test("marks general PDF as agreement and back to general", async () => {
    const toAgreement = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/category`, {
      method: "PATCH",
      body: JSON.stringify({ category: "agreement" }),
    });
    expect(toAgreement.ok).toBe(true);
    let fetched = await fetchDocument(workspaceSlug, docId);
    expect(fetched.category).toBe("agreement");

    const toGeneral = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/category`, {
      method: "PATCH",
      body: JSON.stringify({ category: "general" }),
    });
    expect(toGeneral.ok).toBe(true);
    fetched = await fetchDocument(workspaceSlug, docId);
    expect(fetched.category).toBe("general");
  });

  test("gets document download URL", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/download-url`, {});
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { download_url: string; filename: string };
    expect(body.download_url).toBeTruthy();
    expect(body.filename).toMatch(/\.pdf$/i);
  });

  test("gets document pages", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/pages`, {});
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { total: number; pages: unknown[] };
    expect(body.total).toBeGreaterThan(0);
    expect(body.pages.length).toBeGreaterThan(0);
  });

  test("gets document page signed URL", async () => {
    const res = await apiFetch(`/api/workspaces/${workspaceSlug}/documents/${docId}/pages/signed-url`, {
      method: "POST",
      body: JSON.stringify({ page_number: 1 }),
    });
    expect(res.ok).toBe(true);
    const body = (await res.json()) as { image_url: string; page_number: number };
    expect(body.image_url).toBeTruthy();
    expect(body.page_number).toBe(1);
  });

  test("lists documents with tri-state category filters", async () => {
    const resAll = await apiFetch(`/api/workspaces/${workspaceSlug}/documents`, {});
    expect(resAll.ok).toBe(true);
    const all = (await resAll.json()) as { data: unknown[] };
    expect(all.data.length).toBeGreaterThan(0);

    for (const category of ["general", "agreement", "deal_room"] as const) {
      const res = await apiFetch(
        `/api/workspaces/${workspaceSlug}/documents?category=${category}`,
      );
      expect(res.ok).toBe(true);
      const body = (await res.json()) as { data: { category?: string }[] };
      for (const row of body.data) {
        if (row.category) {
          expect(row.category).toBe(category);
        }
      }
    }

    const resFilter = await apiFetch(`/api/workspaces/${workspaceSlug}/documents?filter=recent`, {});
    expect(resFilter.ok).toBe(true);
  });
});

/**
 * Access-request inbox isolation (real backend API).
 *
 * Proves SQL/API scope boundaries: document vs deal_room pending inboxes never mix.
 * Requires API on REAL_API_BASE_URL (default http://localhost:8080).
 */
import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedDealRoom,
  seedLink,
  apiFetch,
} from "./real-helpers";

test.describe("Access request inbox scope (real backend)", () => {
  test("document and deal-room pending inboxes stay mutually exclusive", async () => {
    const seed = await seedRealBackend();
    const ws = seed.workspaceSlug;
    const doc = await seedDocument(ws);

    const room = await seedDealRoom(ws, {
      name: "Access Inbox Scope Room",
      templateType: "seed",
      documentIds: [doc.id],
    });

    // Document link: allowlist one email so outsiders can request access.
    const docLink = await seedLink(ws, doc.id, {
      name: `Doc Scope Link ${Date.now()}`,
      permissionType: "email",
      requireEmail: true,
      allowedEmails: ["allowed-doc@example.com"],
    });

    const roomLinkRes = await apiFetch(`/api/workspaces/${ws}/deal-rooms/${room.id}/links`, {
      method: "POST",
      body: JSON.stringify({
        name: `Room Scope Link ${Date.now()}`,
        require_email: true,
        allowed_emails: ["allowed-room@example.com"],
        download_enabled: true,
      }),
    });
    expect(roomLinkRes.ok, await roomLinkRes.text()).toBe(true);
    const roomLink = (await roomLinkRes.json()) as {
      id: string;
      shortUrl?: string;
      short_url?: string;
      public_token?: string;
    };
    const roomShort = roomLink.shortUrl || roomLink.short_url || "";
    const roomToken = roomLink.public_token || (roomShort ? roomShort.split("/").pop()! : "");
    expect(roomToken).toBeTruthy();

    const docApplicant = `doc-applicant-${Date.now()}@example.com`;
    const roomApplicant = `room-applicant-${Date.now()}@example.com`;

    const docReq = await apiFetch(`/api/v1/public/links/${docLink.publicToken}/access-requests`, {
      method: "POST",
      body: JSON.stringify({
        email: docApplicant,
        reason: "document inbox scope e2e",
        signer_name: "Doc E2E",
      }),
    });
    expect([200, 201]).toContain(docReq.status);

    const roomReq = await apiFetch(`/api/v1/public/links/${roomToken}/access-requests`, {
      method: "POST",
      body: JSON.stringify({
        email: roomApplicant,
        reason: "deal-room inbox scope e2e",
        signer_name: "Room E2E",
      }),
    });
    expect([200, 201]).toContain(roomReq.status);

    const docInboxRes = await apiFetch(
      `/api/workspaces/${ws}/links/pending-access-requests?scope=document`,
    );
    expect(docInboxRes.ok).toBe(true);
    const docInbox = (await docInboxRes.json()) as {
      data: { email: string; link_id: string }[];
    };
    const docEmails = docInbox.data.map((r) => r.email);
    expect(docEmails).toContain(docApplicant);
    expect(docEmails).not.toContain(roomApplicant);

    const roomInboxRes = await apiFetch(
      `/api/workspaces/${ws}/links/pending-access-requests?scope=deal_room&deal_room_id=${room.id}`,
    );
    expect(roomInboxRes.ok).toBe(true);
    const roomInbox = (await roomInboxRes.json()) as {
      data: { email: string; link_id: string }[];
    };
    const roomEmails = roomInbox.data.map((r) => r.email);
    expect(roomEmails).toContain(roomApplicant);
    expect(roomEmails).not.toContain(docApplicant);

    const missingRoomId = await apiFetch(
      `/api/workspaces/${ws}/links/pending-access-requests?scope=deal_room`,
    );
    expect(missingRoomId.status).toBe(400);

    const badScope = await apiFetch(
      `/api/workspaces/${ws}/links/pending-access-requests?scope=all`,
    );
    expect(badScope.status).toBe(400);

    // Guessed foreign room id must not leak applicants (404 or empty).
    const foreign = await apiFetch(
      `/api/workspaces/${ws}/links/pending-access-requests?scope=deal_room&deal_room_id=00000000-0000-4000-8000-000000000099`,
    );
    expect([404, 200]).toContain(foreign.status);
    if (foreign.status === 200) {
      const body = (await foreign.json()) as { data: unknown[] };
      expect(body.data).toEqual([]);
    }
  });
});

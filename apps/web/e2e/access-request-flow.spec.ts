import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  seedLink,
  apiFetch,
} from "./real-helpers";

let seed: Awaited<ReturnType<typeof seedRealBackend>>;
let shortUrl: string;
let linkId: string;
const visitorEmail = "visitor@example.com";

test.describe("link access request flow (real backend)", () => {
  test.beforeAll(async () => {
    seed = await seedRealBackend();
    const doc = await seedDocument(seed.workspaceSlug);
    const link = await seedLink(seed.workspaceSlug, doc.id, {
      downloadEnabled: true,
      requireEmail: true,
    });
    shortUrl = link.shortUrl;
    linkId = link.id;

    // Restrict the link to a single allowed email so the visitor is denied.
    const rulesRes = await apiFetch(
      `/api/workspaces/${seed.workspaceSlug}/links/${linkId}/access-rules`,
      {
        method: "POST",
        body: JSON.stringify({
          rules: [{ ruleType: "email", value: "allowed@example.com", action: "allow" }],
        }),
      }
    );
    if (!rulesRes.ok) {
      console.log("[rules error]", rulesRes.status, await rulesRes.text());
    }
    expect(rulesRes.ok).toBe(true);
  });

  test("visitor requests access and creator approves", async ({ page }) => {
    // 1. Visitor opens the link and submits a denied email so the request-access CTA appears.
    await page.goto(shortUrl);
    await page.waitForLoadState("networkidle");
    await page.getByLabel(/Email/i).fill(visitorEmail);
    await page.getByRole("button", { name: /Continue/i }).click();

    const requestButton = page.getByRole("button", { name: /Request access/i });
    await expect(requestButton).toBeVisible({ timeout: 10000 });
    await requestButton.click();

    // 2. Fill and submit the access request form.
    await page.locator("#request-email").fill(visitorEmail);
    await page.getByRole("button", { name: /Submit request/i }).click();
    await expect(page.getByText(/Your access request has been submitted/i).first()).toBeVisible({ timeout: 10000 });

    // 3. Creator lists access requests and finds the pending one.
    const listRes = await apiFetch(
      `/api/workspaces/${seed.workspaceSlug}/links/${linkId}/access-requests`,
      { headers: { } }
    );
    expect(listRes.ok).toBe(true);
    const listBody = (await listRes.json()) as {
      data: Array<{ ID: string; Email: string; Status: string }>;
    };
    const request = listBody.data.find((r) => r.Email === visitorEmail);
    expect(request).toBeDefined();
    expect(request!.Status).toBe("pending");

    // 4. Creator approves the request.
    const approveRes = await apiFetch(
      `/api/workspaces/${seed.workspaceSlug}/links/${linkId}/access-requests/${request!.ID}/approve`,
      {
        method: "POST",
      }
    );
    expect(approveRes.ok).toBe(true);
    const approvedBody = (await approveRes.json()) as {
      data: { Status: string; Email: string };
    };
    expect(approvedBody.data.Status).toBe("approved");
    expect(approvedBody.data.Email).toBe(visitorEmail);

    // 5. Approval creates an allow-rule for the visitor email.
    const rulesRes = await apiFetch(
      `/api/workspaces/${seed.workspaceSlug}/links/${linkId}/access-rules`,
      { headers: { } }
    );
    expect(rulesRes.ok).toBe(true);
    const rulesBody = (await rulesRes.json()) as {
      data: Array<{ Value: string; Action: string }>;
    };
    expect(
      rulesBody.data.some(
        (r) => r.Value === visitorEmail && r.Action === "allow"
      )
    ).toBe(true);

    // 6. Approval creates an invitation for the visitor.
    const invRes = await apiFetch(
      `/api/workspaces/${seed.workspaceSlug}/links/${linkId}/invitations`,
      { headers: { } }
    );
    expect(invRes.ok).toBe(true);
    const invBody = (await invRes.json()) as {
      data: Array<{ Email: string; Status: string }>;
    };
    const invitation = invBody.data.find((i) => i.Email === visitorEmail);
    expect(invitation).toBeDefined();
  });
});

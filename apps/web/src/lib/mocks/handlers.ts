import { http, HttpResponse } from "msw";
import type { ActionItem, WorkspaceMember } from "@/types";
import {
  mockAccessLogs,
  mockActionItems,
  mockActivities,
  mockContacts,
  mockDealRooms,
  mockDealRoomTemplates,
  mockDocuments,
  mockHeatAlerts,
  mockLinks,
  mockPageAnalytics,
  mockSignals,
  mockSuggestions,
  mockWorkspaceMembers,
  mockWorkspaces,
  defaultWorkspaceSettings,
  getMockDashboardStats,
  getMockSignalFeed,
} from "./data";

let workspaceSettings = { ...defaultWorkspaceSettings };

let integrationsStatus = {
  slack: false,
  hubspot: false,
  zapier: false,
};

let securitySettings = {
  forceEmailVerification: true,
  watermarkDownloads: false,
  twoFactorEnabled: false,
};

// Snapshot of initial state so E2E tests can reset between cases.
const initialState = {
  workspaces: structuredClone(mockWorkspaces),
  documents: structuredClone(mockDocuments),
  links: structuredClone(mockLinks),
  dealRooms: structuredClone(mockDealRooms),
  members: structuredClone(mockWorkspaceMembers),
  settings: structuredClone(defaultWorkspaceSettings),
  integrations: structuredClone(integrationsStatus),
  security: structuredClone(securitySettings),
};

function resetMockState() {
  mockUsers.clear();
  mockWorkspaces.splice(0, mockWorkspaces.length, ...initialState.workspaces);
  mockDocuments.splice(0, mockDocuments.length, ...initialState.documents);
  mockLinks.splice(0, mockLinks.length, ...initialState.links);
  mockDealRooms.splice(0, mockDealRooms.length, ...initialState.dealRooms);
  mockWorkspaceMembers.splice(0, mockWorkspaceMembers.length, ...initialState.members);
  workspaceSettings = { ...initialState.settings };
  integrationsStatus = { ...initialState.integrations };
  securitySettings = { ...initialState.security };
}

// In-memory auth store for the mock environment.
interface MockUser {
  id: string;
  email: string;
  password: string;
  name: string;
}
const mockUsers = new Map<string, MockUser>();

function generateId(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`;
}

function createTokenResponse(userId: string, email: string) {
  return {
    user: { id: userId, email, name: email.split("@")[0] },
    access_token: `mock_access_${userId}`,
    refresh_token: `mock_refresh_${userId}`,
    expires_in: 900,
  };
}

function validatePassword(password: string): boolean {
  if (password.length < 8) return false;
  let hasUpper = false;
  let hasLower = false;
  let hasDigit = false;
  let hasSpecial = false;
  for (const ch of password) {
    if (/[A-Z]/.test(ch)) hasUpper = true;
    else if (/[a-z]/.test(ch)) hasLower = true;
    else if (/\d/.test(ch)) hasDigit = true;
    else if (/[^\w\s]/.test(ch)) hasSpecial = true;
  }
  return hasUpper && hasLower && hasDigit && hasSpecial;
}

function placeholdImageUrl(width: number, height: number): string {
  return `data:image/svg+xml,${encodeURIComponent(
    `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}"><rect width="100%" height="100%" fill="#e2e8f0"/><text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" fill="#64748b" font-size="24">Page</text></svg>`
  )}`;
}

export const handlers = [
  // Auth
  http.post("*/api/auth/register", async ({ request }) => {
    const body = (await request.json()) as { email?: string; password?: string };
    const email = body.email?.trim().toLowerCase();
    const password = body.password ?? "";
    if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      return HttpResponse.json({ code: "invalid_email", message: "invalid email address" }, { status: 400 });
    }
    if (!validatePassword(password)) {
      return HttpResponse.json(
        { code: "weak_password", message: "password must be at least 8 characters and include uppercase, lowercase, digit and special character" },
        { status: 400 }
      );
    }
    if (Array.from(mockUsers.values()).some((u) => u.email === email)) {
      return HttpResponse.json({ code: "email_conflict", message: "email already registered" }, { status: 409 });
    }
    const id = generateId("u");
    mockUsers.set(id, { id, email, password, name: email.split("@")[0] });
    return HttpResponse.json(createTokenResponse(id, email), { status: 201 });
  }),

  http.post("*/api/auth/login", async ({ request }) => {
    const body = (await request.json()) as { email?: string; password?: string };
    const email = body.email?.trim().toLowerCase();
    const user = Array.from(mockUsers.values()).find((u) => u.email === email);
    if (!user || user.password !== body.password) {
      return HttpResponse.json({ code: "unauthorized", message: "invalid email or password" }, { status: 401 });
    }
    return HttpResponse.json(createTokenResponse(user.id, user.email));
  }),

  http.post("*/api/auth/refresh", async ({ request }) => {
    const body = (await request.json()) as { refresh_token?: string };
    const token = body.refresh_token ?? "";
    const userId = token.replace("mock_refresh_", "");
    const user = mockUsers.get(userId);
    if (!user) {
      return HttpResponse.json({ code: "unauthorized", message: "invalid or expired refresh token" }, { status: 401 });
    }
    return HttpResponse.json({
      access_token: `mock_access_${user.id}`,
      refresh_token: `mock_refresh_${user.id}`,
      expires_in: 900,
    });
  }),

  http.post("*/api/auth/logout", async () => {
    return HttpResponse.json({ code: "ok", message: "logged out" });
  }),

  http.get("*/api/auth/verify-email/:token", () => {
    return HttpResponse.json({ code: "verified", message: "email verified successfully" });
  }),

  // Test-only reset endpoint used by E2E suites to isolate cases.
  http.post("*/__e2e/reset", () => {
    resetMockState();
    return new HttpResponse(null, { status: 204 });
  }),

  // Workspaces
  http.get("*/api/workspaces", () => {
    return HttpResponse.json({ data: mockWorkspaces });
  }),

  http.post("*/api/workspaces", async ({ request }) => {
    const body = (await request.json()) as { name: string; slug: string; brand_color?: string };
    const newWorkspace = {
      id: generateId("ws"),
      name: body.name,
      slug: body.slug,
      logoUrl: "",
      brandColor: body.brand_color ?? "#0055ff",
    };
    mockWorkspaces.push(newWorkspace);
    workspaceSettings = { ...workspaceSettings, name: body.name, slug: body.slug };
    return HttpResponse.json(newWorkspace, { status: 201 });
  }),

  // Dashboard
  http.get("*/api/workspaces/:workspaceSlug/dashboard/stats", () => {
    return HttpResponse.json(getMockDashboardStats());
  }),

  // Documents
  http.get("*/api/workspaces/:workspaceSlug/documents", () => {
    return HttpResponse.json({ data: mockDocuments });
  }),

  http.get("*/api/workspaces/:workspaceSlug/documents/:id", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(doc);
  }),

  http.delete("*/api/workspaces/:workspaceSlug/documents/:id", ({ params }) => {
    const index = mockDocuments.findIndex((d) => d.id === params.id);
    if (index === -1) return new HttpResponse(null, { status: 404 });
    mockDocuments.splice(index, 1);
    return new HttpResponse(null, { status: 204 });
  }),

  http.post("*/api/workspaces/:workspaceSlug/documents", async ({ request }) => {
    const formData = await request.formData();
    const file = formData.get("file") as File | null;
    const title = file?.name ?? "uploaded.pdf";
    const ext = title.split(".").pop()?.toLowerCase() ?? "pdf";
    const fileType = (["pdf", "docx", "pptx", "xlsx"] as const).includes(ext as never) ? (ext as import("@/types").Document["fileType"]) : "pdf";
    const newDoc = {
      id: generateId("doc"),
      title,
      fileName: title,
      fileType,
      fileSize: file?.size ?? 1_000_000,
      pageCount: 10,
      status: "ready" as const,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    mockDocuments.unshift(newDoc);
    return HttpResponse.json(newDoc, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/documents/:id/pages", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    const pages = Array.from({ length: doc.pageCount }, (_, i) => ({
      page_number: i + 1,
      width: 800,
      height: 1000,
    }));
    return HttpResponse.json({ document_id: doc.id, pages, total: pages.length });
  }),

  http.post("*/api/workspaces/:workspaceSlug/documents/:id/pages/signed-url", async ({ params, request }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { page_number?: number };
    const pageNumber = body.page_number ?? 1;
    return HttpResponse.json({
      page_number: pageNumber,
      image_url: placeholdImageUrl(800, 1000),
      expires_at: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      width: 800,
      height: 1000,
    });
  }),

  http.get("*/api/workspaces/:workspaceSlug/documents/:id/download-url", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({
      download_url: placeholdImageUrl(200, 200),
      expires_at: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      filename: doc.fileName,
      content_type: "application/pdf",
    });
  }),

  // Viewer events
  http.post("*/api/workspaces/:workspaceSlug/events", async () => {
    return new HttpResponse(null, { status: 204 });
  }),

  // Links
  http.get("*/api/workspaces/:workspaceSlug/links", ({ request }) => {
    const url = new URL(request.url);
    const documentId = url.searchParams.get("documentId");
    const data = documentId ? mockLinks.filter((l) => l.documentId === documentId) : mockLinks;
    return HttpResponse.json({ data });
  }),

  http.get("*/api/workspaces/:workspaceSlug/links/:id", ({ params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(link);
  }),

  http.patch("*/api/workspaces/:workspaceSlug/links/:id", async ({ request, params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    const patch = (await request.json()) as Partial<typeof link>;
    Object.assign(link, patch);
    return HttpResponse.json(link);
  }),

  http.get("*/api/workspaces/:workspaceSlug/links/:id/access-logs", ({ params }) => {
    return HttpResponse.json({ data: mockAccessLogs.filter((l) => l.linkId === params.id) });
  }),

  http.post("*/api/workspaces/:workspaceSlug/links", async ({ request }) => {
    const body = (await request.json()) as {
      document_id: string;
      name?: string;
      permission_type?: string;
      password?: string;
      expires_at?: string;
      max_access_count?: number;
      download_enabled?: boolean;
      watermark_enabled?: boolean;
    };
    const doc = mockDocuments.find((d) => d.id === body.document_id);
    const newLink = {
      id: generateId("link"),
      documentId: body.document_id,
      documentTitle: doc?.title ?? "Untitled",
      shortUrl: `https://invest.acme.capital/d/${generateId("sh")}`,
      accessCount: 0,
      heatLevel: "cold" as const,
      createdAt: new Date().toISOString(),
      expiresAt: body.expires_at,
      isActive: true,
      avgDurationSeconds: 0,
      permissionType: (body.permission_type === "public" ? "public" : body.permission_type === "email_required" ? "email" : body.permission_type === "whitelist" ? "email" : "public") as "public" | "email" | "password" | "nda",
    };
    mockLinks.unshift(newLink);
    return HttpResponse.json(newLink, { status: 201 });
  }),

  // Contacts
  http.get("*/api/workspaces/:workspaceSlug/contacts", () => {
    return HttpResponse.json({ data: mockContacts });
  }),

  http.get("*/api/workspaces/:workspaceSlug/contacts/:id", ({ params }) => {
    const contact = mockContacts.find((c) => c.id === params.id);
    if (!contact) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(contact);
  }),

  http.get("*/api/workspaces/:workspaceSlug/contacts/:id/activities", ({ params }) => {
    return HttpResponse.json({ data: mockActivities.filter((a) => a.contactId === params.id) });
  }),

  // Deal rooms
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms", () => {
    return HttpResponse.json({ data: mockDealRooms });
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id", ({ params }) => {
    const room = mockDealRooms.find((r) => r.id === params.id);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(room);
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms", async ({ request }) => {
    const body = (await request.json()) as {
      name: string;
      slug: string;
      description?: string;
      template_type?: string;
      requires_nda?: boolean;
      requires_approval?: boolean;
    };
    const scenario = body.template_type?.replace(/_/g, "-") ?? "custom";
    const template = mockDealRoomTemplates.find((t) => t.scenario === scenario);
    const newRoom = {
      id: generateId("dr"),
      name: body.name,
      description: body.description ?? "",
      template: (template?.scenario ?? scenario) as import("@/types").DealRoom["template"],
      ndaEnabled: body.requires_nda ?? false,
      documentCount: 0,
      memberCount: 0,
      pendingApprovals: 0,
      createdAt: new Date().toISOString(),
      lastAccessedAt: undefined,
      status: "active" as const,
      uploadedFiles: [],
      recentVisitors: [],
    };
    mockDealRooms.unshift(newRoom);
    return HttpResponse.json(newRoom, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-room-templates", () => {
    return HttpResponse.json({ data: mockDealRoomTemplates });
  }),

  // Insights
  http.get("*/api/workspaces/:workspaceSlug/insights/overview", () => {
    const tierCounts = {
      hot: mockHeatAlerts.filter((a) => a.heatLevel === "hot").length + 2,
      warm: mockHeatAlerts.filter((a) => a.heatLevel === "warm").length + 1,
      cold: mockLinks.filter((l) => l.heatLevel === "cold").length,
    };
    const topDocuments = mockDocuments
      .map((d) => {
        const views = mockLinks
          .filter((l) => l.documentId === d.id)
          .reduce((sum, l) => sum + l.accessCount, 0);
        const heatLevel = views > 30 ? "hot" : views > 5 ? "warm" : "cold";
        return { id: d.id, title: d.title, views, heatLevel };
      })
      .sort((a, b) => b.views - a.views)
      .slice(0, 5);
    const topLinks = mockLinks
      .map((l) => ({
        id: l.id,
        shortUrl: l.shortUrl,
        views: l.accessCount,
        heatLevel: l.heatLevel,
      }))
      .sort((a, b) => b.views - a.views)
      .slice(0, 5);
    const topContacts = mockContacts
      .map((c) => ({
        id: c.id,
        email: c.email,
        score: c.score,
        heatLevel: c.heatLevel,
      }))
      .sort((a, b) => b.score - a.score)
      .slice(0, 5);
    return HttpResponse.json({ tierCounts, topDocuments, topLinks, topContacts });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/pages/:documentId", ({ params }) => {
    return HttpResponse.json({ data: mockPageAnalytics[params.documentId as string] || [] });
  }),

  http.get("*/api/workspaces/:workspaceSlug/insights/suggestions", () => {
    return HttpResponse.json({ data: mockSuggestions });
  }),

  // Assistant
  http.post("*/api/workspaces/:workspaceSlug/assistant/chat", async ({ request }) => {
    const body = (await request.json()) as { query: string; document_id?: string; session_id?: string };
    return HttpResponse.json({
      session_id: body.session_id || generateId("sess"),
      answer: `Based on the document, here's an answer to "${body.query}".`,
      evidence: body.document_id
        ? [
            {
              chunk_id: "chk_demo_001",
              quote: "Revenue grew 3x year over year.",
              page_number: 1,
              boxes: [{ x: 0.12, y: 0.34, w: 0.45, h: 0.06 }],
              score: 0.92,
            },
          ]
        : [],
      follow_up_questions: ["Can you explain the growth drivers?", "What are the risks?"],
    });
  }),

  // Search
  http.post("*/api/workspaces/:workspaceSlug/search", async ({ request }) => {
    const body = (await request.json()) as { query: string; document_id?: string };
    return HttpResponse.json({
      document_id: body.document_id,
      query: body.query,
      results: body.document_id
        ? [
            {
              chunk_id: "chk_demo_001",
              quote: "Revenue grew 3x year over year.",
              page_number: 1,
              boxes: [{ x: 0.12, y: 0.34, w: 0.45, h: 0.06 }],
              score: 0.92,
            },
          ]
        : [],
    });
  }),

  // Members
  http.get("*/api/workspaces/:workspaceSlug/members", () => {
    return HttpResponse.json({ data: mockWorkspaceMembers });
  }),

  http.post("*/api/workspaces/:workspaceSlug/invitations", async ({ request }) => {
    const body = (await request.json()) as { email: string; role: WorkspaceMember["role"] };
    const newMember = {
      id: generateId("wm"),
      userId: generateId("u"),
      email: body.email,
      name: body.email.split("@")[0],
      role: body.role,
      joinedAt: new Date().toISOString(),
      status: "pending" as const,
    };
    mockWorkspaceMembers.push(newMember);
    return HttpResponse.json({ data: newMember }, { status: 201 });
  }),

  // Workspace settings
  http.get("*/api/workspaces/:workspaceSlug/settings", () => {
    return HttpResponse.json({ data: workspaceSettings });
  }),

  http.put("*/api/workspaces/:workspaceSlug/settings", async ({ request }) => {
    const body = (await request.json()) as typeof workspaceSettings;
    workspaceSettings = { ...workspaceSettings, ...body };
    return HttpResponse.json({ data: workspaceSettings });
  }),

  http.post("*/api/workspaces/:workspaceSlug/logo", async () => {
    const mockLogoUrl = "https://placehold.co/128x128/0f172a/ffffff?text=Logo";
    workspaceSettings = { ...workspaceSettings, logoUrl: mockLogoUrl };
    return HttpResponse.json({ data: { logoUrl: mockLogoUrl } }, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/billing", () => {
    const totalStorage = mockDocuments.reduce((sum, d) => sum + d.fileSize, 0);
    return HttpResponse.json({
      data: {
        plan: "Pro",
        period: "Annual",
        storageUsed: Math.round((totalStorage / 1024 / 1024) * 10) / 10,
        storageLimit: 50,
        linksUsed: mockLinks.length,
        linksLimit: 100,
        roomsUsed: mockDealRooms.length,
        roomsLimit: 10,
      },
    });
  }),

  // Integrations
  http.get("*/api/workspaces/:workspaceSlug/integrations/settings", () => {
    return HttpResponse.json({ data: integrationsStatus });
  }),

  http.put("*/api/workspaces/:workspaceSlug/integrations/settings", async ({ request }) => {
    const body = (await request.json()) as typeof integrationsStatus;
    integrationsStatus = { ...integrationsStatus, ...body };
    return HttpResponse.json({ data: integrationsStatus });
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/slack/connect", () => {
    return HttpResponse.json({ url: "https://slack.com/oauth/v2/authorize?client_id=mock" });
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/slack/disconnect", () => {
    integrationsStatus.slack = false;
    return HttpResponse.json({ code: "ok", message: "disconnected" });
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/hubspot/connect", () => {
    return HttpResponse.json({ url: "https://app.hubspot.com/oauth/authorize?client_id=mock" });
  }),

  http.post("*/api/workspaces/:workspaceSlug/integrations/hubspot/disconnect", () => {
    integrationsStatus.hubspot = false;
    return HttpResponse.json({ code: "ok", message: "disconnected" });
  }),

  // Security
  http.get("*/api/workspaces/:workspaceSlug/security", () => {
    return HttpResponse.json({ data: securitySettings });
  }),

  http.put("*/api/workspaces/:workspaceSlug/security", async ({ request }) => {
    const body = (await request.json()) as typeof securitySettings;
    securitySettings = { ...securitySettings, ...body };
    return HttpResponse.json({ data: securitySettings });
  }),

  // Signals
  http.get("*/api/workspaces/:workspaceSlug/signals", () => {
    return HttpResponse.json(getMockSignalFeed());
  }),

  http.get("*/api/workspaces/:workspaceSlug/signals/:id", ({ params }) => {
    const signal = mockSignals.find((s) => s.id === params.id);
    if (!signal) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(signal);
  }),

  http.patch("*/api/workspaces/:workspaceSlug/signals/actions/:id", async ({ params, request }) => {
    const body = (await request.json()) as { status?: string };
    const action = mockActionItems.find((a) => a.id === params.id);
    if (!action) return new HttpResponse(null, { status: 404 });
    if (body?.status) action.status = body.status as ActionItem["status"];
    return HttpResponse.json(action);
  }),

  // Public viewer
  http.get("*/api/v1/public/links/:token", ({ params, request }) => {
    const url = new URL(request.url);
    const token = params.token as string;
    const link = mockLinks.find((l) => l.shortUrl.endsWith(token)) ?? mockLinks[0];
    const doc = mockDocuments.find((d) => d.id === link.documentId) ?? mockDocuments[0];
    return HttpResponse.json({
      link: {
        id: link.id,
        name: link.documentTitle,
        documentId: link.documentId,
        permissionType: link.permissionType ?? "public",
        downloadEnabled: true,
        watermarkEnabled: false,
      },
      document: {
        id: doc.id,
        title: doc.title,
        pageCount: doc.pageCount,
        status: doc.status,
        sourceType: doc.fileType,
        fileSize: doc.fileSize,
      },
      visitorId: generateId("visitor"),
      requiresEmail: link.permissionType === "email" && !url.searchParams.get("email"),
      requiresPassword: link.permissionType === "password" && !url.searchParams.get("password"),
      requiresNda: link.permissionType === "nda" && url.searchParams.get("nda_agreed") !== "true",
    });
  }),

  http.get("*/api/v1/public/documents/:documentId/pages", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.documentId);
    if (!doc) return new HttpResponse(null, { status: 404 });
    const pages = Array.from({ length: doc.pageCount }, (_, i) => ({
      pageNumber: i + 1,
      width: 800,
      height: 1000,
    }));
    return HttpResponse.json({ documentId: doc.id, pages, total: pages.length });
  }),

  http.get("*/api/v1/public/documents/:documentId/pages/signed-url", ({ params, request }) => {
    const doc = mockDocuments.find((d) => d.id === params.documentId);
    if (!doc) return new HttpResponse(null, { status: 404 });
    const url = new URL(request.url);
    const pageNumber = Number(url.searchParams.get("page_number") ?? "1");
    return HttpResponse.json({
      pageNumber,
      imageUrl: placeholdImageUrl(800, 1000),
      expiresAt: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      width: 800,
      height: 1000,
    });
  }),

  http.get("*/api/v1/public/documents/:documentId/download-url", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.documentId);
    if (!doc) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({
      downloadUrl: placeholdImageUrl(200, 200),
      expiresAt: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      filename: doc.fileName,
      contentType: "application/pdf",
    });
  }),

  http.post("*/api/v1/public/events", async () => {
    return new HttpResponse(null, { status: 204 });
  }),
];

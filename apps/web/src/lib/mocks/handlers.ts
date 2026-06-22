import { http, HttpResponse } from "msw";
import {
  mockAccessLogs,
  mockActivities,
  mockContacts,
  mockDealRooms,
  mockDealRoomTemplates,
  mockDocuments,
  mockHeatAlerts,
  mockLinks,
  mockPageAnalytics,
  mockRiskAlerts,
  mockSignals,
  mockSuggestions,
  mockWorkspaceMembers,
  mockWorkspaces,
  defaultWorkspaceSettings,
  getMockDashboardStats,
  getMockSignalFeed,
} from "./data";
import type { DealRoom, WorkspaceMember } from "@/types";

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

export const handlers = [
  http.get("*/api/workspaces", () => {
    return HttpResponse.json({ data: mockWorkspaces });
  }),

  http.get("*/api/workspaces/:workspaceSlug/dashboard/stats", () => {
    return HttpResponse.json(getMockDashboardStats());
  }),

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

  http.post("*/api/workspaces/:workspaceSlug/documents", async () => {
    return HttpResponse.json(mockDocuments[0], { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/links", ({ request }) => {
    const url = new URL(request.url);
    const documentId = url.searchParams.get("documentId");
    const data = documentId
      ? mockLinks.filter((l) => l.documentId === documentId)
      : mockLinks;
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

  http.post("*/api/workspaces/:workspaceSlug/links", async () => {
    return HttpResponse.json(mockLinks[0], { status: 201 });
  }),

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
    const newRoom: DealRoom = {
      id: `dr_${Date.now()}`,
      name: body.name,
      description: body.description ?? "",
      template: template?.scenario ?? (scenario as DealRoom["template"]),
      ndaEnabled: body.requires_nda ?? false,
      documentCount: 0,
      memberCount: 0,
      pendingApprovals: 0,
      createdAt: new Date().toISOString(),
      lastAccessedAt: undefined,
      status: "active",
      uploadedFiles: [],
      recentVisitors: [],
    };
    mockDealRooms.unshift(newRoom);
    return HttpResponse.json(newRoom, { status: 201 });
  }),

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

  http.get("*/api/workspaces/:workspaceSlug/workspace/members", () => {
    return HttpResponse.json({ data: mockWorkspaceMembers });
  }),

  http.get("*/api/workspaces/:workspaceSlug/signals", () => {
    return HttpResponse.json(getMockSignalFeed());
  }),

  http.get("*/api/workspaces/:workspaceSlug/signals/:id", ({ params }) => {
    const signal = mockSignals.find((s) => s.id === params.id);
    if (!signal) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(signal);
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-room-templates", () => {
    return HttpResponse.json({ data: mockDealRoomTemplates });
  }),

  http.get("*/api/workspaces/:workspaceSlug/risk-alerts", () => {
    return HttpResponse.json({ data: mockRiskAlerts });
  }),

  http.post("*/api/workspaces/:workspaceSlug/assistant/chat", async ({ request }) => {
    const body = (await request.json()) as { query: string; document_id?: string; session_id?: string };
    return HttpResponse.json({
      session_id: body.session_id || `sess_${Date.now()}`,
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

  http.get("*/api/workspaces/:workspaceSlug/workspace/settings", () => {
    return HttpResponse.json({ data: workspaceSettings });
  }),

  http.put("*/api/workspaces/:workspaceSlug/workspace/settings", async ({ request }) => {
    const body = (await request.json()) as typeof workspaceSettings;
    workspaceSettings = { ...workspaceSettings, ...body };
    return HttpResponse.json({ data: workspaceSettings });
  }),

  http.post("*/api/workspaces/:workspaceSlug/workspace/logo", async () => {
    // 模拟上传：返回固定 CDN 图片 URL（生产环境会替换为实际上传地址）
    const mockLogoUrl = "https://placehold.co/128x128/0f172a/ffffff?text=Logo";
    workspaceSettings = { ...workspaceSettings, logoUrl: mockLogoUrl };
    return HttpResponse.json({ data: { logoUrl: mockLogoUrl } }, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/workspace/billing", () => {
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

  http.get("*/api/workspaces/:workspaceSlug/workspace/integrations", () => {
    return HttpResponse.json({ data: integrationsStatus });
  }),

  http.put("*/api/workspaces/:workspaceSlug/workspace/integrations", async ({ request }) => {
    const body = (await request.json()) as typeof integrationsStatus;
    integrationsStatus = { ...integrationsStatus, ...body };
    return HttpResponse.json({ data: integrationsStatus });
  }),

  http.get("*/api/workspaces/:workspaceSlug/workspace/security", () => {
    return HttpResponse.json({ data: securitySettings });
  }),

  http.put("*/api/workspaces/:workspaceSlug/workspace/security", async ({ request }) => {
    const body = (await request.json()) as typeof securitySettings;
    securitySettings = { ...securitySettings, ...body };
    return HttpResponse.json({ data: securitySettings });
  }),

  http.post("*/api/workspaces/:workspaceSlug/workspace/members", async ({ request }) => {
    const body = (await request.json()) as { email: string; role: WorkspaceMember["role"] };
    const newMember: WorkspaceMember = {
      id: `wm_${Date.now()}`,
      userId: `u_${Date.now()}`,
      email: body.email,
      name: body.email.split("@")[0],
      role: body.role,
      joinedAt: new Date().toISOString(),
      status: "pending",
    };
    mockWorkspaceMembers.push(newMember);
    return HttpResponse.json({ data: newMember }, { status: 201 });
  }),
];

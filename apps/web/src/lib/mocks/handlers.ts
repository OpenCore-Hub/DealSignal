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
  http.get("/api/workspaces", () => {
    return HttpResponse.json({ data: mockWorkspaces });
  }),

  http.get("/api/dashboard/stats", () => {
    return HttpResponse.json(getMockDashboardStats());
  }),

  http.get("/api/documents", () => {
    return HttpResponse.json({ data: mockDocuments });
  }),

  http.get("/api/documents/:id", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(doc);
  }),

  http.delete("/api/documents/:id", ({ params }) => {
    const index = mockDocuments.findIndex((d) => d.id === params.id);
    if (index === -1) return new HttpResponse(null, { status: 404 });
    mockDocuments.splice(index, 1);
    return new HttpResponse(null, { status: 204 });
  }),

  http.post("/api/documents", async () => {
    return HttpResponse.json(mockDocuments[0], { status: 201 });
  }),

  http.get("/api/links", ({ request }) => {
    const url = new URL(request.url);
    const documentId = url.searchParams.get("documentId");
    const data = documentId
      ? mockLinks.filter((l) => l.documentId === documentId)
      : mockLinks;
    return HttpResponse.json({ data });
  }),

  http.get("/api/links/:id", ({ params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(link);
  }),

  http.patch("/api/links/:id", async ({ request, params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    const patch = (await request.json()) as Partial<typeof link>;
    Object.assign(link, patch);
    return HttpResponse.json(link);
  }),

  http.get("/api/links/:id/access-logs", ({ params }) => {
    return HttpResponse.json({ data: mockAccessLogs.filter((l) => l.linkId === params.id) });
  }),

  http.post("/api/links", async () => {
    return HttpResponse.json(mockLinks[0], { status: 201 });
  }),

  http.get("/api/contacts", () => {
    return HttpResponse.json({ data: mockContacts });
  }),

  http.get("/api/contacts/:id", ({ params }) => {
    const contact = mockContacts.find((c) => c.id === params.id);
    if (!contact) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(contact);
  }),

  http.get("/api/contacts/:id/activities", ({ params }) => {
    return HttpResponse.json({ data: mockActivities.filter((a) => a.contactId === params.id) });
  }),

  http.get("/api/deal-rooms", () => {
    return HttpResponse.json({ data: mockDealRooms });
  }),

  http.get("/api/deal-rooms/:id", ({ params }) => {
    const room = mockDealRooms.find((r) => r.id === params.id);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(room);
  }),

  http.post("/api/deal-rooms", async ({ request }) => {
    const body = (await request.json()) as {
      name: string;
      description: string;
      templateId: string;
      ndaEnabled: boolean;
    };
    const template = mockDealRoomTemplates.find((t) => t.id === body.templateId);
    const newRoom: DealRoom = {
      id: `dr_${Date.now()}`,
      name: body.name,
      description: body.description,
      template: template?.scenario ?? "custom",
      ndaEnabled: body.ndaEnabled,
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

  http.get("/api/insights/overview", () => {
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

  http.get("/api/insights/pages/:documentId", ({ params }) => {
    return HttpResponse.json({ data: mockPageAnalytics[params.documentId as string] || [] });
  }),

  http.get("/api/insights/suggestions", () => {
    return HttpResponse.json({ data: mockSuggestions });
  }),

  http.get("/api/workspace/members", () => {
    return HttpResponse.json({ data: mockWorkspaceMembers });
  }),

  http.get("/api/signals", () => {
    return HttpResponse.json(getMockSignalFeed());
  }),

  http.get("/api/signals/:id", ({ params }) => {
    const signal = mockSignals.find((s) => s.id === params.id);
    if (!signal) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json(signal);
  }),

  http.get("/api/deal-room-templates", () => {
    return HttpResponse.json({ data: mockDealRoomTemplates });
  }),

  http.get("/api/risk-alerts", () => {
    return HttpResponse.json({ data: mockRiskAlerts });
  }),

  http.get("/api/workspace/settings", () => {
    return HttpResponse.json({ data: workspaceSettings });
  }),

  http.put("/api/workspace/settings", async ({ request }) => {
    const body = (await request.json()) as typeof workspaceSettings;
    workspaceSettings = { ...workspaceSettings, ...body };
    return HttpResponse.json({ data: workspaceSettings });
  }),

  http.post("/api/workspace/logo", async () => {
    // 模拟上传：返回固定 CDN 图片 URL（生产环境会替换为实际上传地址）
    const mockLogoUrl = "https://placehold.co/128x128/0f172a/ffffff?text=Logo";
    workspaceSettings = { ...workspaceSettings, logoUrl: mockLogoUrl };
    return HttpResponse.json({ data: { logoUrl: mockLogoUrl } }, { status: 201 });
  }),

  http.get("/api/workspace/billing", () => {
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

  http.get("/api/workspace/integrations", () => {
    return HttpResponse.json({ data: integrationsStatus });
  }),

  http.put("/api/workspace/integrations", async ({ request }) => {
    const body = (await request.json()) as typeof integrationsStatus;
    integrationsStatus = { ...integrationsStatus, ...body };
    return HttpResponse.json({ data: integrationsStatus });
  }),

  http.get("/api/workspace/security", () => {
    return HttpResponse.json({ data: securitySettings });
  }),

  http.put("/api/workspace/security", async ({ request }) => {
    const body = (await request.json()) as typeof securitySettings;
    securitySettings = { ...securitySettings, ...body };
    return HttpResponse.json({ data: securitySettings });
  }),

  http.post("/api/workspace/members", async ({ request }) => {
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

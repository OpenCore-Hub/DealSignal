import { http, HttpResponse } from "msw";
import type { DealRoom } from "@/types";
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
  getMockDashboardStats,
  getMockSignalFeed,
} from "./data";

export const handlers = [
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
];

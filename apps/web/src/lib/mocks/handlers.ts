import { http, HttpResponse } from "msw";
import type {
  ActionItem,
  Contact,
  DealRoom,
  DealRoomDocumentItem,
  DealRoomFolder,
  DealRoomFolderDocs,
  DealRoomMember,
  Link,
  WorkspaceMember,
} from "@/types";
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

function findRoom(roomId: string): DealRoom | undefined {
  return mockDealRooms.find((r) => r.id === roomId);
}

function getRoomFolders(room: DealRoom): DealRoomFolder[] {
  return room.folders ?? [{ path: "/", name: "Root", sort_order: 0 }];
}

function getRoomFolderDocs(room: DealRoom): DealRoomFolderDocs[] {
  return room.documents ?? [];
}

function nextSortOrder(arr: { sort_order: number }[]): number {
  return arr.length === 0 ? 0 : Math.max(...arr.map((x) => x.sort_order)) + 1;
}

function sanitizeFolderPath(name: string, parentPath = "/"): string {
  const slug = name.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
  if (parentPath === "/") return `/${slug}`;
  return `${parentPath}/${slug}`;
}

function updateRoomDerivedFields(room: DealRoom) {
  const docs = getRoomFolderDocs(room);
  room.documentCount = docs.reduce((sum, fd) => sum + fd.documents.length, 0);
  room.memberCount = room.members?.length ?? 0;
  room.pendingApprovals = room.accessRequests?.filter((r) => r.status === "pending").length ?? 0;
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
  http.get("*/api/workspaces/:workspaceSlug/documents", ({ request }) => {
    const filter = new URL(request.url).searchParams.get("filter");
    let docs: typeof mockDocuments;
    switch (filter) {
      case "recent": {
        const lastAccessedAt = (docId: string) => {
          const linkDates = mockLinks
            .filter((l) => l.documentId === docId && l.lastViewedAt)
            .map((l) => new Date(l.lastViewedAt!).getTime());
          return Math.max(...linkDates, 0);
        };
        docs = [...mockDocuments].sort(
          (a, b) => lastAccessedAt(b.id) - lastAccessedAt(a.id) || new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
        );
        break;
      }
      case "popular": {
        const totalViews = (docId: string) =>
          mockLinks.filter((l) => l.documentId === docId).reduce((sum, l) => sum + l.accessCount, 0);
        docs = [...mockDocuments]
          .filter((d) => d.status !== "archived" && totalViews(d.id) >= 30)
          .sort(
            (a, b) =>
              totalViews(b.id) - totalViews(a.id) || new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
          );
        break;
      }
      case "unshared":
        docs = mockDocuments.filter((d) => !mockLinks.some((l) => l.documentId === d.id && l.isActive));
        break;
      case "archived":
        docs = mockDocuments.filter((d) => d.status === "archived");
        break;
      default:
        docs = mockDocuments;
    }
    return HttpResponse.json({ data: docs });
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

  http.post("*/api/workspaces/:workspaceSlug/documents/:id/archive", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    doc.status = "archived";
    return HttpResponse.json(doc);
  }),

  http.post("*/api/workspaces/:workspaceSlug/documents/:id/unarchive", ({ params }) => {
    const doc = mockDocuments.find((d) => d.id === params.id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    doc.status = "ready";
    return HttpResponse.json(doc);
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
      sourceType: fileType,
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

  http.put("*/api/workspaces/:workspaceSlug/links/:id", async ({ request, params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    const payload = (await request.json()) as {
      document_ids?: string[];
      name?: string;
      permission_type?: string;
      require_email_verification?: boolean;
      require_password?: boolean;
      require_nda?: boolean;
      allowed_emails?: string[];
      allowed_domains?: string[];
      password?: string;
      contact_ids?: string[];
      expires_at?: string;
      max_access_count?: number;
      download_enabled?: boolean;
      watermark_enabled?: boolean;
      ai_copilot_enabled?: boolean;
    };
    // Update the in-memory link to reflect the edited values so subsequent reads
    // (including tests) see the new state.
    if (payload.document_ids && payload.document_ids.length > 0) {
      link.documentIds = payload.document_ids;
      link.documentId = payload.document_ids[0];
      const selectedDocs = payload.document_ids
        .map((id) => mockDocuments.find((d) => d.id === id))
        .filter(Boolean) as typeof mockDocuments;
      link.documents = selectedDocs.map((d) => ({
        id: d.id,
        title: d.title,
        sourceType: d.sourceType,
        pageCount: d.pageCount,
        status: d.status,
        fileSize: d.fileSize,
      }));
      link.documentTitle = selectedDocs.map((d) => d.title).join(", ") || link.documentTitle;
      link.isBundle = payload.document_ids.length > 1;
    }
    if (payload.permission_type) link.permissionType = payload.permission_type as Link["permissionType"];
    if (typeof payload.require_email_verification === "boolean") link.requireEmailVerification = payload.require_email_verification;
    if (typeof payload.require_password === "boolean") link.requirePassword = payload.require_password;
    if (typeof payload.require_nda === "boolean") link.requireNda = payload.require_nda;
    if (payload.allowed_emails) link.allowedEmails = payload.allowed_emails;
    if (payload.allowed_domains) link.allowedDomains = payload.allowed_domains;
    if (payload.expires_at) link.expiresAt = payload.expires_at;
    if (typeof payload.max_access_count === "number") link.maxAccessCount = payload.max_access_count;
    if (typeof payload.download_enabled === "boolean") link.downloadEnabled = payload.download_enabled;
    if (typeof payload.watermark_enabled === "boolean") link.watermarkEnabled = payload.watermark_enabled;
    if (typeof payload.ai_copilot_enabled === "boolean") link.aiCopilotEnabled = payload.ai_copilot_enabled;
    if (payload.contact_ids) link.contactIds = payload.contact_ids;
    return HttpResponse.json(link);
  }),

  http.patch("*/api/workspaces/:workspaceSlug/links/:id", async ({ request, params }) => {
    const link = mockLinks.find((l) => l.id === params.id);
    if (!link) return new HttpResponse(null, { status: 404 });
    const patch = (await request.json()) as Partial<typeof link>;
    Object.assign(link, patch);
    return HttpResponse.json(link);
  }),

  http.delete("*/api/workspaces/:workspaceSlug/links/:id", ({ params }) => {
    const index = mockLinks.findIndex((l) => l.id === params.id);
    if (index === -1) return new HttpResponse(null, { status: 404 });
    mockLinks.splice(index, 1);
    return new HttpResponse(null, { status: 204 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/links/:id/access-logs", ({ params }) => {
    return HttpResponse.json({ data: mockAccessLogs.filter((l) => l.linkId === params.id) });
  }),

  http.post("*/api/workspaces/:workspaceSlug/links", async ({ request }) => {
    const body = (await request.json()) as {
      document_id: string;
      name?: string;
      permission_type?: string;
      require_email?: boolean;
      require_email_verification?: boolean;
      require_password?: boolean;
      require_nda?: boolean;
      allowed_emails?: string[];
      allowed_domains?: string[];
      password?: string;
      expires_at?: string;
      max_access_count?: number;
      download_enabled?: boolean;
      watermark_enabled?: boolean;
    };
    const doc = mockDocuments.find((d) => d.id === body.document_id);

    const requirePassword = body.require_password || body.permission_type === "password" || !!body.password;
    const requireNDA = body.require_nda || body.permission_type === "nda";
    const hasWhitelist = (body.allowed_emails && body.allowed_emails.length > 0) || (body.allowed_domains && body.allowed_domains.length > 0);
    const requireEmailVerification =
      body.require_email_verification ||
      body.permission_type === "email_required" ||
      body.permission_type === "whitelist" ||
      hasWhitelist ||
      requireNDA ||
      false;

    let permissionType: "public" | "email" | "password" | "nda" = "public";
    if (requirePassword) permissionType = "password";
    else if (requireNDA) permissionType = "nda";
    // Modern email verification uses permission_type "public" + require_email_verification.
    // Only the legacy "email_required" permission type maps to "email".
    else if (body.permission_type === "email_required" || body.require_email) permissionType = "email";

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
      permissionType,
      _requireEmailVerification: requireEmailVerification,
      _requirePassword: requirePassword,
      _requireNDA: requireNDA,
      _password: body.password,
      _allowedEmails: body.allowed_emails ?? [],
      _allowedDomains: body.allowed_domains ?? [],
    } as Link & {
      _requireEmailVerification?: boolean;
      _requirePassword?: boolean;
      _requireNDA?: boolean;
      _password?: string;
      _allowedEmails?: string[];
      _allowedDomains?: string[];
    };
    mockLinks.unshift(newLink);
    return HttpResponse.json(newLink, { status: 201 });
  }),

  // Contacts
  http.get("*/api/workspaces/:workspaceSlug/contacts", () => {
    return HttpResponse.json({ data: mockContacts });
  }),

  http.post("*/api/workspaces/:workspaceSlug/contacts", async ({ request }) => {
    const body = (await request.json()) as { email: string; name?: string };
    const newContact: Contact = {
      id: generateId("contact"),
      email: body.email,
      name: body.name ?? "",
      organization: "",
      role: "",
      heatLevel: "cold",
      score: 0,
      scoreHistory: [],
      totalVisits: 0,
      totalDurationSeconds: 0,
      viewedDocuments: [],
    };
    mockContacts.unshift(newContact);
    return HttpResponse.json(newContact, { status: 201 });
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
    const room = findRoom(params.id as string);
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
    const folders: DealRoomFolder[] = template
      ? template.folderStructure.map((f, i) => ({
          path: `/${f.name.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`,
          name: f.name,
          description: f.description,
          sort_order: i,
        }))
      : [{ path: "/", name: "Root", sort_order: 0 }];
    const newRoom: DealRoom = {
      id: generateId("dr"),
      name: body.name,
      description: body.description ?? "",
      slug: body.slug,
      template: (template?.scenario ?? scenario) as DealRoom["template"],
      ndaEnabled: body.requires_nda ?? false,
      requiresApproval: body.requires_approval ?? false,
      documentCount: 0,
      memberCount: 0,
      pendingApprovals: 0,
      createdAt: new Date().toISOString(),
      lastAccessedAt: undefined,
      status: "active",
      folders,
      documents: [],
      members: [],
      accessRequests: [],
    };
    mockDealRooms.unshift(newRoom);
    return HttpResponse.json(newRoom, { status: 201 });
  }),

  http.get("*/api/workspaces/:workspaceSlug/deal-room-templates", () => {
    return HttpResponse.json({ data: mockDealRoomTemplates });
  }),

  // Deal room folders
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/folders", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: getRoomFolders(room) });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/folders", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { name: string; parent_path?: string };
    const folders = getRoomFolders(room);
    const path = sanitizeFolderPath(body.name, body.parent_path ?? "/");
    if (folders.some((f) => f.path === path)) {
      return HttpResponse.json({ code: "folder_exists", message: "folder already exists" }, { status: 409 });
    }
    folders.push({
      path,
      name: body.name,
      sort_order: nextSortOrder(folders),
    });
    room.folders = folders;
    return HttpResponse.json({ data: folders }, { status: 201 });
  }),

  http.patch("*/api/workspaces/:workspaceSlug/deal-rooms/:id/folders/*path", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const path = `/${params.path as string}`;
    const folders = getRoomFolders(room);
    const folder = folders.find((f) => f.path === path);
    if (!folder) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { name: string };
    const newPath = sanitizeFolderPath(body.name, "/");
    if (newPath !== path && folders.some((f) => f.path === newPath)) {
      return HttpResponse.json({ code: "folder_exists", message: "folder already exists" }, { status: 409 });
    }
    folder.name = body.name;
    folder.path = newPath;
    // Cascade update documents in this folder.
    const docs = getRoomFolderDocs(room);
    for (const fd of docs) {
      if (fd.folder === path) fd.folder = newPath;
      for (const doc of fd.documents) {
        if (doc.folder_path === path) doc.folder_path = newPath;
      }
    }
    room.folders = folders;
    return HttpResponse.json({ data: folders });
  }),

  http.delete("*/api/workspaces/:workspaceSlug/deal-rooms/:id/folders/*path", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const path = `/${params.path as string}`;
    const docs = getRoomFolderDocs(room);
    const hasDocs = docs.some((fd) => fd.folder === path && fd.documents.length > 0);
    if (hasDocs) {
      return HttpResponse.json({ code: "folder_not_empty", message: "folder is not empty" }, { status: 400 });
    }
    const folders = getRoomFolders(room).filter((f) => f.path !== path);
    room.folders = folders;
    return HttpResponse.json({ data: folders });
  }),

  // Deal room documents
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/documents", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: getRoomFolderDocs(room) });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/documents", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as {
      document_id: string;
      folder_path?: string;
      sort_order?: number;
    };
    const doc = mockDocuments.find((d) => d.id === body.document_id);
    if (!doc) return new HttpResponse(null, { status: 404 });
    const folderPath = body.folder_path ?? "/";
    const folders = getRoomFolders(room);
    if (!folders.some((f) => f.path === folderPath)) {
      return HttpResponse.json({ code: "folder_not_found", message: "folder not found" }, { status: 404 });
    }
    const docs = getRoomFolderDocs(room);
    let fd = docs.find((d) => d.folder === folderPath);
    if (!fd) {
      fd = { folder: folderPath, permission: "view", documents: [] };
      docs.push(fd);
    }
    const item: DealRoomDocumentItem = {
      id: generateId("rd"),
      document_id: doc.id,
      title: doc.title,
      folder_path: folderPath,
      sort_order: body.sort_order ?? nextSortOrder(fd.documents),
      source_type: doc.sourceType,
      status: doc.status,
      page_count: doc.pageCount,
      file_size: doc.fileSize,
      created_at: doc.createdAt,
    };
    fd.documents.push(item);
    room.documents = docs;
    updateRoomDerivedFields(room);
    return HttpResponse.json(item, { status: 201 });
  }),

  http.patch("*/api/workspaces/:workspaceSlug/deal-rooms/:id/documents/:docId", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { folder_path?: string; sort_order?: number };
    const docs = getRoomFolderDocs(room);
    let item: DealRoomDocumentItem | undefined;
    let fromFd: DealRoomFolderDocs | undefined;
    for (const fd of docs) {
      const found = fd.documents.find((d) => d.id === params.docId);
      if (found) {
        item = found;
        fromFd = fd;
        break;
      }
    }
    if (!item || !fromFd) return new HttpResponse(null, { status: 404 });

    if (typeof body.sort_order === "number") {
      item.sort_order = body.sort_order;
      // Swap sort_order with adjacent document when moving up/down.
      const siblings = fromFd.documents.filter((d) => d.id !== item!.id).sort((a, b) => a.sort_order - b.sort_order);
      for (const sibling of siblings) {
        if (sibling.sort_order === item.sort_order) {
          sibling.sort_order = item.sort_order + (body.sort_order < sibling.sort_order ? 1 : -1);
        }
      }
    }

    if (body.folder_path !== undefined && body.folder_path !== item.folder_path) {
      fromFd.documents = fromFd.documents.filter((d) => d.id !== item!.id);
      if (fromFd.documents.length === 0) {
        room.documents = docs.filter((d) => d !== fromFd);
      }
      let toFd = docs.find((d) => d.folder === body.folder_path);
      if (!toFd) {
        toFd = { folder: body.folder_path, permission: "view", documents: [] };
        docs.push(toFd);
      }
      item.folder_path = body.folder_path;
      item.sort_order = nextSortOrder(toFd.documents);
      toFd.documents.push(item);
    }

    room.documents = docs;
    updateRoomDerivedFields(room);
    return HttpResponse.json(item);
  }),

  http.delete("*/api/workspaces/:workspaceSlug/deal-rooms/:id/documents/:docId", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const docs = getRoomFolderDocs(room);
    for (const fd of docs) {
      const idx = fd.documents.findIndex((d) => d.id === params.docId);
      if (idx !== -1) {
        fd.documents.splice(idx, 1);
        break;
      }
    }
    room.documents = docs.filter((fd) => fd.documents.length > 0);
    updateRoomDerivedFields(room);
    return new HttpResponse(null, { status: 204 });
  }),

  // Deal room members
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/members", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: room.members ?? [] });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/members", async ({ request, params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { email: string; role: DealRoomMember["role"] };
    const members = room.members ?? [];
    const newMember: DealRoomMember = {
      id: generateId("rm"),
      email: body.email,
      role: body.role,
      nda_status: room.ndaEnabled ? "pending" : "none",
      status: "active",
    };
    members.push(newMember);
    room.members = members;
    updateRoomDerivedFields(room);
    return HttpResponse.json({ data: newMember }, { status: 201 });
  }),

  http.delete("*/api/workspaces/:workspaceSlug/deal-rooms/:id/members/:memberId", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const members = room.members ?? [];
    const index = members.findIndex((m) => m.id === params.memberId);
    if (index === -1) return new HttpResponse(null, { status: 404 });
    members.splice(index, 1);
    room.members = members;
    updateRoomDerivedFields(room);
    return new HttpResponse(null, { status: 204 });
  }),

  // Deal room access requests
  http.get("*/api/workspaces/:workspaceSlug/deal-rooms/:id/access-requests", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    return HttpResponse.json({ data: room.accessRequests ?? [] });
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/access-requests/:requestId/approve", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const requests = room.accessRequests ?? [];
    const request = requests.find((r) => r.id === params.requestId);
    if (!request) return new HttpResponse(null, { status: 404 });
    request.status = "approved";
    request.reviewed_at = new Date().toISOString();
    // Promote to member if not already present.
    const members = room.members ?? [];
    if (!members.some((m) => m.email === request.email)) {
      members.push({
        id: generateId("rm"),
        email: request.email,
        role: "viewer",
        nda_status: room.ndaEnabled ? "pending" : "none",
        status: "active",
      });
      room.members = members;
    }
    updateRoomDerivedFields(room);
    return HttpResponse.json(request);
  }),

  http.post("*/api/workspaces/:workspaceSlug/deal-rooms/:id/access-requests/:requestId/reject", ({ params }) => {
    const room = findRoom(params.id as string);
    if (!room) return new HttpResponse(null, { status: 404 });
    const requests = room.accessRequests ?? [];
    const request = requests.find((r) => r.id === params.requestId);
    if (!request) return new HttpResponse(null, { status: 404 });
    request.status = "rejected";
    request.reviewed_at = new Date().toISOString();
    updateRoomDerivedFields(room);
    return HttpResponse.json(request);
  }),

  // Public deal room
  http.get("*/api/v1/public/deal-rooms/:slug", ({ request, params }) => {
    const url = new URL(request.url);
    const email = url.searchParams.get("email")?.toLowerCase();
    const slug = params.slug as string;
    const room = mockDealRooms.find((r) => r.slug === slug || r.id === slug);
    if (!room) return new HttpResponse(null, { status: 404 });

    const member = room.members?.find((m) => m.email.toLowerCase() === email) ?? null;
    const requests = room.accessRequests ?? [];
    const pendingRequest = email ? requests.find((r) => r.email.toLowerCase() === email && r.status === "pending") : undefined;

    // If email is not a member and room requires approval, show request form.
    // If member exists but status is not active, treat as pending.
    let effectiveMember = member;
    if (!member && pendingRequest) {
      effectiveMember = {
        id: pendingRequest.id,
        email: pendingRequest.email,
        role: "viewer",
        nda_status: "none",
        status: "pending",
      };
    }

    return HttpResponse.json({
      room: {
        id: room.id,
        name: room.name,
        description: room.description,
        ndaEnabled: room.ndaEnabled,
        requiresApproval: room.requiresApproval ?? false,
      },
      member: effectiveMember
        ? {
            id: effectiveMember.id,
            email: effectiveMember.email,
            role: effectiveMember.role,
            ndaStatus: effectiveMember.nda_status,
            status: effectiveMember.status,
          }
        : null,
      folders: getRoomFolders(room),
      documents: getRoomFolderDocs(room),
    });
  }),

  http.post("*/api/v1/public/deal-rooms/:slug/access-requests", async ({ request, params }) => {
    const slug = params.slug as string;
    const room = mockDealRooms.find((r) => r.slug === slug || r.id === slug);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { email: string; reason?: string };
    const requests = room.accessRequests ?? [];
    if (!requests.some((r) => r.email.toLowerCase() === body.email.toLowerCase())) {
      requests.push({
        id: generateId("ra"),
        email: body.email,
        status: "pending",
        reason: body.reason,
      });
      room.accessRequests = requests;
      updateRoomDerivedFields(room);
    }
    return HttpResponse.json({ request_id: requests[requests.length - 1].id }, { status: 201 });
  }),

  http.post("*/api/v1/public/deal-rooms/:slug/nda", async ({ request, params }) => {
    const slug = params.slug as string;
    const room = mockDealRooms.find((r) => r.slug === slug || r.id === slug);
    if (!room) return new HttpResponse(null, { status: 404 });
    const body = (await request.json()) as { email: string };
    const members = room.members ?? [];
    const member = members.find((m) => m.email.toLowerCase() === body.email.toLowerCase());
    if (member) {
      member.nda_status = "signed";
      member.nda_signed_at = new Date().toISOString();
    }
    return new HttpResponse(null, { status: 204 });
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
  http.post("*/api/v1/public/links/:token", async ({ params, request }) => {
    const body = (await request.json().catch(() => ({}))) as {
      email?: string;
      email_code?: string;
      password?: string;
      nda_agreed?: boolean;
    };
    const token = params.token as string;
    const link = mockLinks.find((l) => l.shortUrl.endsWith(token)) ?? mockLinks[0];
    const extended = link as Link & {
      _requireEmailVerification?: boolean;
      _requirePassword?: boolean;
      _requireNDA?: boolean;
      _password?: string;
      _allowedEmails?: string[];
      _allowedDomains?: string[];
    };

    // The mock permissionType "email" corresponds to the legacy "email_required" type,
    // where the visitor must supply both email and code. Modern email verification uses
    // permissionType "public" + _requireEmailVerification and is code-only.
    const isLegacyEmailRequired = extended.permissionType === "email";
    const requiresEmailVerification =
      extended._requireEmailVerification || isLegacyEmailRequired || extended.permissionType === "nda";
    const requiresPassword = extended._requirePassword || extended.permissionType === "password";
    const requiresNda = extended._requireNDA || extended.permissionType === "nda";
    const hasWhitelist =
      (extended._allowedEmails && extended._allowedEmails.length > 0) ||
      (extended._allowedDomains && extended._allowedDomains.length > 0);
    // Email is required for legacy email_required, whitelist matching, or NDA records.
    // Modern email verification (code-only) should not ask for email.
    const requiresEmail = isLegacyEmailRequired || hasWhitelist || requiresNda;

    if (requiresEmail && !body.email) {
      return HttpResponse.json(
        { code: "requires_email", message: "email required", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 403 }
      );
    }
    if (requiresEmailVerification && !body.email_code) {
      return HttpResponse.json(
        { code: "requires_email_code", message: "email code required", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 403 }
      );
    }
    if (requiresEmailVerification && body.email_code !== "123456") {
      return HttpResponse.json(
        { code: "invalid_email_code", message: "invalid email code", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 401 }
      );
    }
    if (hasWhitelist) {
      const domain = body.email!.split("@")[1]?.toLowerCase();
      const allowed = [
        ...(extended._allowedEmails ?? []),
        ...(extended._allowedDomains ?? []),
      ].some((entry) => {
        const e = entry.trim().toLowerCase();
        return e === body.email!.toLowerCase() || (e.startsWith("@") && e.slice(1) === domain);
      });
      if (!allowed) {
        return HttpResponse.json(
          { code: "whitelist_denied", message: "email not in whitelist", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
          { status: 403 }
        );
      }
    }
    if (requiresPassword && !body.password) {
      return HttpResponse.json(
        { code: "requires_password", message: "password required", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 403 }
      );
    }
    if (requiresPassword && body.password !== extended._password) {
      return HttpResponse.json(
        { code: "invalid_password", message: "invalid password", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 401 }
      );
    }
    if (requiresNda && !body.nda_agreed) {
      return HttpResponse.json(
        { code: "nda_required", message: "nda agreement required", requiresEmail, requiresEmailVerification, requiresPassword, requiresNda },
        { status: 403 }
      );
    }

    const doc = mockDocuments.find((d) => d.id === link.documentId) ?? mockDocuments[0];
    const publicDocument = {
      id: doc.id,
      title: doc.title,
      pageCount: doc.pageCount,
      status: doc.status,
      sourceType: doc.fileType,
      fileSize: doc.fileSize,
    };
    return HttpResponse.json({
      link: {
        id: link.id,
        name: link.documentTitle,
        documentId: link.documentId,
        permissionType: link.permissionType ?? "public",
        downloadEnabled: true,
        watermarkEnabled: false,
      },
      document: publicDocument,
      documents: [publicDocument],
      visitorId: generateId("visitor"),
      requiresEmail,
      requiresEmailVerification,
      requiresPassword,
      requiresNda,
      sessionToken: "mock_session_token",
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

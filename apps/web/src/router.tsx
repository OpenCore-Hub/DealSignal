/* eslint-disable react-refresh/only-export-components */
import { Suspense, lazy } from "react";
import { createBrowserRouter, Navigate, Outlet, useRouteError } from "react-router";
import { useTranslation } from "react-i18next";
import { AppShell } from "@/components/layout/AppShell";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

const DashboardPage = lazy(() => import("@/routes/dashboard").then((m) => ({ default: m.DashboardPage })));
const DocumentsPage = lazy(() => import("@/routes/documents").then((m) => ({ default: m.DocumentsPage })));
const AgreementDocumentsPage = lazy(() => import("@/routes/agreement-documents").then((m) => ({ default: m.AgreementDocumentsPage })));
const DocumentDetailPage = lazy(() => import("@/routes/documents/detail").then((m) => ({ default: m.DocumentDetailPage })));
const UploadPage = lazy(() => import("@/routes/upload").then((m) => ({ default: m.UploadPage })));
const LinksPage = lazy(() => import("@/routes/links").then((m) => ({ default: m.LinksPage })));
const LinkDetailPage = lazy(() => import("@/routes/links/detail").then((m) => ({ default: m.LinkDetailPage })));
const NewLinkPage = lazy(() => import("@/routes/links/new").then((m) => ({ default: m.NewLinkPage })));
const DealRoomsPage = lazy(() => import("@/routes/deal-rooms").then((m) => ({ default: m.DealRoomsPage })));
const DealRoomDetailPage = lazy(() => import("@/routes/deal-rooms/detail").then((m) => ({ default: m.DealRoomDetailPage })));
const NewDealRoomPage = lazy(() => import("@/routes/deal-rooms/new").then((m) => ({ default: m.NewDealRoomPage })));
const ContactsPage = lazy(() => import("@/routes/contacts").then((m) => ({ default: m.ContactsPage })));
const ContactDetailPage = lazy(() => import("@/routes/contacts/detail").then((m) => ({ default: m.ContactDetailPage })));
const NewContactPage = lazy(() => import("@/routes/contacts/new").then((m) => ({ default: m.NewContactPage })));
const InsightsPage = lazy(() => import("@/routes/insights").then((m) => ({ default: m.InsightsPage })));
const InsightsOverviewPage = lazy(() => import("@/routes/insights/overview").then((m) => ({ default: m.InsightsOverviewPage })));
const InsightsPagesPage = lazy(() => import("@/routes/insights/pages").then((m) => ({ default: m.InsightsPagesPage })));
const InsightsSuggestionsPage = lazy(() => import("@/routes/insights/suggestions").then((m) => ({ default: m.InsightsSuggestionsPage })));
const SettingsPage = lazy(() => import("@/routes/settings").then((m) => ({ default: m.SettingsPage })));
const SettingsGeneralPage = lazy(() => import("@/routes/settings/general").then((m) => ({ default: m.SettingsGeneralPage })));
const SettingsBrandPage = lazy(() => import("@/routes/settings/brand").then((m) => ({ default: m.SettingsBrandPage })));
const SettingsMembersPage = lazy(() => import("@/routes/settings/members").then((m) => ({ default: m.SettingsMembersPage })));
const SettingsIntegrationsPage = lazy(() => import("@/routes/settings/integrations").then((m) => ({ default: m.SettingsIntegrationsPage })));
const SettingsBillingPage = lazy(() => import("@/routes/settings/billing").then((m) => ({ default: m.SettingsBillingPage })));
const SettingsSecurityPage = lazy(() => import("@/routes/settings/security").then((m) => ({ default: m.SettingsSecurityPage })));
const SettingsLanguagePage = lazy(() => import("@/routes/settings/language").then((m) => ({ default: m.SettingsLanguagePage })));
const ViewerPage = lazy(() => import("@/routes/viewer").then((m) => ({ default: m.ViewerPage })));
const PublicViewerPage = lazy(() => import("@/components/viewer/PublicViewerPage").then((m) => ({ default: m.PublicViewerPage })));
const PublicDealRoomPage = lazy(() => import("@/routes/deal-rooms/public").then((m) => ({ default: m.PublicDealRoomPage })));
const NotFoundPage = lazy(() => import("@/routes/not-found").then((m) => ({ default: m.NotFoundPage })));
const LoginPage = lazy(() => import("@/routes/login").then((m) => ({ default: m.LoginPage })));
const RegisterPage = lazy(() => import("@/routes/register").then((m) => ({ default: m.RegisterPage })));
const VerifyEmailPage = lazy(() => import("@/routes/verify-email").then((m) => ({ default: m.VerifyEmailPage })));
const WorkspacesPage = lazy(() => import("@/routes/workspaces").then((m) => ({ default: m.WorkspacesPage })));
const CreateWorkspacePage = lazy(() => import("@/routes/workspaces/new").then((m) => ({ default: m.CreateWorkspacePage })));

function PageLoader() {
  return (
    <div className="flex h-full items-center justify-center p-8">
      <div className="w-full max-w-md space-y-4">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-32 w-full" />
      </div>
    </div>
  );
}

function WorkspaceLayout() {
  return (
    <AppShell>
      <Suspense fallback={<PageLoader />}>
        <Outlet />
      </Suspense>
    </AppShell>
  );
}

function getAuthToken(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return localStorage.getItem("access_token");
  } catch {
    return null;
  }
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const token = getAuthToken();
  if (!token) {
    return <Navigate to={`/login?redirect=${encodeURIComponent(window.location.pathname + window.location.search)}`} replace />;
  }
  return <>{children}</>;
}

function RouteError() {
  const { t } = useTranslation("common");
  const error = useRouteError() as Error;
  return (
    <div className="flex min-h-[100dvh] flex-col items-center justify-center gap-4 p-6 text-center">
      <h1 className="text-h1 text-foreground">{t("error.title")}</h1>
      <p className="max-w-md text-body text-muted-foreground">
        {error?.message || t("error.pageLoadFailed")}
      </p>
      <div className="flex gap-3">
        <Button variant="outline" onClick={() => window.location.reload()}>
          {t("reload")}
        </Button>
        <Button onClick={() => window.location.href = "/"}>{t("backToHome")}</Button>
      </div>
    </div>
  );
}

export const router = createBrowserRouter([
  {
    path: "/",
    element: (
      <Suspense fallback={<PageLoader />}>
        <ProtectedRoute>
          <WorkspacesPage />
        </ProtectedRoute>
      </Suspense>
    ),
    errorElement: <RouteError />,
  },
  {
    path: "/login",
    element: (
      <Suspense fallback={<PageLoader />}>
        <LoginPage />
      </Suspense>
    ),
    errorElement: <RouteError />,
  },
  {
    path: "/register",
    element: (
      <Suspense fallback={<PageLoader />}>
        <RegisterPage />
      </Suspense>
    ),
    errorElement: <RouteError />,
  },
  {
    path: "/verify-email/:token",
    element: (
      <Suspense fallback={<PageLoader />}>
        <VerifyEmailPage />
      </Suspense>
    ),
    errorElement: <RouteError />,
  },
  {
    path: "/:workspaceSlug",
    element: (
      <ProtectedRoute>
        <WorkspaceLayout />
      </ProtectedRoute>
    ),
    errorElement: <RouteError />,
    children: [
      { index: true, element: <Navigate to="dashboard" replace /> },
      { path: "dashboard", element: <DashboardPage /> },
      { path: "documents", element: <DocumentsPage /> },
      { path: "documents/upload", element: <UploadPage /> },
      { path: "documents/:documentId", element: <DocumentDetailPage /> },
      { path: "agreement-documents", element: <AgreementDocumentsPage /> },
      { path: "links", element: <LinksPage /> },
      { path: "links/new", element: <NewLinkPage /> },
      { path: "links/:linkId", element: <LinkDetailPage /> },
      { path: "deal-rooms", element: <DealRoomsPage /> },
      { path: "deal-rooms/new", element: <NewDealRoomPage /> },
      { path: "deal-rooms/:roomId", element: <DealRoomDetailPage /> },
      { path: "contacts", element: <ContactsPage /> },
      { path: "contacts/new", element: <NewContactPage /> },
      { path: "contacts/:contactId", element: <ContactDetailPage /> },
      {
        path: "insights",
        element: <InsightsPage />,
        children: [
          { index: true, element: <Navigate to="overview" replace /> },
          { path: "overview", element: <InsightsOverviewPage /> },
          { path: "pages", element: <InsightsPagesPage /> },
          { path: "suggestions", element: <InsightsSuggestionsPage /> },
        ],
      },
      {
        path: "settings",
        element: <SettingsPage />,
        children: [
          { index: true, element: <Navigate to="general" replace /> },
          { path: "general", element: <SettingsGeneralPage /> },
          { path: "language", element: <SettingsLanguagePage /> },
          { path: "brand", element: <SettingsBrandPage /> },
          { path: "members", element: <SettingsMembersPage /> },
          { path: "integrations", element: <SettingsIntegrationsPage /> },
          { path: "billing", element: <SettingsBillingPage /> },
          { path: "security", element: <SettingsSecurityPage /> },
        ],
      },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
  { path: "/workspaces/new", element: <Suspense fallback={<PageLoader />}><CreateWorkspacePage /></Suspense> },
  { path: "/viewer/:documentId", element: <Suspense fallback={<PageLoader />}><ViewerPage /></Suspense> },
  { path: "/l/:token", element: <Suspense fallback={<PageLoader />}><PublicViewerPage /></Suspense> },
  { path: "/r/:slug", element: <Suspense fallback={<PageLoader />}><PublicDealRoomPage /></Suspense> },
]);

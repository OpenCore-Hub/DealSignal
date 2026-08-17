/* eslint-disable react-refresh/only-export-components */
import { Suspense, lazy, useEffect } from "react";
import { createBrowserRouter, Navigate, useOutlet, useParams } from "react-router";
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
const EditLinkPage = lazy(() => import("@/routes/links/edit").then((m) => ({ default: m.EditLinkPage })));
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
const InsightsAccessPage = lazy(() => import("@/routes/insights/access").then((m) => ({ default: m.InsightsAccessPage })));
const InsightsKeyPagesPage = lazy(() => import("@/routes/insights/key-pages").then((m) => ({ default: m.InsightsKeyPagesPage })));
const SettingsPage = lazy(() => import("@/routes/settings").then((m) => ({ default: m.SettingsPage })));
const RequireWorkspaceWrite = lazy(() =>
  import("@/components/auth/RequireWorkspaceWrite").then((m) => ({ default: m.RequireWorkspaceWrite })),
);
const SettingsGeneralPage = lazy(() => import("@/routes/settings/general").then((m) => ({ default: m.SettingsGeneralPage })));
const SettingsBrandPage = lazy(() => import("@/routes/settings/brand").then((m) => ({ default: m.SettingsBrandPage })));
const SettingsMembersPage = lazy(() => import("@/routes/settings/members").then((m) => ({ default: m.SettingsMembersPage })));
const SettingsIntegrationsPage = lazy(() => import("@/routes/settings/integrations").then((m) => ({ default: m.SettingsIntegrationsPage })));
const SettingsBillingPage = lazy(() => import("@/routes/settings/billing").then((m) => ({ default: m.SettingsBillingPage })));
const SettingsBillingPlansPage = lazy(() =>
  import("@/routes/settings/billing-plans").then((m) => ({ default: m.SettingsBillingPlansPage })),
);
const SettingsSecurityPage = lazy(() => import("@/routes/settings/security").then((m) => ({ default: m.SettingsSecurityPage })));
const SettingsCompliancePage = lazy(() => import("@/routes/settings/compliance").then((m) => ({ default: m.SettingsCompliancePage })));
const SettingsLanguagePage = lazy(() => import("@/routes/settings/language").then((m) => ({ default: m.SettingsLanguagePage })));
const ViewerPage = lazy(() => import("@/routes/viewer").then((m) => ({ default: m.ViewerPage })));
const PublicViewerPage = lazy(() => import("@/components/viewer/PublicViewerPage").then((m) => ({ default: m.PublicViewerPage })));

function DealRoomRedirect() {
	const { slug } = useParams<{ slug: string }>();
	useEffect(() => {
		if (!slug) return;
		fetch(`/api/v1/public/deal-rooms/${encodeURIComponent(slug)}/redirect`)
			.then((r) => {
				if (r.redirected) window.location.replace(r.url);
			})
			.catch(() => {});
	}, [slug]);
	return <PageLoader />;
}

const NotFoundPage = lazy(() => import("@/routes/not-found").then((m) => ({ default: m.NotFoundPage })));
const LoginPage = lazy(() => import("@/routes/login").then((m) => ({ default: m.LoginPage })));
const RegisterPage = lazy(() => import("@/routes/register").then((m) => ({ default: m.RegisterPage })));
const VerifyEmailPage = lazy(() => import("@/routes/verify-email").then((m) => ({ default: m.VerifyEmailPage })));
const CheckEmailPage = lazy(() => import("@/routes/check-email").then((m) => ({ default: m.CheckEmailPage })));
const ForgotPasswordPage = lazy(() => import("@/routes/forgot-password").then((m) => ({ default: m.ForgotPasswordPage })));
const ResetPasswordPage = lazy(() => import("@/routes/reset-password").then((m) => ({ default: m.ResetPasswordPage })));
const AcceptInvitationPage = lazy(() =>
  import("@/routes/accept-invitation").then((m) => ({ default: m.AcceptInvitationPage })),
);
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
  // Capture the matched route element at render time. Using <Outlet /> directly
  // inside <PageTransition> / <AnimatePresence> causes the exiting page to
  // re-render the current route while animating out, which can leave the entering
  // motion.div stuck at opacity:0 and produce a blank page on route changes.
  const element = useOutlet();
  return (
    <AppShell>
      <Suspense fallback={<PageLoader />}>
        {element}
      </Suspense>
    </AppShell>
  );
}

function hasAuthSession(): boolean {
  if (typeof window === "undefined") return false;
  try {
    return document.cookie.split(";").some((c) => c.trim().startsWith("auth_session="));
  } catch {
    return false;
  }
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  if (!hasAuthSession()) {
    return <Navigate to={`/login?redirect=${encodeURIComponent(window.location.pathname + window.location.search)}`} replace />;
  }
  return <>{children}</>;
}

function RouteError() {
  const { t } = useTranslation("common");
  return (
    <div className="flex min-h-[100dvh] flex-col items-center justify-center gap-4 p-6 text-center">
      <h1 className="text-h1 text-foreground">{t("error.title")}</h1>
      <p className="max-w-md text-body text-muted-foreground">
        {t("error.pageLoadFailed")}
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
    path: "/check-email",
    element: (
      <Suspense fallback={<PageLoader />}>
        <CheckEmailPage />
      </Suspense>
    ),
    errorElement: <RouteError />,
  },
  {
    path: "/forgot-password",
    element: (
      <Suspense fallback={<PageLoader />}>
        <ForgotPasswordPage />
      </Suspense>
    ),
    errorElement: <RouteError />,
  },
  {
    path: "/reset-password/:token",
    element: (
      <Suspense fallback={<PageLoader />}>
        <ResetPasswordPage />
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
    // Must be registered before /:workspaceSlug so invite links are not treated as workspace routes.
    path: "/invitations/:token/accept",
    element: (
      <Suspense fallback={<PageLoader />}>
        <AcceptInvitationPage />
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
      {
        path: "documents/upload",
        element: (
          <RequireWorkspaceWrite>
            <UploadPage />
          </RequireWorkspaceWrite>
        ),
      },
      { path: "documents/:documentId", element: <DocumentDetailPage /> },
      { path: "agreement-documents", element: <AgreementDocumentsPage /> },
      { path: "links", element: <LinksPage /> },
      {
        path: "links/new",
        element: (
          <RequireWorkspaceWrite>
            <NewLinkPage />
          </RequireWorkspaceWrite>
        ),
      },
      {
        path: "links/:id/edit",
        element: (
          <RequireWorkspaceWrite>
            <EditLinkPage />
          </RequireWorkspaceWrite>
        ),
      },
      { path: "links/:linkId", element: <LinkDetailPage /> },
      { path: "deal-rooms", element: <DealRoomsPage /> },
      {
        path: "deal-rooms/new",
        element: (
          <RequireWorkspaceWrite>
            <NewDealRoomPage />
          </RequireWorkspaceWrite>
        ),
      },
      { path: "deal-rooms/:roomId", element: <DealRoomDetailPage /> },
      { path: "contacts", element: <ContactsPage /> },
      {
        path: "contacts/new",
        element: (
          <RequireWorkspaceWrite>
            <NewContactPage />
          </RequireWorkspaceWrite>
        ),
      },
      { path: "contacts/:contactId", element: <ContactDetailPage /> },
      {
        path: "insights",
        element: <InsightsPage />,
        children: [
          { index: true, element: <Navigate to="overview" replace /> },
          { path: "overview", element: <InsightsOverviewPage /> },
          { path: "pages", element: <InsightsPagesPage /> },
          { path: "access", element: <InsightsAccessPage /> },
          { path: "key-pages", element: <InsightsKeyPagesPage /> },
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
          { path: "billing/plans", element: <SettingsBillingPlansPage /> },
          { path: "security", element: <SettingsSecurityPage /> },
          { path: "compliance", element: <SettingsCompliancePage /> },
        ],
      },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
  { path: "/workspaces/new", element: <Suspense fallback={<PageLoader />}><CreateWorkspacePage /></Suspense> },
  { path: "/viewer/:documentId", element: <Suspense fallback={<PageLoader />}><ViewerPage /></Suspense> },
  { path: "/l/:token", element: <Suspense fallback={<PageLoader />}><PublicViewerPage /></Suspense> },
  { path: "/r/:slug", element: <Suspense fallback={<PageLoader />}><DealRoomRedirect /></Suspense> },
]);

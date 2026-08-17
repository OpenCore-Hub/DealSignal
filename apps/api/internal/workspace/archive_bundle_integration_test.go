//go:build integration

package workspace_test

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func createReadyBillingDoc(t *testing.T, f *billingFixture, title string) db.CreateDocumentRow {
	t.Helper()
	doc, err := f.q.CreateDocument(f.ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		TenantID:    f.workspace.TenantID,
		WorkspaceID: f.workspace.ID,
		CreatedBy:   f.user.ID,
		Title:       title,
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "billing-key-" + uuid.NewString(),
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document %s: %v", title, err)
	}
	return doc
}

func archiveBillingDoc(t *testing.T, f *billingFixture, docID pgtype.UUID) {
	t.Helper()
	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))
	if err := uploadSvc.ArchiveDocument(
		f.ctx,
		uuid.UUID(f.workspace.ID.Bytes).String(),
		uuid.UUID(f.workspace.TenantID.Bytes).String(),
		uuid.UUID(docID.Bytes).String(),
	); err != nil {
		t.Fatalf("ArchiveDocument: %v", err)
	}
}

func reloadBillingLink(t *testing.T, f *billingFixture, id pgtype.UUID) db.Link {
	t.Helper()
	row, err := f.q.GetLinkByIDAndWorkspace(f.ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          id,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload link: %v", err)
	}
	return row
}

// Archiving one member of a multi-doc share must keep the link live, rebind
// primary when needed, and leave sibling page-view / dwell rows readable.
func TestBillingArchiveBundleMemberKeepsShareAndMetrics_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	primary := f.doc
	second := createReadyBillingDoc(t, f, "Bundle Second")
	third := createReadyBillingDoc(t, f, "Bundle Third")
	primaryID := uuid.UUID(primary.ID.Bytes).String()
	secondID := uuid.UUID(second.ID.Bytes).String()
	thirdID := uuid.UUID(third.ID.Bytes).String()

	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentIDs:    []string{primaryID, secondID, thirdID},
		Name:           "bundle-archive-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	for _, doc := range []db.CreateDocumentRow{primary, second} {
		if _, err := f.q.CreatePage(f.ctx, db.CreatePageParams{
			TenantID:       f.workspace.TenantID,
			WorkspaceID:    f.workspace.ID,
			DocumentID:     doc.ID,
			PageNumber:     1,
			ImageObjectKey: pgtype.Text{String: "page-" + uuid.UUID(doc.ID.Bytes).String(), Valid: true},
			Width:          pgtype.Int4{Int32: 800, Valid: true},
			Height:         pgtype.Int4{Int32: 1100, Valid: true},
			FileSize:       pgtype.Int8{Int64: 10, Valid: true},
			Title:          pgtype.Text{String: "p1", Valid: true},
		}); err != nil {
			t.Fatalf("create page: %v", err)
		}
	}
	if err := f.q.CreatePageView(f.ctx, db.CreatePageViewParams{
		TenantID:        f.workspace.TenantID,
		WorkspaceID:     f.workspace.ID,
		LinkID:          created.ID,
		VisitorID:       pgtype.Text{String: "visitor-bundle", Valid: true},
		PageNumber:      1,
		DurationSeconds: 12,
		Column7:         pgtype.Numeric{Int: nil, Exp: 0, NaN: false, Valid: false},
		DocumentID:      second.ID,
	}); err != nil {
		t.Fatalf("create page view: %v", err)
	}
	// Legacy NULL attribution belongs to the then-primary. Rebind must not
	// move it onto the next live member.
	if err := f.q.CreatePageView(f.ctx, db.CreatePageViewParams{
		TenantID:        f.workspace.TenantID,
		WorkspaceID:     f.workspace.ID,
		LinkID:          created.ID,
		VisitorID:       pgtype.Text{String: "visitor-legacy", Valid: true},
		PageNumber:      1,
		DurationSeconds: 8,
		Column7:         pgtype.Numeric{Int: nil, Exp: 0, NaN: false, Valid: false},
	}); err != nil {
		t.Fatalf("create legacy page view: %v", err)
	}

	assertBundleMetrics := func(t *testing.T) {
		t.Helper()
		metrics, err := f.q.GetLinkPageViewMetrics(f.ctx, created.ID)
		if err != nil {
			t.Fatalf("GetLinkPageViewMetrics: %v", err)
		}
		if metrics.TotalPageViews != 2 {
			t.Fatalf("link dwell total_page_views=%d want 2", metrics.TotalPageViews)
		}
		pages, err := f.q.GetPageAnalyticsByDocument(f.ctx, db.GetPageAnalyticsByDocumentParams{
			DocumentID:  second.ID,
			WorkspaceID: f.workspace.ID,
		})
		if err != nil {
			t.Fatalf("GetPageAnalyticsByDocument: %v", err)
		}
		if len(pages) != 1 || pages[0].ViewCount != 1 {
			t.Fatalf("sibling page analytics=%+v want 1 view (no legacy leak)", pages)
		}
	}
	assertBundleMetrics(t)

	// Non-primary archive: share stays live, primary unchanged.
	archiveBillingDoc(t, f, third.ID)
	row := reloadBillingLink(t, f, created.ID)
	if row.Status != "active" {
		t.Fatalf("after non-primary archive status=%q want active", row.Status)
	}
	if uuid.UUID(row.DocumentID.Bytes) != uuid.UUID(primary.ID.Bytes) {
		t.Fatalf("non-primary archive must not rebind primary")
	}
	assertBundleMetrics(t)

	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.LinksUsed != 1 {
		t.Fatalf("bundle with live members must keep 1 quota slot, used=%d", billing.LinksUsed)
	}

	// Primary archive: keep document_id so S1 NULL views stay on the original primary.
	archiveBillingDoc(t, f, primary.ID)
	row = reloadBillingLink(t, f, created.ID)
	if row.Status != "active" {
		t.Fatalf("after primary archive status=%q want active", row.Status)
	}
	if uuid.UUID(row.DocumentID.Bytes) != uuid.UUID(primary.ID.Bytes) {
		t.Fatalf("archive must not rebind primary, got %s", uuid.UUID(row.DocumentID.Bytes))
	}
	assertBundleMetrics(t)
	primaryPages, err := f.q.GetPageAnalyticsByDocument(f.ctx, db.GetPageAnalyticsByDocumentParams{
		DocumentID:  primary.ID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("GetPageAnalyticsByDocument primary: %v", err)
	}
	if len(primaryPages) != 1 || primaryPages[0].ViewCount != 1 {
		t.Fatalf("legacy NULL views must stay on archived primary, got %+v", primaryPages)
	}

	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after primary archive: %v", err)
	}
	if billing.LinksUsed != 1 {
		t.Fatalf("rebound bundle must keep quota, used=%d", billing.LinksUsed)
	}

	// Last live member: park and free inventory. Historical page_views stay.
	archiveBillingDoc(t, f, second.ID)
	row = reloadBillingLink(t, f, created.ID)
	if row.Status != "archived" {
		t.Fatalf("last member archive status=%q want archived", row.Status)
	}
	metrics, err := f.q.GetLinkPageViewMetrics(f.ctx, created.ID)
	if err != nil {
		t.Fatalf("GetLinkPageViewMetrics after park: %v", err)
	}
	if metrics.TotalPageViews != 2 {
		t.Fatalf("parking must not delete page_views, total=%d", metrics.TotalPageViews)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after last archive: %v", err)
	}
	if billing.LinksUsed != 0 {
		t.Fatalf("last member archive must free quota, used=%d", billing.LinksUsed)
	}

	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))
	if err := uploadSvc.UnarchiveDocument(
		f.ctx, wsID,
		uuid.UUID(f.workspace.TenantID.Bytes).String(),
		secondID,
	); err != nil {
		t.Fatalf("UnarchiveDocument: %v", err)
	}
	row = reloadBillingLink(t, f, created.ID)
	if row.Status != "archived" {
		t.Fatalf("unarchive must not auto-renew parked share, status=%q", row.Status)
	}
}

// Deleting a rebound (or original) bundle primary must keep the share when
// another live member remains — the same live-member rule as archive.
func TestBillingDeleteBundleMemberKeepsShare_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()
	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))

	primary := f.doc
	second := createReadyBillingDoc(t, f, "Delete Bundle Second")
	third := createReadyBillingDoc(t, f, "Delete Bundle Third")
	primaryID := uuid.UUID(primary.ID.Bytes).String()
	secondID := uuid.UUID(second.ID.Bytes).String()
	thirdID := uuid.UUID(third.ID.Bytes).String()

	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentIDs:    []string{primaryID, secondID, thirdID},
		Name:           "bundle-delete-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	impact, err := uploadSvc.GetDocumentDeleteImpact(f.ctx, wsID, primaryID)
	if err != nil {
		t.Fatalf("GetDocumentDeleteImpact: %v", err)
	}
	if impact.ActiveLinkCount != 1 {
		t.Fatalf("membership count=%d want 1", impact.ActiveLinkCount)
	}
	if impact.RevokedLinkCount != 0 {
		t.Fatalf("bundle with siblings must not revoke, revoked=%d", impact.RevokedLinkCount)
	}

	if err := uploadSvc.DeleteDocument(f.ctx, wsID, primaryID); err != nil {
		t.Fatalf("DeleteDocument primary: %v", err)
	}
	row := reloadBillingLink(t, f, created.ID)
	if row.Status != "active" {
		t.Fatalf("after primary delete status=%q want active", row.Status)
	}
	if uuid.UUID(row.DocumentID.Bytes) != uuid.UUID(primary.ID.Bytes) {
		t.Fatalf("delete must not rebind primary, got %s", uuid.UUID(row.DocumentID.Bytes))
	}

	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.LinksUsed != 1 {
		t.Fatalf("bundle with live members must keep quota, used=%d", billing.LinksUsed)
	}

	if err := uploadSvc.DeleteDocument(f.ctx, wsID, secondID); err != nil {
		t.Fatalf("DeleteDocument second: %v", err)
	}
	row = reloadBillingLink(t, f, created.ID)
	if row.Status != "active" {
		t.Fatalf("after second delete status=%q want active", row.Status)
	}

	lastImpact, err := uploadSvc.GetDocumentDeleteImpact(f.ctx, wsID, thirdID)
	if err != nil {
		t.Fatalf("GetDocumentDeleteImpact last: %v", err)
	}
	if lastImpact.RevokedLinkCount != 1 {
		t.Fatalf("last member must revoke, revoked=%d", lastImpact.RevokedLinkCount)
	}

	if err := uploadSvc.DeleteDocument(f.ctx, wsID, thirdID); err != nil {
		t.Fatalf("DeleteDocument last: %v", err)
	}
	row = reloadBillingLink(t, f, created.ID)
	if row.Status != "deleted" {
		t.Fatalf("last member delete status=%q want deleted", row.Status)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after last delete: %v", err)
	}
	if billing.LinksUsed != 0 {
		t.Fatalf("last member delete must free quota, used=%d", billing.LinksUsed)
	}
}

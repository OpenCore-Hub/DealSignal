package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Action describes a data-subject-right operation performed by a workspace admin.
type Action string

const (
	ActionExport    Action = "export"
	ActionAnonymize Action = "anonymize"
	ActionDelete    Action = "delete"
)

// Pool is the subset of pgxpool.Pool required by compliance operations.
type Pool interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
}

// Service implements GDPR/CCPA-style data subject rights for visitor PII.
type Service struct {
	queries *db.Queries
	pool    Pool
	cfg     *config.Config
}

// NewService creates a compliance service.
func NewService(queries *db.Queries, pool Pool, cfg *config.Config) *Service {
	return &Service{queries: queries, pool: pool, cfg: cfg}
}

// ExportVisitorData returns all visitor PII stored in the workspace.
func (s *Service) ExportVisitorData(ctx context.Context, workspaceID pgtype.UUID, actorUserID pgtype.UUID, email string) (map[string]any, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, fmt.Errorf("visitor_email is required")
	}

	accessLogs, err := s.queryAccessLogs(ctx, workspaceID, email)
	if err != nil {
		return nil, fmt.Errorf("export access logs: %w", err)
	}
	pageViews, err := s.queryPageViews(ctx, workspaceID, email)
	if err != nil {
		return nil, fmt.Errorf("export page views: %w", err)
	}
	securityEvents, err := s.querySecurityEvents(ctx, workspaceID, email)
	if err != nil {
		return nil, fmt.Errorf("export security events: %w", err)
	}
	ndaAgreements, err := s.queryNDAAgreements(ctx, workspaceID, email)
	if err != nil {
		return nil, fmt.Errorf("export nda agreements: %w", err)
	}
	roomNDAAgreements, err := s.queryRoomNDAAgreements(ctx, workspaceID, email)
	if err != nil {
		return nil, fmt.Errorf("export room nda agreements: %w", err)
	}
	questions, err := s.queryVisitorQuestions(ctx, workspaceID, email)
	if err != nil {
		return nil, fmt.Errorf("export questions: %w", err)
	}
	fileRequests, err := s.queryFileRequests(ctx, workspaceID, email)
	if err != nil {
		return nil, fmt.Errorf("export file requests: %w", err)
	}
	uploads, err := s.queryUploadedFiles(ctx, workspaceID, email)
	if err != nil {
		return nil, fmt.Errorf("export uploads: %w", err)
	}
	contacts, err := s.queryContacts(ctx, workspaceID, email)
	if err != nil {
		return nil, fmt.Errorf("export contacts: %w", err)
	}

	result := map[string]any{
		"visitor_email":       email,
		"exported_at":         time.Now().UTC().Format(time.RFC3339),
		"access_logs":         accessLogs,
		"page_views":          pageViews,
		"security_events":     securityEvents,
		"nda_agreements":      ndaAgreements,
		"room_nda_agreements": roomNDAAgreements,
		"questions":           questions,
		"file_requests":       fileRequests,
		"uploaded_files":      uploads,
		"contacts":            contacts,
	}
	if err := s.logAudit(ctx, workspaceID, actorUserID, ActionExport, email, map[string]int64{"total": 0}); err != nil {
		logger.ErrorCtx(ctx, "compliance audit log failed", err)
	}
	return result, nil
}

// AnonymizeVisitorData pseudonymizes visitor PII while preserving aggregate analytics.
func (s *Service) AnonymizeVisitorData(ctx context.Context, workspaceID pgtype.UUID, actorUserID pgtype.UUID, email string) (map[string]int64, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, fmt.Errorf("visitor_email is required")
	}

	anon := s.anonLabel(email)
	summary := make(map[string]int64)

	summary["access_logs"], _ = s.exec(ctx,
		"UPDATE access_logs SET visitor_email = $1, ip = NULL WHERE workspace_id = $2 AND visitor_email ILIKE $3",
		anon, workspaceID, email)

	summary["security_events"], _ = s.exec(ctx,
		"UPDATE security_events SET email = $1, ip = NULL WHERE workspace_id = $2 AND email ILIKE $3",
		anon, workspaceID, email)

	summary["nda_agreements"], _ = s.exec(ctx,
		"UPDATE link_nda_agreements SET email = $1, ip = NULL, user_agent = NULL WHERE workspace_id = $2 AND email ILIKE $3",
		anon, workspaceID, email)

	summary["room_nda_agreements"], _ = s.exec(ctx,
		"UPDATE room_nda_agreements SET email = $1, ip = NULL, user_agent = NULL WHERE email ILIKE $2 AND room_id IN (SELECT id FROM deal_rooms WHERE workspace_id = $3)",
		anon, email, workspaceID)

	summary["questions"], _ = s.exec(ctx,
		"UPDATE link_visitor_questions SET visitor_email = $1 WHERE workspace_id = $2 AND visitor_email ILIKE $3",
		anon, workspaceID, email)

	summary["file_requests"], _ = s.exec(ctx,
		"UPDATE link_file_requests SET visitor_email = $1 WHERE workspace_id = $2 AND visitor_email ILIKE $3",
		anon, workspaceID, email)

	summary["uploaded_files"], _ = s.exec(ctx,
		"UPDATE link_uploaded_files SET uploader_email = $1, uploader_ip = NULL, uploader_user_agent = NULL WHERE workspace_id = $2 AND uploader_email ILIKE $3",
		anon, workspaceID, email)

	summary["contacts"], _ = s.exec(ctx,
		"UPDATE contacts SET email = $1, name = 'Anonymous' WHERE workspace_id = $2 AND email ILIKE $3",
		anon, workspaceID, email)

	var total int64
	for _, v := range summary {
		total += v
	}
	summary["total"] = total

	if err := s.logAudit(ctx, workspaceID, actorUserID, ActionAnonymize, email, summary); err != nil {
		logger.ErrorCtx(ctx, "compliance audit log failed", err)
	}
	return summary, nil
}

// DeleteVisitorData deletes identifiable visitor records from the workspace.
func (s *Service) DeleteVisitorData(ctx context.Context, workspaceID pgtype.UUID, actorUserID pgtype.UUID, email string) (map[string]int64, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, fmt.Errorf("visitor_email is required")
	}

	summary := make(map[string]int64)

	summary["access_logs"], _ = s.exec(ctx,
		"DELETE FROM access_logs WHERE workspace_id = $1 AND visitor_email ILIKE $2",
		workspaceID, email)

	summary["security_events"], _ = s.exec(ctx,
		"DELETE FROM security_events WHERE workspace_id = $1 AND email ILIKE $2",
		workspaceID, email)

	summary["nda_agreements"], _ = s.exec(ctx,
		"DELETE FROM link_nda_agreements WHERE workspace_id = $1 AND email ILIKE $2",
		workspaceID, email)

	summary["room_nda_agreements"], _ = s.exec(ctx,
		"DELETE FROM room_nda_agreements WHERE email ILIKE $1 AND room_id IN (SELECT id FROM deal_rooms WHERE workspace_id = $2)",
		email, workspaceID)

	summary["questions"], _ = s.exec(ctx,
		"DELETE FROM link_visitor_questions WHERE workspace_id = $1 AND visitor_email ILIKE $2",
		workspaceID, email)

	summary["file_requests"], _ = s.exec(ctx,
		"DELETE FROM link_file_requests WHERE workspace_id = $1 AND visitor_email ILIKE $2",
		workspaceID, email)

	summary["uploaded_files"], _ = s.exec(ctx,
		"DELETE FROM link_uploaded_files WHERE workspace_id = $1 AND uploader_email ILIKE $2",
		workspaceID, email)

	summary["contacts"], _ = s.exec(ctx,
		"DELETE FROM contacts WHERE workspace_id = $1 AND email ILIKE $2",
		workspaceID, email)

	var total int64
	for _, v := range summary {
		total += v
	}
	summary["total"] = total

	if err := s.logAudit(ctx, workspaceID, actorUserID, ActionDelete, email, summary); err != nil {
		logger.ErrorCtx(ctx, "compliance audit log failed", err)
	}
	return summary, nil
}

func (s *Service) anonLabel(email string) string {
	return "anonymous-" + ShortHashIP(s.cfg.IPHashKey, email, 8)
}

func (s *Service) exec(ctx context.Context, sql string, args ...any) (int64, error) {
	result, err := s.pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (s *Service) queryRows(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		descs := rows.FieldDescriptions()
		row := make(map[string]any, len(descs))
		for i, d := range descs {
			row[string(d.Name)] = vals[i]
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) queryAccessLogs(ctx context.Context, workspaceID pgtype.UUID, email string) ([]map[string]any, error) {
	return s.queryRows(ctx,
		"SELECT id, link_id, visitor_id, visitor_email, event_type, ip, user_agent, created_at FROM access_logs WHERE workspace_id = $1 AND visitor_email ILIKE $2 ORDER BY created_at DESC",
		workspaceID, email)
}

func (s *Service) queryPageViews(ctx context.Context, workspaceID pgtype.UUID, email string) ([]map[string]any, error) {
	return s.queryRows(ctx, `
		SELECT pv.id, pv.link_id, pv.visitor_id, pv.page_number, pv.duration_seconds, pv.scroll_depth, pv.created_at
		FROM page_views pv
		JOIN access_logs al ON al.visitor_id = pv.visitor_id AND al.link_id = pv.link_id
		WHERE pv.workspace_id = $1 AND al.visitor_email ILIKE $2
		ORDER BY pv.created_at DESC`,
		workspaceID, email)
}

func (s *Service) querySecurityEvents(ctx context.Context, workspaceID pgtype.UUID, email string) ([]map[string]any, error) {
	return s.queryRows(ctx,
		"SELECT id, link_id, visitor_id, email, event_type, ip, user_agent, reason, created_at FROM security_events WHERE workspace_id = $1 AND email ILIKE $2 ORDER BY created_at DESC",
		workspaceID, email)
}

func (s *Service) queryNDAAgreements(ctx context.Context, workspaceID pgtype.UUID, email string) ([]map[string]any, error) {
	return s.queryRows(ctx,
		"SELECT id, link_id, visitor_id, email, ip, user_agent, signed_at FROM link_nda_agreements WHERE workspace_id = $1 AND email ILIKE $2 ORDER BY signed_at DESC",
		workspaceID, email)
}

func (s *Service) queryRoomNDAAgreements(ctx context.Context, workspaceID pgtype.UUID, email string) ([]map[string]any, error) {
	return s.queryRows(ctx, `
		SELECT rna.id, rna.room_id, rna.email, rna.ip, rna.user_agent, rna.agreed_at
		FROM room_nda_agreements rna
		JOIN deal_rooms dr ON dr.id = rna.room_id
		WHERE dr.workspace_id = $1 AND rna.email ILIKE $2
		ORDER BY rna.agreed_at DESC`,
		workspaceID, email)
}

func (s *Service) queryVisitorQuestions(ctx context.Context, workspaceID pgtype.UUID, email string) ([]map[string]any, error) {
	return s.queryRows(ctx,
		"SELECT id, link_id, visitor_id, visitor_email, question, answer, status, created_at, updated_at FROM link_visitor_questions WHERE workspace_id = $1 AND visitor_email ILIKE $2 ORDER BY created_at DESC",
		workspaceID, email)
}

func (s *Service) queryFileRequests(ctx context.Context, workspaceID pgtype.UUID, email string) ([]map[string]any, error) {
	return s.queryRows(ctx,
		"SELECT id, link_id, visitor_id, visitor_email, message, status, created_at, updated_at FROM link_file_requests WHERE workspace_id = $1 AND visitor_email ILIKE $2 ORDER BY created_at DESC",
		workspaceID, email)
}

func (s *Service) queryUploadedFiles(ctx context.Context, workspaceID pgtype.UUID, email string) ([]map[string]any, error) {
	return s.queryRows(ctx,
		"SELECT id, link_id, document_id, original_filename, storage_key, file_size, mime_type, uploader_email, uploader_visitor_id, uploader_ip, status, created_at FROM link_uploaded_files WHERE workspace_id = $1 AND uploader_email ILIKE $2 ORDER BY created_at DESC",
		workspaceID, email)
}

func (s *Service) queryContacts(ctx context.Context, workspaceID pgtype.UUID, email string) ([]map[string]any, error) {
	return s.queryRows(ctx,
		"SELECT id, email, name, created_at FROM contacts WHERE workspace_id = $1 AND email ILIKE $2",
		workspaceID, email)
}

func (s *Service) logAudit(ctx context.Context, workspaceID, actorUserID pgtype.UUID, action Action, email string, summary map[string]int64) error {
	payload, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO compliance_audit_log (tenant_id, workspace_id, actor_user_id, action, visitor_email, payload)
		 SELECT w.tenant_id, $1::uuid, $2, $3, $4, $5 FROM workspaces w WHERE w.id = $1`,
		workspaceID, actorUserID, string(action), email, payload)
	return err
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

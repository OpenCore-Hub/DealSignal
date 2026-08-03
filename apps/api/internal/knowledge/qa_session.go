package knowledge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const knowledgeQATitleMaxRunes = 80

const (
	knowledgeQASessionListDefaultLimit = 20
	knowledgeQASessionListMaxLimit     = 50
)

// SessionQueryRequest is the lazy-create session query body.
type SessionQueryRequest struct {
	SessionID string `json:"sessionId"`
	Query     string `json:"query"`
	Answer    bool   `json:"answer"`
	TopK      int    `json:"top_k"`
}

// CreateSessionRequest is the optional body for explicit session create.
type CreateSessionRequest struct {
	Title string `json:"title"`
}

// QASession is the API shape for a knowledge Q&A session.
type QASession struct {
	ID         string     `json:"id"`
	RoomID     string     `json:"roomId"`
	Title      string     `json:"title,omitempty"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	LastTurnAt *time.Time `json:"lastTurnAt,omitempty"`
	TurnCount  int        `json:"turnCount,omitempty"`
}

// QASessionSummary is a list row with preview fields for A.1 history.
type QASessionSummary struct {
	QASession
	QuestionPreview string `json:"questionPreview,omitempty"`
}

// SessionListResponse is a keyset page of session summaries.
type SessionListResponse struct {
	Items      []QASessionSummary `json:"items"`
	NextCursor string             `json:"nextCursor,omitempty"`
}

// QATurn is one audited research turn.
type QATurn struct {
	ID           string      `json:"id"`
	SessionID    string      `json:"sessionId"`
	Sequence     int         `json:"sequence"`
	Question     string      `json:"question"`
	Answer       string      `json:"answer,omitempty"`
	Refused      bool        `json:"refused"`
	ResultStatus string      `json:"resultStatus"`
	Hits         []QueryHit  `json:"hits"`
	Mode         string      `json:"mode,omitempty"`
	TopK         int         `json:"topK,omitempty"`
	ErrorSummary string      `json:"errorSummary,omitempty"`
	CreatedAt    time.Time   `json:"createdAt"`
	Feedback     *QAFeedback `json:"feedback,omitempty"`
}

// SessionDetail is a session plus ordered turns.
type SessionDetail struct {
	Session QASession `json:"session"`
	Turns   []QATurn  `json:"turns"`
}

// SessionQueryResponse is the product query path result.
type SessionQueryResponse struct {
	SessionID string     `json:"sessionId"`
	Turn      QATurn     `json:"turn"`
	Query     string     `json:"query"`
	Mode      string     `json:"mode"`
	Answer    string     `json:"answer,omitempty"`
	Results   []QueryHit `json:"results"`
}

// QueryWithSession runs knowledge Query and appends an audit turn (JSON transport).
// Empty sessionId creates a new active session (first question).
func (s *Service) QueryWithSession(
	ctx context.Context,
	roomID, workspaceID, userID string,
	req SessionQueryRequest,
) (SessionQueryResponse, error) {
	return s.queryWithSession(ctx, roomID, workspaceID, userID, req, "json")
}

// runSessionQuery implements sessionQueryRunner for the HTTP handler.
func (s *Service) runSessionQuery(
	ctx context.Context,
	roomID, workspaceID, userID string,
	req SessionQueryRequest,
	transport string,
) (SessionQueryResponse, error) {
	return s.queryWithSession(ctx, roomID, workspaceID, userID, req, transport)
}

// queryWithSession is the shared audit path for JSON and SSE transports.
func (s *Service) queryWithSession(
	ctx context.Context,
	roomID, workspaceID, userID string,
	req SessionQueryRequest,
	transport string,
) (SessionQueryResponse, error) {
	started := time.Now()
	if !s.Enabled() {
		return SessionQueryResponse{}, ErrUnavailable
	}
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return SessionQueryResponse{}, err
	}

	q := strings.TrimSpace(req.Query)
	if q == "" {
		return SessionQueryResponse{}, fmt.Errorf("query is required")
	}
	if err := s.enforceAnswersQuota(ctx, workspaceID); err != nil {
		return SessionQueryResponse{}, err
	}

	session, err := s.resolveWritableSession(ctx, roomID, workspaceID, userID, req.SessionID, q)
	if err != nil {
		return SessionQueryResponse{}, err
	}

	queryReq := QueryRequest{Query: q, Answer: req.Answer, TopK: req.TopK}
	res, qerr := s.Query(ctx, roomID, workspaceID, userID, queryReq)

	// Client Stop / disconnect cancels the request ctx. Persist the audit turn
	// anyway so the desk can hydrate (P5) — same pattern as assistant/link.
	auditCtx, auditCancel := auditWriteContext(ctx)
	defer auditCancel()
	turn, err := s.appendTurn(auditCtx, session, roomID, workspaceID, userID, q, queryReq, res, qerr)
	if err != nil {
		return SessionQueryResponse{}, err
	}

	results := turn.Hits
	if results == nil {
		results = []QueryHit{}
	}
	// Always 200-path when the audit turn was written — UI reads resultStatus/errorSummary.
	// Upstream unavailability is reflected on the turn, not as a dropped response.
	_ = qerr
	recordKnowledgeQATurn(turn.ResultStatus, transport, started)
	return SessionQueryResponse{
		SessionID: turn.SessionID,
		Turn:      turn,
		Query:     q,
		Mode:      turn.Mode,
		Answer:    turn.Answer,
		Results:   results,
	}, nil
}

// ListSessions returns a keyset page of room session summaries (newest first).
func (s *Service) ListSessions(
	ctx context.Context,
	roomID, workspaceID, userID string,
	limit int,
	cursor string,
) (SessionListResponse, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return SessionListResponse{}, err
	}
	if limit <= 0 {
		limit = knowledgeQASessionListDefaultLimit
	}
	if limit > knowledgeQASessionListMaxLimit {
		limit = knowledgeQASessionListMaxLimit
	}

	params := db.ListKnowledgeQASessionSummariesForRoomParams{
		RoomID:    pgUUID(roomID),
		HasCursor: false,
		PageLimit: int32(limit + 1), // fetch one extra to detect a next page
	}
	if strings.TrimSpace(cursor) != "" {
		at, id, err := decodeSessionListCursor(cursor)
		if err != nil {
			return SessionListResponse{}, fmt.Errorf("%w: invalid cursor", ErrInvalidInput)
		}
		params.HasCursor = true
		params.CursorAt = pgtype.Timestamptz{Time: at.UTC(), Valid: true}
		params.CursorID = pgUUID(id.String())
	}

	rows, err := s.queries.ListKnowledgeQASessionSummariesForRoom(ctx, params)
	if err != nil {
		return SessionListResponse{}, err
	}

	out := SessionListResponse{Items: make([]QASessionSummary, 0, len(rows))}
	for i, row := range rows {
		if i >= limit {
			last := out.Items[len(out.Items)-1]
			sortAt := last.CreatedAt
			if last.LastTurnAt != nil {
				sortAt = *last.LastTurnAt
			}
			sid, err := uuid.Parse(last.ID)
			if err != nil {
				return SessionListResponse{}, err
			}
			out.NextCursor = encodeSessionListCursor(sortAt, sid)
			break
		}
		out.Items = append(out.Items, mapQASessionSummary(row))
	}
	return out, nil
}

// CreateSession closes any active room sessions and creates a fresh active session.
// Product UI prefers lazy-create on first ask; this endpoint supports explicit create.
func (s *Service) CreateSession(
	ctx context.Context,
	roomID, workspaceID, userID string,
	req CreateSessionRequest,
) (QASession, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return QASession{}, err
	}
	title := truncateRunes(req.Title, knowledgeQATitleMaxRunes)
	row, err := s.createSessionClosingActives(ctx, roomID, workspaceID, userID, title)
	if err != nil {
		return QASession{}, err
	}
	return mapQASession(row, 0), nil
}

// GetActiveSession returns the newest active session with turns, or ErrNotFound.
func (s *Service) GetActiveSession(ctx context.Context, roomID, workspaceID, userID string) (SessionDetail, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return SessionDetail{}, err
	}
	row, err := s.queries.GetActiveKnowledgeQASessionForRoom(ctx, pgUUID(roomID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SessionDetail{}, ErrNotFound
		}
		return SessionDetail{}, err
	}
	return s.loadSessionDetail(ctx, row, userID)
}

// GetSession returns a session by id for the room.
func (s *Service) GetSession(ctx context.Context, roomID, workspaceID, userID, sessionID string) (SessionDetail, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return SessionDetail{}, err
	}
	row, err := s.queries.GetKnowledgeQASession(ctx, db.GetKnowledgeQASessionParams{
		ID:     pgUUID(sessionID),
		RoomID: pgUUID(roomID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SessionDetail{}, ErrNotFound
		}
		return SessionDetail{}, err
	}
	return s.loadSessionDetail(ctx, row, userID)
}

// CloseSession marks active sessions in the room closed (新会话).
// Closes all actives so orphan actives cannot remain after a successful close.
func (s *Service) CloseSession(ctx context.Context, roomID, workspaceID, userID, sessionID string) (QASession, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return QASession{}, err
	}
	closeAndLoad := func(q *db.Queries) (db.KnowledgeQaSession, error) {
		if err := q.CloseActiveKnowledgeQASessionsForRoom(ctx, pgUUID(roomID)); err != nil {
			return db.KnowledgeQaSession{}, err
		}
		row, err := q.GetKnowledgeQASession(ctx, db.GetKnowledgeQASessionParams{
			ID:     pgUUID(sessionID),
			RoomID: pgUUID(roomID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.KnowledgeQaSession{}, ErrNotFound
			}
			return db.KnowledgeQaSession{}, err
		}
		return row, nil
	}
	if s.pool == nil {
		row, err := closeAndLoad(s.queries)
		if err != nil {
			return QASession{}, err
		}
		return mapQASession(row, 0), nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return QASession{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row, err := closeAndLoad(s.queries.WithTx(tx))
	if err != nil {
		return QASession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return QASession{}, err
	}
	return mapQASession(row, 0), nil
}

func (s *Service) resolveWritableSession(
	ctx context.Context,
	roomID, workspaceID, userID, sessionID, firstQuestion string,
) (db.KnowledgeQaSession, error) {
	sid := strings.TrimSpace(sessionID)
	if sid != "" {
		row, err := s.queries.GetKnowledgeQASession(ctx, db.GetKnowledgeQASessionParams{
			ID:     pgUUID(sid),
			RoomID: pgUUID(roomID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.KnowledgeQaSession{}, ErrNotFound
			}
			return db.KnowledgeQaSession{}, err
		}
		if row.Status == "active" {
			return row, nil
		}
		// Closed session → create a fresh one.
	}
	return s.createSessionClosingActives(ctx, roomID, workspaceID, userID, firstQuestion)
}

func (s *Service) createSessionClosingActives(
	ctx context.Context,
	roomID, workspaceID, userID, firstQuestion string,
) (db.KnowledgeQaSession, error) {
	title := truncateRunes(firstQuestion, knowledgeQATitleMaxRunes)
	params := db.CreateKnowledgeQASessionParams{
		WorkspaceID: pgUUID(workspaceID),
		RoomID:      pgUUID(roomID),
		CreatedBy:   pgUUID(userID),
		Title:       pgtype.Text{String: title, Valid: title != ""},
	}
	if s.pool == nil {
		_ = s.queries.CloseActiveKnowledgeQASessionsForRoom(ctx, pgUUID(roomID))
		return s.queries.CreateKnowledgeQASession(ctx, params)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.KnowledgeQaSession{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)
	if err := qtx.CloseActiveKnowledgeQASessionsForRoom(ctx, pgUUID(roomID)); err != nil {
		return db.KnowledgeQaSession{}, err
	}
	row, err := qtx.CreateKnowledgeQASession(ctx, params)
	if err != nil {
		return db.KnowledgeQaSession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.KnowledgeQaSession{}, err
	}
	return row, nil
}

func (s *Service) appendTurn(
	ctx context.Context,
	session db.KnowledgeQaSession,
	roomID, workspaceID, userID, question string,
	req QueryRequest,
	res QueryResponse,
	qerr error,
) (QATurn, error) {
	refused := false
	status := "answered"
	hits := res.Results
	answer := res.Answer
	errCode := ""
	mode := res.Mode
	topK := req.TopK
	if topK <= 0 {
		topK = s.cfg.DefaultTopK
	}

	if qerr != nil {
		status = "error"
		hits = []QueryHit{}
		answer = ""
		errCode = classifyQueryErrorCode(qerr)
	} else {
		refused, status = classifyTurnResult(answer, len(hits))
		if refused {
			hits = []QueryHit{}
		}
	}
	if hits == nil {
		hits = []QueryHit{}
	}
	hitsJSON, err := json.Marshal(hits)
	if err != nil {
		return QATurn{}, err
	}
	snapshot, _ := json.Marshal(s.corpusSnapshot(ctx, roomID))

	write := func(q *db.Queries) (db.KnowledgeQaTurn, error) {
		if _, err := q.LockKnowledgeQASession(ctx, session.ID); err != nil {
			return db.KnowledgeQaTurn{}, err
		}
		seqRow, err := q.NextKnowledgeQATurnSequence(ctx, session.ID)
		if err != nil {
			return db.KnowledgeQaTurn{}, err
		}
		row, err := q.CreateKnowledgeQATurn(ctx, db.CreateKnowledgeQATurnParams{
			SessionID:            session.ID,
			RoomID:               pgUUID(roomID),
			WorkspaceID:          pgUUID(workspaceID),
			Sequence:             seqRow + 1,
			Question:             question,
			Answer:               pgtype.Text{String: answer, Valid: answer != ""},
			Refused:              refused,
			ResultStatus:         status,
			CorpusStatusSnapshot: snapshot,
			Hits:                 hitsJSON,
			Mode:                 pgtype.Text{String: mode, Valid: mode != ""},
			TopK:                 pgtype.Int4{Int32: int32(topK), Valid: topK > 0},
			ErrorSummary:         pgtype.Text{String: errCode, Valid: errCode != ""},
			CreatedBy:            pgUUID(userID),
		})
		if err != nil {
			return db.KnowledgeQaTurn{}, err
		}
		title := truncateRunes(question, knowledgeQATitleMaxRunes)
		if err := q.TouchKnowledgeQASessionAfterTurn(ctx, db.TouchKnowledgeQASessionAfterTurnParams{
			ID:            session.ID,
			LastTurnAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			TitleFallback: pgtype.Text{String: title, Valid: title != ""},
		}); err != nil {
			return db.KnowledgeQaTurn{}, err
		}
		return row, nil
	}

	if s.pool == nil {
		row, err := write(s.queries)
		if err != nil {
			return QATurn{}, err
		}
		return mapQATurn(row), nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return QATurn{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row, err := write(s.queries.WithTx(tx))
	if err != nil {
		return QATurn{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return QATurn{}, err
	}
	return mapQATurn(row), nil
}

const knowledgeQAAuditWriteTimeout = 10 * time.Second

// auditWriteContext survives parent cancel (SSE abort) with a bounded write window.
func auditWriteContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), knowledgeQAAuditWriteTimeout)
}

func classifyQueryErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrUnavailable):
		return "knowledge_unavailable"
	case errors.Is(err, ErrForbidden):
		return "forbidden"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, context.Canceled):
		return "client_cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "query_timeout"
	default:
		return "query_failed"
	}
}

func (s *Service) corpusSnapshot(ctx context.Context, roomID string) map[string]any {
	out := map[string]any{"status": "unknown"}
	corpus, err := s.queries.GetDealRoomRagCorpus(ctx, pgUUID(roomID))
	if err != nil {
		return out
	}
	out["status"] = corpus.Status
	rows, err := s.queries.ListDealRoomRagDocuments(ctx, pgUUID(roomID))
	if err != nil {
		return out
	}
	synced, total := 0, 0
	for _, r := range rows {
		if r.Status == "deleted" {
			continue
		}
		total++
		if r.Status == "synced" {
			synced++
		}
	}
	out["synced"] = synced
	out["total"] = total
	return out
}

func (s *Service) loadSessionDetail(ctx context.Context, row db.KnowledgeQaSession, userID string) (SessionDetail, error) {
	turns, err := s.queries.ListKnowledgeQATurnsForSession(ctx, row.ID)
	if err != nil {
		return SessionDetail{}, err
	}
	out := make([]QATurn, 0, len(turns))
	for _, t := range turns {
		out = append(out, mapQATurn(t))
	}
	out, err = attachFeedbackToTurns(ctx, s.queries, row.ID, userID, out)
	if err != nil {
		return SessionDetail{}, err
	}
	return SessionDetail{
		Session: mapQASession(row, len(out)),
		Turns:   out,
	}, nil
}

func mapQASession(row db.KnowledgeQaSession, turnCount int) QASession {
	s := QASession{
		ID:        uuid.UUID(row.ID.Bytes).String(),
		RoomID:    uuid.UUID(row.RoomID.Bytes).String(),
		Title:     textOrEmpty(row.Title),
		Status:    row.Status,
		TurnCount: turnCount,
	}
	if row.CreatedAt.Valid {
		s.CreatedAt = row.CreatedAt.Time.UTC()
	}
	if row.UpdatedAt.Valid {
		s.UpdatedAt = row.UpdatedAt.Time.UTC()
	}
	if row.LastTurnAt.Valid {
		t := row.LastTurnAt.Time.UTC()
		s.LastTurnAt = &t
	}
	return s
}

func mapQASessionSummary(row db.ListKnowledgeQASessionSummariesForRoomRow) QASessionSummary {
	base := db.KnowledgeQaSession{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		RoomID:      row.RoomID,
		CreatedBy:   row.CreatedBy,
		Title:       row.Title,
		Status:      row.Status,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		LastTurnAt:  row.LastTurnAt,
	}
	return QASessionSummary{
		QASession:       mapQASession(base, int(row.TurnCount)),
		QuestionPreview: strings.TrimSpace(row.QuestionPreview),
	}
}

func encodeSessionListCursor(at time.Time, id uuid.UUID) string {
	raw := fmt.Sprintf("%d|%s", at.UTC().UnixNano(), id.String())
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeSessionListCursor(cursor string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("bad cursor shape")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	return time.Unix(0, nanos).UTC(), id, nil
}

func mapQATurn(row db.KnowledgeQaTurn) QATurn {
	t := QATurn{
		ID:           uuid.UUID(row.ID.Bytes).String(),
		SessionID:    uuid.UUID(row.SessionID.Bytes).String(),
		Sequence:     int(row.Sequence),
		Question:     row.Question,
		Answer:       textOrEmpty(row.Answer),
		Refused:      row.Refused,
		ResultStatus: row.ResultStatus,
		Mode:         textOrEmpty(row.Mode),
		ErrorSummary: textOrEmpty(row.ErrorSummary),
		Hits:         []QueryHit{},
	}
	if row.TopK.Valid {
		t.TopK = int(row.TopK.Int32)
	}
	if row.CreatedAt.Valid {
		t.CreatedAt = row.CreatedAt.Time.UTC()
	}
	if len(row.Hits) > 0 {
		var hits []QueryHit
		if err := json.Unmarshal(row.Hits, &hits); err == nil && hits != nil {
			t.Hits = hits
		}
	}
	return t
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

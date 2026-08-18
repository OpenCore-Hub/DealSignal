package link

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	routeReasonPinnedFAQ   = "pinned_faq"
	maxPinnedFAQAliases    = 10
	maxPinnedFAQAliasChars = 500
)

var (
	ErrAskFAQAliasesInvalid = errors.New("faq aliases are invalid")
	ErrAskFAQAliasConflict  = errors.New("faq alias conflicts with another pinned question")
)

func pinnedFAQAliases(t db.LinkAskTurn) []string {
	if len(t.PinnedFaqAliases) == 0 {
		return nil
	}
	out := make([]string, 0, len(t.PinnedFaqAliases))
	for _, alias := range t.PinnedFaqAliases {
		if trimmed := strings.TrimSpace(alias); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func faqMatchKeys(t db.LinkAskTurn) []string {
	seen := make(map[string]struct{})
	var keys []string
	add := func(raw string) {
		key := normalizeAskQuestionKey(raw)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	add(t.Question)
	for _, alias := range t.PinnedFaqAliases {
		add(alias)
	}
	return keys
}

func faqKeysOverlap(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, key := range a {
		set[key] = struct{}{}
	}
	for _, key := range b {
		if _, ok := set[key]; ok {
			return true
		}
	}
	return false
}

func (s *Service) pinnedFAQKeysConflict(ctx context.Context, link db.Link, turn db.LinkAskTurn) error {
	others, err := s.listPinnedFAQSourceTurns(ctx, link)
	if err != nil {
		return err
	}
	proposedKeys := faqMatchKeys(turn)
	for _, other := range others {
		if uuid.UUID(other.ID.Bytes) == uuid.UUID(turn.ID.Bytes) {
			continue
		}
		if faqKeysOverlap(proposedKeys, faqMatchKeys(other)) {
			return ErrAskFAQAliasConflict
		}
	}
	return nil
}

func normalizeFAQAliasList(raw []string) ([]string, error) {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if len(trimmed) > maxPinnedFAQAliasChars {
			return nil, ErrAskFAQAliasesInvalid
		}
		key := normalizeAskQuestionKey(trimmed)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
		if len(out) > maxPinnedFAQAliases {
			return nil, ErrAskFAQAliasesInvalid
		}
	}
	return out, nil
}

func comparePinnedFAQTurns(a, b db.LinkAskTurn) bool {
	aSort := int32(1 << 30)
	bSort := int32(1 << 30)
	if a.PinnedFaqSort.Valid {
		aSort = a.PinnedFaqSort.Int32
	}
	if b.PinnedFaqSort.Valid {
		bSort = b.PinnedFaqSort.Int32
	}
	if aSort != bSort {
		return aSort < bSort
	}
	aPinned := a.PinnedFaqAt.Time
	bPinned := b.PinnedFaqAt.Time
	if !aPinned.Equal(bPinned) {
		return aPinned.After(bPinned)
	}
	return uuid.UUID(a.ID.Bytes).String() < uuid.UUID(b.ID.Bytes).String()
}

func matchPinnedFAQFromTurns(turns []db.LinkAskTurn, question string) (db.LinkAskTurn, bool) {
	key := normalizeAskQuestionKey(question)
	if key == "" {
		return db.LinkAskTurn{}, false
	}
	var hits []db.LinkAskTurn
	for _, turn := range turns {
		if !turn.PinnedFaqAt.Valid {
			continue
		}
		if pinnedFAQAnswer(turn) == "" {
			continue
		}
		for _, candidate := range faqMatchKeys(turn) {
			if candidate == key {
				hits = append(hits, turn)
				break
			}
		}
	}
	if len(hits) == 0 {
		return db.LinkAskTurn{}, false
	}
	sort.SliceStable(hits, func(i, j int) bool {
		return comparePinnedFAQTurns(hits[i], hits[j])
	})
	return hits[0], true
}

func (s *Service) listPinnedFAQSourceTurns(ctx context.Context, link db.Link) ([]db.LinkAskTurn, error) {
	if link.DealRoomID.Valid {
		rows, err := s.queries.ListRoomPublicAskFAQs(ctx, db.ListRoomPublicAskFAQsParams{
			DealRoomID:  link.DealRoomID,
			WorkspaceID: link.WorkspaceID,
			Limit:       publicAskFAQLimit,
		})
		if err != nil {
			return nil, err
		}
		out := make([]db.LinkAskTurn, 0, len(rows))
		for _, row := range rows {
			out = append(out, db.LinkAskTurn{
				ID:               row.ID,
				SessionID:        row.SessionID,
				TenantID:         row.TenantID,
				WorkspaceID:      row.WorkspaceID,
				LinkID:           row.LinkID,
				VisitorID:        row.VisitorID,
				Question:         row.Question,
				Lane:             row.Lane,
				Status:           row.Status,
				AiPayload:        row.AiPayload,
				HostAnswer:       row.HostAnswer,
				AnsweredBy:       row.AnsweredBy,
				RouteReason:      row.RouteReason,
				PinnedFaqAt:      row.PinnedFaqAt,
				PinnedFaqBy:      row.PinnedFaqBy,
				PinnedFaqSort:    row.PinnedFaqSort,
				FaqSourceTurnID:  row.FaqSourceTurnID,
				PinnedFaqAliases: row.PinnedFaqAliases,
				CreatedAt:        row.CreatedAt,
				UpdatedAt:        row.UpdatedAt,
			})
		}
		return out, nil
	}

	return s.queries.ListLinkPinnedAskFAQs(ctx, db.ListLinkPinnedAskFAQsParams{
		LinkID:      link.ID,
		WorkspaceID: link.WorkspaceID,
		Limit:       publicAskFAQLimit,
	})
}

func (s *Service) matchPinnedFAQ(ctx context.Context, link db.Link, question string) (db.LinkAskTurn, bool, error) {
	turns, err := s.listPinnedFAQSourceTurns(ctx, link)
	if err != nil {
		return db.LinkAskTurn{}, false, err
	}
	hit, ok := matchPinnedFAQFromTurns(turns, question)
	return hit, ok, nil
}

func shouldInterceptPinnedFAQ(routeReason string) bool {
	return routeReason != routeReasonUserEscalate && routeReason != routeReasonPolicyFormal
}

func (s *Service) createFaqReplayTurn(
	ctx context.Context,
	link db.Link,
	visitorID, visitorEmail, question string,
	source db.LinkAskTurn,
) (PublicAskTurn, error) {
	q, err := validateAskQuestion(question)
	if err != nil {
		return PublicAskTurn{}, err
	}
	answer := pinnedFAQAnswer(source)
	if answer == "" {
		return PublicAskTurn{}, ErrAskTurnNotPinnable
	}

	lane := askLaneAI
	status := askStatusAIAnswered
	hostAnswer := pgtype.Text{}
	var aiPayload []byte
	if source.HostAnswer.Valid && strings.TrimSpace(source.HostAnswer.String) != "" {
		lane = askLaneHost
		status = askStatusHostAnswered
		hostAnswer = pgtype.Text{String: strings.TrimSpace(source.HostAnswer.String), Valid: true}
	} else if askAIPayloadIsRefused(source.AiPayload) {
		return PublicAskTurn{}, ErrAskTurnNotPinnable
	} else {
		aiPayload = source.AiPayload
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PublicAskTurn{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	sess, err := s.getOrCreateAskSession(ctx, qtx, link, visitorID, visitorEmail)
	if err != nil {
		return PublicAskTurn{}, err
	}

	turn, err := qtx.CreateFaqReplayAskTurn(ctx, db.CreateFaqReplayAskTurnParams{
		SessionID:       sess.ID,
		TenantID:        link.TenantID,
		WorkspaceID:     link.WorkspaceID,
		LinkID:          link.ID,
		VisitorID:       visitorID,
		Question:        q,
		Lane:            lane,
		Status:          status,
		RouteReason:     pgtype.Text{String: routeReasonPinnedFAQ, Valid: true},
		HostAnswer:      hostAnswer,
		AiPayload:       aiPayload,
		FaqSourceTurnID: source.ID,
	})
	if err != nil {
		return PublicAskTurn{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PublicAskTurn{}, err
	}
	s.softInvalidateRoomList(ctx, link.WorkspaceID)
	return mapPublicAskTurnForVisitor(turn), nil
}

func (s *Service) maybeReplayPinnedFAQ(
	ctx context.Context,
	link db.Link,
	visitorID, visitorEmail, question, routeReason string,
) (PublicAskTurn, bool, error) {
	if !shouldInterceptPinnedFAQ(routeReason) {
		return PublicAskTurn{}, false, nil
	}
	source, ok, err := s.matchPinnedFAQ(ctx, link, question)
	if err != nil || !ok {
		return PublicAskTurn{}, false, err
	}
	turn, err := s.createFaqReplayTurn(ctx, link, visitorID, visitorEmail, question, source)
	if err != nil {
		if errors.Is(err, ErrAskTurnNotPinnable) {
			return PublicAskTurn{}, false, nil
		}
		return PublicAskTurn{}, false, err
	}
	return turn, true, nil
}

func (s *Service) SetAskTurnFAQAliases(
	ctx context.Context,
	link db.Link,
	turnID pgtype.UUID,
	userID string,
	aliases []string,
) (OwnerAskTurn, error) {
	if err := authorizeAskHostOwnerView(ctx, s.queries, link.WorkspaceID, link.DealRoomID, userID); err != nil {
		return OwnerAskTurn{}, err
	}

	normalized, err := normalizeFAQAliasList(aliases)
	if err != nil {
		return OwnerAskTurn{}, err
	}

	turn, err := s.queries.GetLinkAskTurnByID(ctx, db.GetLinkAskTurnByIDParams{
		ID:          turnID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OwnerAskTurn{}, ErrNotFoundInWorkspace
		}
		return OwnerAskTurn{}, err
	}
	if !turn.PinnedFaqAt.Valid {
		return OwnerAskTurn{}, ErrAskTurnNotPinned
	}

	proposed := turn
	proposed.PinnedFaqAliases = normalized
	if err := s.pinnedFAQKeysConflict(ctx, link, proposed); err != nil {
		return OwnerAskTurn{}, err
	}

	rows, err := s.queries.SetLinkAskTurnFAQAliases(ctx, db.SetLinkAskTurnFAQAliasesParams{
		PinnedFaqAliases: normalized,
		ID:               turnID,
		WorkspaceID:      link.WorkspaceID,
		LinkID:           link.ID,
	})
	if err != nil {
		return OwnerAskTurn{}, err
	}
	if rows == 0 {
		return OwnerAskTurn{}, ErrAskTurnNotPinned
	}

	updated, err := s.queries.GetOwnerAskTurnByID(ctx, db.GetOwnerAskTurnByIDParams{
		ID:          turnID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	})
	if err != nil {
		return OwnerAskTurn{}, err
	}
	return mapOwnerAskTurnFromOwnerIDRow(updated), nil
}

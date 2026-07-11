package dealroom

import (
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/compliance"
	"github.com/jackc/pgx/v5/pgtype"
)

func hashIPText(key, ip string) pgtype.Text {
	if ip == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: compliance.HashIP(key, ip), Valid: true}
}

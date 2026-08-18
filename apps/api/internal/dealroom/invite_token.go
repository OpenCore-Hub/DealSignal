package dealroom

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth/emailid"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
)

const roomInviteTokenPrefix = "dsr1"

var errRoomInviteToken = errors.New("invalid room invite token")

// mintRoomInviteToken builds a URL-safe HMAC token. Email is not encoded in the
// path (referrer leak); verification matches HMAC over room ID + mailbox keys.
func mintRoomInviteToken(secret, roomID, email string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errRoomInviteToken
	}
	rid, err := uuid.Parse(strings.TrimSpace(roomID))
	if err != nil {
		return "", errRoomInviteToken
	}
	mailbox := emailid.Canonical(email)
	if mailbox == "" {
		mailbox = strings.ToLower(strings.TrimSpace(email))
	}
	if mailbox == "" {
		return "", errRoomInviteToken
	}
	mac := roomInviteMAC(secret, rid, mailbox)
	return roomInviteTokenPrefix + "." +
		base64.RawURLEncoding.EncodeToString(rid[:]) + "." +
		base64.RawURLEncoding.EncodeToString(mac), nil
}

func parseRoomInviteToken(token string) (uuid.UUID, []byte, error) {
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != roomInviteTokenPrefix {
		return uuid.UUID{}, nil, errRoomInviteToken
	}
	rawID, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(rawID) != 16 {
		return uuid.UUID{}, nil, errRoomInviteToken
	}
	mac, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(mac) != sha256.Size {
		return uuid.UUID{}, nil, errRoomInviteToken
	}
	var rid uuid.UUID
	copy(rid[:], rawID)
	return rid, mac, nil
}

func roomInviteMAC(secret string, roomID uuid.UUID, email string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(roomID.String()))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(email))
	return mac.Sum(nil)
}

func matchRoomInviteMember(secret string, roomID uuid.UUID, mac []byte, rows []db.RoomMember) (db.RoomMember, bool) {
	var active db.RoomMember
	var hasActive bool
	for _, m := range rows {
		if m.Status != "pending" && m.Status != "active" {
			continue
		}
		if !roomInviteMACMatches(secret, roomID, m.Email, mac) {
			continue
		}
		if m.Status == "pending" {
			return m, true
		}
		active = m
		hasActive = true
	}
	return active, hasActive
}

func roomInviteMACMatches(secret string, roomID uuid.UUID, email string, mac []byte) bool {
	for _, key := range emailid.Keys(email) {
		if hmac.Equal(roomInviteMAC(secret, roomID, key), mac) {
			return true
		}
	}
	canon := emailid.Canonical(email)
	if canon != "" && hmac.Equal(roomInviteMAC(secret, roomID, canon), mac) {
		return true
	}
	return false
}

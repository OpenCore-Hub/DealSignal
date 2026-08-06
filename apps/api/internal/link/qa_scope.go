package link

// QaEnabledForLink controls visitor Q&A on share links.
// Document-only shares disable Ask until unified Visitor Ask ships; deal-room links keep it.
func QaEnabledForLink(isDealRoomLink bool) bool {
	return isDealRoomLink
}

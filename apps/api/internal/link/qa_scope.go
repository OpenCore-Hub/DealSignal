package link

// QaEnabledForLink controls visitor Q&A on share links.
// Document-only shares disable Ask until unified Visitor Ask ships; deal-room links keep it.
func QaEnabledForLink(isDealRoomLink bool) bool {
	return isDealRoomLink
}

// ResolveQaEnabled returns the persisted qa_enabled flag.
// Document-only links always disable Ask; deal-room links always enable Ask Host baseline.
func ResolveQaEnabled(isDealRoomLink bool, requested bool) bool {
	if !isDealRoomLink {
		return false
	}
	_ = requested // legacy clients may still send qa_enabled; deal-room links ignore it
	return true
}

// ResolveQaEnabledFromOptional resolves qa_enabled on create when the field may be omitted.
// Deal-room links always true; document links are always false.
func ResolveQaEnabledFromOptional(isDealRoomLink bool, requested *bool) bool {
	if !isDealRoomLink {
		return false
	}
	_ = requested
	return true
}

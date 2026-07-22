package ingestion

// embedOnIngest reports whether ProcessDocument should write vector embeddings.
// Deal-room documents are preview-only (pages/chunks) until an explicit KB
// create/rebuild embeds them.
func embedOnIngest(embedderConfigured, documentInDealRoom bool) bool {
	return embedderConfigured && !documentInDealRoom
}

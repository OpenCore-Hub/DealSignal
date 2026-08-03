package knowledge

import "testing"

func TestFolderPathExcludedIncludesDescendants(t *testing.T) {
	locked := map[string]bool{"/legal": true}
	if !folderPathExcluded("/legal", locked) {
		t.Fatal("exact locked folder should be excluded")
	}
	if !folderPathExcluded("/legal/nda", locked) {
		t.Fatal("descendant should be excluded")
	}
	if folderPathExcluded("/finance", locked) {
		t.Fatal("sibling folder must not be excluded")
	}
}

func TestKnowledgeExcludedDocumentOrFolder(t *testing.T) {
	lockedFolders := map[string]bool{"/legal": true}
	if !knowledgeExcluded(true, "/general", nil) {
		t.Fatal("document lock should exclude")
	}
	if !knowledgeExcluded(false, "/legal/nda", lockedFolders) {
		t.Fatal("folder lock should exclude")
	}
	if knowledgeExcluded(false, "/general", lockedFolders) {
		t.Fatal("unlocked doc in unlocked folder should remain eligible")
	}
}

func TestLockedFolderPathSetParsesSettings(t *testing.T) {
	settings := []byte(`{"folders":[{"path":"/legal","locked":true},{"path":"/general","locked":false}]}`)
	got := lockedFolderPathSet(settings)
	if !got["/legal"] || got["/general"] {
		t.Fatalf("unexpected locked folders: %#v", got)
	}
}

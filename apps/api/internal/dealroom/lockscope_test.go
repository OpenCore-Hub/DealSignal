package dealroom

import "testing"

func TestFolderPathLocked(t *testing.T) {
	locked := map[string]bool{"/legal": true}
	if !FolderPathLocked("/legal", locked) {
		t.Fatal("exact locked folder should match")
	}
	if !FolderPathLocked("/legal/nda", locked) {
		t.Fatal("descendant of locked folder should match")
	}
	if FolderPathLocked("/finance", locked) {
		t.Fatal("unrelated folder should not match")
	}
}

func TestResourceLockedOut(t *testing.T) {
	lockedFolders := map[string]bool{"/legal": true}
	if !ResourceLockedOut(true, "/general", nil) {
		t.Fatal("document lock should exclude")
	}
	if !ResourceLockedOut(false, "/legal/nda", lockedFolders) {
		t.Fatal("folder lock should exclude")
	}
	if ResourceLockedOut(false, "/general", lockedFolders) {
		t.Fatal("unlocked doc in unlocked folder should remain eligible")
	}
}

func TestLockedFolderPathSet(t *testing.T) {
	settings := []byte(`{"folders":[{"path":"/legal","locked":true},{"path":"/general","locked":false}]}`)
	got := LockedFolderPathSet(settings)
	if !got["/legal"] || got["/general"] {
		t.Fatalf("unexpected locked folders: %#v", got)
	}
}

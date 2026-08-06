package dealroom

import (
	"encoding/json"
	"strings"
)

type folderLockEntry struct {
	Path   string `json:"path"`
	Locked bool   `json:"locked"`
}

// LockedFolderPathSet returns folder paths marked locked in deal_rooms.settings.
func LockedFolderPathSet(settings []byte) map[string]bool {
	out := map[string]bool{}
	if len(settings) == 0 || string(settings) == "{}" {
		return out
	}
	var parsed struct {
		Folders []folderLockEntry `json:"folders"`
	}
	if err := json.Unmarshal(settings, &parsed); err != nil {
		return out
	}
	for _, f := range parsed.Folders {
		if !f.Locked {
			continue
		}
		p := NormalizeFolderPath(f.Path)
		if p != "/" {
			out[p] = true
		}
	}
	return out
}

// LockedFolderPathSetFromFolderJSON builds a locked-path set from settings.folders JSON.
func LockedFolderPathSetFromFolderJSON(foldersJSON []byte) map[string]bool {
	out := map[string]bool{}
	var folders []folderLockEntry
	if err := json.Unmarshal(foldersJSON, &folders); err != nil {
		return out
	}
	for _, f := range folders {
		if !f.Locked {
			continue
		}
		p := NormalizeFolderPath(f.Path)
		if p != "/" {
			out[p] = true
		}
	}
	return out
}

// NormalizeFolderPath canonicalizes deal-room folder paths for comparisons.
func NormalizeFolderPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return "/"
	}
	return p
}

// FolderPathLocked reports whether folderPath is a locked folder or a descendant.
func FolderPathLocked(folderPath string, lockedFolders map[string]bool) bool {
	if len(lockedFolders) == 0 {
		return false
	}
	p := NormalizeFolderPath(folderPath)
	if lockedFolders[p] {
		return true
	}
	for locked := range lockedFolders {
		if locked == "/" {
			continue
		}
		if strings.HasPrefix(p, locked+"/") {
			return true
		}
	}
	return false
}

// ResourceLockedOut reports whether a room document must stay out of visitor share
// links and the knowledge corpus (document lock or folder lock).
func ResourceLockedOut(docLocked bool, folderPath string, lockedFolders map[string]bool) bool {
	return docLocked || FolderPathLocked(folderPath, lockedFolders)
}

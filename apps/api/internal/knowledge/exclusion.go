package knowledge

import (
	"encoding/json"
	"strings"
)

type roomFolderLock struct {
	Path   string `json:"path"`
	Locked bool   `json:"locked"`
}

// lockedFolderPathSet returns folder paths marked locked in deal_rooms.settings.
func lockedFolderPathSet(settings []byte) map[string]bool {
	out := map[string]bool{}
	if len(settings) == 0 || string(settings) == "{}" {
		return out
	}
	var parsed struct {
		Folders []roomFolderLock `json:"folders"`
	}
	if err := json.Unmarshal(settings, &parsed); err != nil {
		return out
	}
	for _, f := range parsed.Folders {
		if !f.Locked {
			continue
		}
		p := normalizeKnowledgeFolderPath(f.Path)
		if p != "/" {
			out[p] = true
		}
	}
	return out
}

func normalizeKnowledgeFolderPath(p string) string {
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

// folderPathExcluded reports whether folderPath is a locked folder or a descendant.
func folderPathExcluded(folderPath string, lockedFolders map[string]bool) bool {
	if len(lockedFolders) == 0 {
		return false
	}
	p := normalizeKnowledgeFolderPath(folderPath)
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

// knowledgeExcluded reports whether a room document must stay out of the corpus.
func knowledgeExcluded(docLocked bool, folderPath string, lockedFolders map[string]bool) bool {
	return docLocked || folderPathExcluded(folderPath, lockedFolders)
}

package knowledge

import (
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
)

func lockedFolderPathSet(settings []byte) map[string]bool {
	return dealroom.LockedFolderPathSet(settings)
}

func folderPathExcluded(folderPath string, lockedFolders map[string]bool) bool {
	return dealroom.FolderPathLocked(folderPath, lockedFolders)
}

func knowledgeExcluded(docLocked bool, folderPath string, lockedFolders map[string]bool) bool {
	return dealroom.ResourceLockedOut(docLocked, folderPath, lockedFolders)
}

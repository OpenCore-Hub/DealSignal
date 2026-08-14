package dealroom

import "testing"

func TestRoomTemplatesIncludeCustom(t *testing.T) {
	t.Parallel()
	if len(roomTemplates) < 10 {
		t.Fatalf("want at least 10 scenarios, got %d", len(roomTemplates))
	}
	last := roomTemplates[len(roomTemplates)-1]
	if last.Scenario != "custom" || last.ID != "tmpl_custom" {
		t.Fatalf("last template = %s %s, want tmpl_custom / custom", last.ID, last.Scenario)
	}
	if last.NDAEnabled {
		t.Fatal("custom scenario must default NDA off")
	}
	if len(last.FolderStructure) != 0 {
		t.Fatalf("custom scenario must start empty, got %d folders", len(last.FolderStructure))
	}
	if folders := templateFolders("custom"); len(folders) != 0 {
		t.Fatalf("templateFolders(custom) = %d, want 0", len(folders))
	}
}

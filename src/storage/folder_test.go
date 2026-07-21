package storage

import "testing"

func TestFolderAutoReadScroll(t *testing.T) {
	db := testDB()
	folder := db.CreateFolder("news")
	if folder == nil {
		t.Fatal("expected folder")
	}
	if folder.AutoReadScroll {
		t.Fatal("auto_read_scroll should default to false")
	}

	list := db.ListFolders()
	if len(list) != 1 || list[0].AutoReadScroll {
		t.Fatalf("list auto_read_scroll = %#v", list)
	}

	if !db.UpdateFolderAutoReadScroll(folder.Id, true) {
		t.Fatal("update failed")
	}
	list = db.ListFolders()
	if len(list) != 1 || !list[0].AutoReadScroll {
		t.Fatalf("expected enabled, got %#v", list)
	}
}

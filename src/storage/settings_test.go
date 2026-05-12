package storage

import "testing"

func TestToolbarDisplayDefault(t *testing.T) {
	db := testDB()

	if got := db.GetSettingsValue("toolbar_display"); got != "text" {
		t.Fatalf("invalid toolbar display default: %#v", got)
	}

	settings := db.GetSettings()
	if got := settings["toolbar_display"]; got != "text" {
		t.Fatalf("invalid toolbar display setting: %#v", got)
	}
}

func TestUpdateToolbarDisplay(t *testing.T) {
	db := testDB()

	if !db.UpdateSettings(map[string]interface{}{"toolbar_display": "text"}) {
		t.Fatal("did not update toolbar display")
	}
	if got := db.GetSettingsValue("toolbar_display"); got != "text" {
		t.Fatalf("invalid toolbar display: %#v", got)
	}
}

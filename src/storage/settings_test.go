package storage

import "testing"

func TestUpdateSettingsNormalizesRSSHubBaseURL(t *testing.T) {
	db := testDB()

	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": "https://rsshub.rssforever.com/"}) {
		t.Fatal("update failed")
	}

	got := db.GetSettingsValueString("rsshub_base_url")
	want := "https://rsshub.rssforever.com"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUpdateSettingsRejectsInvalidRSSHubBaseURL(t *testing.T) {
	db := testDB()

	if db.UpdateSettings(map[string]interface{}{"rsshub_base_url": "file:///tmp/rsshub"}) {
		t.Fatal("expected update to fail")
	}
}

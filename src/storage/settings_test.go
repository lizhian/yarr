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

func TestArticleListLayoutDefault(t *testing.T) {
	db := testDB()

	if got := db.GetSettingsValue("article_list_layout"); got != "list" {
		t.Fatalf("invalid article list layout default: %#v", got)
	}

	settings := db.GetSettings()
	if got := settings["article_list_layout"]; got != "list" {
		t.Fatalf("invalid article list layout setting: %#v", got)
	}
}

func TestUpdateArticleListLayout(t *testing.T) {
	db := testDB()

	if !db.UpdateSettings(map[string]interface{}{"article_list_layout": "card"}) {
		t.Fatal("did not update article list layout")
	}
	if got := db.GetSettingsValue("article_list_layout"); got != "card" {
		t.Fatalf("invalid article list layout: %#v", got)
	}
}

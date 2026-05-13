package storage

import "testing"

func setRawSetting(t *testing.T, db *Storage, key string, val string) {
	t.Helper()
	if _, err := db.db.Exec(`
		insert into settings (key, val) values (?, ?)
		on conflict (key) do update set val=?`,
		key, val, val,
	); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSettingsNormalizesRSSHubBaseURL(t *testing.T) {
	db := testDB()

	if !db.UpdateSettings(map[string]interface{}{"rsshub_base_url": "https://rsshub.rssforever.com/\n# https://example.com/rsshub/"}) {
		t.Fatal("update failed")
	}

	got := db.GetSettingsValueString("rsshub_base_url")
	want := "https://rsshub.rssforever.com\n#https://example.com/rsshub"
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

func TestUpdateSettingsRejectsInvalidDisabledRSSHubBaseURL(t *testing.T) {
	db := testDB()

	if db.UpdateSettings(map[string]interface{}{"rsshub_base_url": "# note"}) {
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

func TestThemeFontDefault(t *testing.T) {
	db := testDB()

	if got := db.GetSettingsValue("theme_font"); got != "lxgw-wenkai" {
		t.Fatalf("invalid theme font default: %#v", got)
	}

	settings := db.GetSettings()
	if got := settings["theme_font"]; got != "lxgw-wenkai" {
		t.Fatalf("invalid theme font setting: %#v", got)
	}
}

func TestUpdateThemeFont(t *testing.T) {
	db := testDB()

	if !db.UpdateSettings(map[string]interface{}{"theme_font": "maple-mono-nf-cn"}) {
		t.Fatal("did not update theme font")
	}
	if got := db.GetSettingsValue("theme_font"); got != "maple-mono-nf-cn" {
		t.Fatalf("invalid theme font: %#v", got)
	}
}

func TestThemeFontFallsBackToDefault(t *testing.T) {
	db := testDB()

	if !db.UpdateSettings(map[string]interface{}{"theme_font": ""}) {
		t.Fatal("did not update theme font")
	}
	if got := db.GetSettingsValue("theme_font"); got != "lxgw-wenkai" {
		t.Fatalf("invalid theme font fallback: %#v", got)
	}

	if !db.UpdateSettings(map[string]interface{}{"theme_font": "unknown"}) {
		t.Fatal("did not update theme font")
	}
	if got := db.GetSettingsValue("theme_font"); got != "lxgw-wenkai" {
		t.Fatalf("invalid theme font fallback: %#v", got)
	}
}

func TestStoredInvalidThemeFontFallsBackToDefault(t *testing.T) {
	db := testDB()
	setRawSetting(t, db, "theme_font", `""`)

	if got := db.GetSettingsValue("theme_font"); got != "lxgw-wenkai" {
		t.Fatalf("invalid theme font fallback: %#v", got)
	}

	settings := db.GetSettings()
	if got := settings["theme_font"]; got != "lxgw-wenkai" {
		t.Fatalf("invalid theme font setting fallback: %#v", got)
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

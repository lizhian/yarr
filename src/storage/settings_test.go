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

func TestRSSHubBaseURLDefault(t *testing.T) {
	db := testDB()

	if got := db.GetSettingsValueString("rsshub_base_url"); got != "" {
		t.Fatalf("invalid rsshub_base_url default: %q", got)
	}
	settings := db.GetSettings()
	if got := settings["rsshub_base_url"]; got != "" {
		t.Fatalf("invalid rsshub_base_url setting: %#v", got)
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

func TestBackupEnabledDefault(t *testing.T) {
	db := testDB()

	if got := db.GetSettingsValueBool("backup_enabled"); got {
		t.Fatalf("invalid backup enabled default: %#v", got)
	}

	settings := db.GetSettings()
	if got := settings["backup_enabled"]; got != false {
		t.Fatalf("invalid backup enabled setting: %#v", got)
	}
}

func TestUpdateBackupEnabled(t *testing.T) {
	db := testDB()

	if !db.UpdateSettings(map[string]interface{}{"backup_enabled": true}) {
		t.Fatal("did not update backup enabled")
	}
	if got := db.GetSettingsValueBool("backup_enabled"); !got {
		t.Fatalf("invalid backup enabled: %#v", got)
	}
}

func TestColumnWidthDefaultsUnset(t *testing.T) {
	db := testDB()

	if got := db.GetSettingsValue("feed_list_width"); got != 0 {
		t.Fatalf("invalid feed_list_width default: %#v", got)
	}
	if got := db.GetSettingsValue("item_list_width"); got != 0 {
		t.Fatalf("invalid item_list_width default: %#v", got)
	}

	settings := db.GetSettings()
	if got := settings["feed_list_width"]; got != 0 {
		t.Fatalf("invalid feed_list_width setting: %#v", got)
	}
	if got := settings["item_list_width"]; got != 0 {
		t.Fatalf("invalid item_list_width setting: %#v", got)
	}
}

func TestFeedSettingIgnored(t *testing.T) {
	db := testDB()
	setRawSetting(t, db, "feed", `"feed:9"`)

	if got := db.GetSettingsValue("feed"); got != nil {
		t.Fatalf("feed setting should not have a value: %#v", got)
	}

	settings := db.GetSettings()
	if _, ok := settings["feed"]; ok {
		t.Fatal("feed setting should not be returned")
	}

	if !db.UpdateSettings(map[string]interface{}{"feed": "feed:10"}) {
		t.Fatal("feed setting update should be ignored without failing")
	}
	if got := db.GetSettingsValue("feed"); got != nil {
		t.Fatalf("feed setting should remain ignored after update: %#v", got)
	}
}

func TestMigrationRemovesFeedSetting(t *testing.T) {
	db := testDB()
	setRawSetting(t, db, "feed", `"feed:9"`)
	if _, err := db.db.Exec(`pragma user_version = 17`); err != nil {
		t.Fatal(err)
	}

	if err := migrate(db.db); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.db.QueryRow(`select count(*) from settings where key = 'feed'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("feed setting was not removed: %d", count)
	}
}

func TestFrontendSettingsAreIgnored(t *testing.T) {
	db := testDB()
	keys := []string{"theme_name", "theme_font", "toolbar_display", "sort_newest_first"}
	for _, key := range keys {
		setRawSetting(t, db, key, `"legacy"`)
		if got := db.GetSettingsValue(key); got != nil {
			t.Fatalf("frontend setting %q should not have a database value: %#v", key, got)
		}
		if _, ok := db.GetSettings()[key]; ok {
			t.Fatalf("frontend setting %q should not be returned", key)
		}
		if !db.UpdateSettings(map[string]interface{}{key: "updated"}) {
			t.Fatalf("frontend setting %q update should be ignored", key)
		}
	}
}

func TestMigrationRemovesFrontendSettings(t *testing.T) {
	db := testDB()
	keys := []string{"theme_name", "theme_font", "toolbar_display", "sort_newest_first"}
	for _, key := range keys {
		setRawSetting(t, db, key, `"legacy"`)
	}
	if _, err := db.db.Exec(`pragma user_version = 19`); err != nil {
		t.Fatal(err)
	}

	if err := migrate(db.db); err != nil {
		t.Fatal(err)
	}

	for _, key := range keys {
		var count int
		if err := db.db.QueryRow(`select count(*) from settings where key = ?`, key).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("frontend setting %q was not removed", key)
		}
	}
}

func TestUnknownSettingsAreIgnored(t *testing.T) {
	db := testDB()
	setRawSetting(t, db, "article_list_layout", `"card"`)

	if got := db.GetSettingsValue("article_list_layout"); got != nil {
		t.Fatalf("unknown setting should not have a value: %#v", got)
	}

	settings := db.GetSettings()
	if _, ok := settings["article_list_layout"]; ok {
		t.Fatal("unknown setting should not be returned")
	}

	if !db.UpdateSettings(map[string]interface{}{"article_list_layout": "card"}) {
		t.Fatal("unknown setting update should be ignored without failing")
	}
	if got := db.GetSettingsValue("article_list_layout"); got != nil {
		t.Fatalf("unknown setting should remain ignored after update: %#v", got)
	}
}

func TestRemovedThemeSizeSettingIsIgnored(t *testing.T) {
	db := testDB()
	setRawSetting(t, db, "theme_size", `1.2`)

	if got := db.GetSettingsValue("theme_size"); got != nil {
		t.Fatalf("removed theme_size setting should not have a value: %#v", got)
	}

	settings := db.GetSettings()
	if _, ok := settings["theme_size"]; ok {
		t.Fatal("removed theme_size setting should not be returned")
	}

	if !db.UpdateSettings(map[string]interface{}{"theme_size": 1.3}) {
		t.Fatal("removed theme_size update should be ignored without failing")
	}
	if got := db.GetSettingsValue("theme_size"); got != nil {
		t.Fatalf("removed theme_size setting should remain ignored after update: %#v", got)
	}
}

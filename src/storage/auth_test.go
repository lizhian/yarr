package storage

import "testing"

func TestAuthConfigDefault(t *testing.T) {
	db := testDB()

	config := db.GetAuthConfig()
	if config.Enabled {
		t.Fatal("expected auth disabled by default")
	}
}

func TestSetAuthConfig(t *testing.T) {
	db := testDB()

	if !db.SetAuthConfig(true, " username ", "password") {
		t.Fatal("did not enable auth")
	}

	config := db.GetAuthConfig()
	if !config.Enabled {
		t.Fatal("expected auth enabled")
	}
	if config.Username != "username" {
		t.Fatalf("got username %q", config.Username)
	}
	if config.Password != "password" {
		t.Fatalf("got password %q", config.Password)
	}
}

func TestSetAuthConfigRejectsMissingCredentials(t *testing.T) {
	db := testDB()

	if db.SetAuthConfig(true, "", "password") {
		t.Fatal("expected missing username to be rejected")
	}
	if db.SetAuthConfig(true, "username", "") {
		t.Fatal("expected missing password to be rejected")
	}
}

func TestSetAuthConfigDisabledClearsCredentials(t *testing.T) {
	db := testDB()

	if !db.SetAuthConfig(true, "username", "password") {
		t.Fatal("did not enable auth")
	}
	if !db.SetAuthConfig(false, "", "") {
		t.Fatal("did not disable auth")
	}

	config := db.GetAuthConfig()
	if config.Enabled || config.Username != "" || config.Password != "" {
		t.Fatalf("expected auth config to be cleared, got %#v", config)
	}
}

func TestAuthConfigNotExposedInSettings(t *testing.T) {
	db := testDB()

	if !db.SetAuthConfig(true, "username", "password") {
		t.Fatal("did not enable auth")
	}

	settings := db.GetSettings()
	if _, ok := settings[authEnabledKey]; ok {
		t.Fatal("auth enabled exposed in settings")
	}
	if _, ok := settings[authUsernameKey]; ok {
		t.Fatal("auth username exposed in settings")
	}
	if _, ok := settings[authPasswordKey]; ok {
		t.Fatal("auth password exposed in settings")
	}
}

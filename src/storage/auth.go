package storage

import (
	"database/sql"
	"encoding/json"
	"log"
	"strings"
)

const (
	authEnabledKey  = "auth_enabled"
	authUsernameKey = "auth_username"
	authPasswordKey = "auth_password"
)

type AuthConfig struct {
	Enabled  bool
	Username string
	Password string
}

func (s *Storage) getAuthSetting(key string) interface{} {
	row := s.db.QueryRow(`select val from settings where key=?`, key)
	var val []byte
	if err := row.Scan(&val); err != nil {
		if err != sql.ErrNoRows {
			log.Print(err)
		}
		return nil
	}
	var decoded interface{}
	if err := json.Unmarshal(val, &decoded); err != nil {
		log.Print(err)
		return nil
	}
	return decoded
}

func (s *Storage) GetAuthConfig() AuthConfig {
	enabled, _ := s.getAuthSetting(authEnabledKey).(bool)
	username, _ := s.getAuthSetting(authUsernameKey).(string)
	password, _ := s.getAuthSetting(authPasswordKey).(string)
	username = strings.TrimSpace(username)
	if !enabled || username == "" || password == "" {
		return AuthConfig{}
	}
	return AuthConfig{
		Enabled:  true,
		Username: username,
		Password: password,
	}
}

func (s *Storage) SetAuthConfig(enabled bool, username, password string) bool {
	username = strings.TrimSpace(username)
	if !enabled {
		_, err := s.db.Exec(
			`delete from settings where key in (?, ?, ?)`,
			authEnabledKey, authUsernameKey, authPasswordKey,
		)
		if err != nil {
			log.Print(err)
			return false
		}
		return true
	}

	if username == "" || password == "" {
		return false
	}

	tx, err := s.db.Begin()
	if err != nil {
		log.Print(err)
		return false
	}
	for key, val := range map[string]interface{}{
		authEnabledKey:  true,
		authUsernameKey: username,
		authPasswordKey: password,
	} {
		valEncoded, err := json.Marshal(val)
		if err != nil {
			log.Print(err)
			tx.Rollback()
			return false
		}
		_, err = tx.Exec(`
			insert into settings (key, val) values (?, ?)
			on conflict (key) do update set val=?`,
			key, valEncoded, valEncoded,
		)
		if err != nil {
			log.Print(err)
			tx.Rollback()
			return false
		}
	}
	if err := tx.Commit(); err != nil {
		log.Print(err)
		return false
	}
	return true
}

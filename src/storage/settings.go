package storage

import (
	"database/sql"
	"encoding/json"
	"log"

	"github.com/nkanaev/yarr/src/rsshub"
)

func settingsDefaults() map[string]interface{} {
	return map[string]interface{}{
		"filter":           "",
		"feed_list_width":  0,
		"item_list_width":  0,
		"refresh_rate":     0,
		"rsshub_base_url":  "",
		"backup_enabled":   false,
		"auto_read_scroll": false,
	}
}

func (s *Storage) GetSettingsValue(key string) interface{} {
	if _, ok := settingsDefaults()[key]; !ok {
		return nil
	}
	row := s.db.QueryRow(`select val from settings where key=?`, key)
	var val []byte
	if err := row.Scan(&val); err != nil {
		if err != sql.ErrNoRows {
			log.Print(err)
		}
		return settingsDefaults()[key]
	}
	if len(val) == 0 {
		return nil
	}
	var valDecoded interface{}
	if err := json.Unmarshal([]byte(val), &valDecoded); err != nil {
		log.Print(err)
		return nil
	}
	return valDecoded
}

func (s *Storage) GetSettingsValueInt64(key string) int64 {
	val := s.GetSettingsValue(key)
	if val != nil {
		if fval, ok := val.(float64); ok {
			return int64(fval)
		}
	}
	return 0
}

func (s *Storage) GetSettingsValueString(key string) string {
	val := s.GetSettingsValue(key)
	if val != nil {
		if sval, ok := val.(string); ok {
			return sval
		}
	}
	return ""
}

func (s *Storage) GetSettingsValueBool(key string) bool {
	val := s.GetSettingsValue(key)
	if val != nil {
		if bval, ok := val.(bool); ok {
			return bval
		}
	}
	return false
}

func (s *Storage) GetSettings() map[string]interface{} {
	result := settingsDefaults()
	rows, err := s.db.Query(`select key, val from settings;`)
	if err != nil {
		log.Print(err)
		return result
	}
	for rows.Next() {
		var key string
		var val []byte
		var valDecoded interface{}

		rows.Scan(&key, &val)
		if _, ok := result[key]; !ok {
			continue
		}
		if err = json.Unmarshal([]byte(val), &valDecoded); err != nil {
			log.Print(err)
			continue
		}
		result[key] = valDecoded
	}
	return result
}

func (s *Storage) UpdateSettings(kv map[string]interface{}) bool {
	defaults := settingsDefaults()
	for key, val := range kv {
		if defaults[key] == nil {
			continue
		}
		if key == "rsshub_base_url" {
			sval, ok := val.(string)
			if !ok {
				return false
			}
			normalized, err := rsshub.NormalizeBaseList(sval)
			if err != nil {
				log.Print(err)
				return false
			}
			val = normalized
		}
		valEncoded, err := json.Marshal(val)
		if err != nil {
			log.Print(err)
			return false
		}
		_, err = s.db.Exec(`
			insert into settings (key, val) values (?, ?)
			on conflict (key) do update set val=?`,
			key, valEncoded, valEncoded,
		)
		if err != nil {
			log.Print(err)
			return false
		}
	}
	return true
}

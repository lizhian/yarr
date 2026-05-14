package storage

import (
	"database/sql"
	"encoding/json"
	"log"

	"github.com/nkanaev/yarr/src/rsshub"
)

func settingsDefaults() map[string]interface{} {
	return map[string]interface{}{
		"filter":              "",
		"feed":                "",
		"feed_list_width":     300,
		"item_list_width":     300,
		"sort_newest_first":   true,
		"theme_name":          "light",
		"theme_font":          "lxgw-wenkai",
		"theme_size":          1,
		"refresh_rate":        0,
		"rsshub_base_url":     "",
		"toolbar_display":     "text",
		"article_list_layout": "list",
	}
}

func normalizeSetting(key string, val interface{}) interface{} {
	if key != "theme_font" {
		return val
	}
	if val, ok := val.(string); ok && val == "maple-mono-nf-cn" {
		return val
	}
	return "lxgw-wenkai"
}

func (s *Storage) GetSettingsValue(key string) interface{} {
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
	return normalizeSetting(key, valDecoded)
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
		result[key] = normalizeSetting(key, valDecoded)
	}
	return result
}

func (s *Storage) UpdateSettings(kv map[string]interface{}) bool {
	defaults := settingsDefaults()
	for key, val := range kv {
		if defaults[key] == nil {
			continue
		}
		val = normalizeSetting(key, val)
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

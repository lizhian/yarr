package storage

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"
)

var backupTables = []string{
	"folders",
	"feeds",
	"items",
	"settings",
	"http_states",
	"feed_errors",
	"feed_sizes",
}

func (s *Storage) BackupTables() (map[string][]map[string]interface{}, error) {
	result := make(map[string][]map[string]interface{}, len(backupTables))
	for _, table := range backupTables {
		rows, err := s.backupTable(table)
		if err != nil {
			return nil, err
		}
		result[table] = rows
	}
	return result, nil
}

func (s *Storage) backupTable(table string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(fmt.Sprintf("select * from %s order by rowid", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0)
	for rows.Next() {
		raw := make([]interface{}, len(cols))
		dest := make([]interface{}, len(cols))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			row[col] = backupValue(table, col, raw[i])
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func backupValue(table, col string, val interface{}) interface{} {
	switch v := val.(type) {
	case nil:
		return nil
	case []byte:
		if table == "settings" && col == "val" {
			var decoded interface{}
			if err := json.Unmarshal(v, &decoded); err == nil {
				return decoded
			}
		}
		if utf8.Valid(v) {
			return string(v)
		}
		return v
	case time.Time:
		return v.Format(time.RFC3339Nano)
	default:
		return v
	}
}

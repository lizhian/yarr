package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mattn/go-sqlite3"
)

func (s *Storage) BackupTo(path string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	srcConn, err := s.db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer srcConn.Close()

	dstDB, err := sql.Open("sqlite3", tmpPath)
	if err != nil {
		return err
	}

	dstConn, err := dstDB.Conn(context.Background())
	if err != nil {
		dstDB.Close()
		return err
	}

	if err := sqliteBackup(srcConn, dstConn); err != nil {
		dstConn.Close()
		dstDB.Close()
		return err
	}
	if err := dstConn.Close(); err != nil {
		return err
	}
	if err := dstDB.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.Rename(tmpPath, path)
}

func sqliteBackup(srcConn, dstConn *sql.Conn) error {
	return dstConn.Raw(func(dstDriverConn interface{}) error {
		dst, ok := dstDriverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("unexpected sqlite destination connection %T", dstDriverConn)
		}
		return srcConn.Raw(func(srcDriverConn interface{}) error {
			src, ok := srcDriverConn.(*sqlite3.SQLiteConn)
			if !ok {
				return fmt.Errorf("unexpected sqlite source connection %T", srcDriverConn)
			}

			backup, err := dst.Backup("main", src, "main")
			if err != nil {
				return err
			}
			defer backup.Finish()

			for {
				done, err := backup.Step(256)
				if err != nil {
					return err
				}
				if done {
					return nil
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	})
}

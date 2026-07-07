package server

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nkanaev/yarr/src/server/opml"
	"github.com/nkanaev/yarr/src/storage"
)

const (
	backupOPMLFile     = "subscriptions.opml"
	backupDatabaseFile = "storage.db"
)

type BackupService struct {
	db     *storage.Storage
	dbPath string
	mu     sync.Mutex
	now    func() time.Time
}

type BackupResult struct {
	Path      string   `json:"path"`
	FeedCount int      `json:"feed_count"`
	Files     []string `json:"files"`
}

func NewBackupService(db *storage.Storage, dbPath string) *BackupService {
	return &BackupService{
		db:     db,
		dbPath: dbPath,
		now:    time.Now,
	}
}

func (b *BackupService) Run() (*BackupResult, error) {
	if b == nil {
		return nil, errors.New("backup service is not configured")
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	backupDir, err := b.backupDir(now)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(backupDir, backupDatabaseFile)
	opmlBody := []byte(BuildOPML(b.db).OPML())

	if err := b.db.BackupTo(dbPath); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(filepath.Join(backupDir, backupOPMLFile), opmlBody, 0644); err != nil {
		return nil, err
	}

	return &BackupResult{
		Path:      backupDir,
		FeedCount: len(b.db.ListFeeds()),
		Files:     []string{backupDatabaseFile, backupOPMLFile},
	}, nil
}

func (b *BackupService) backupDir(now time.Time) (string, error) {
	path, err := backupDBPath(b.dbPath)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
	if dir == "." {
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "backups", now.Format("2006-01-02")), nil
}

func backupDBPath(dbPath string) (string, error) {
	orig := dbPath
	if pos := strings.IndexRune(dbPath, '?'); pos != -1 {
		dbPath = dbPath[:pos]
	}
	if dbPath == "" || dbPath == ":memory:" || strings.HasPrefix(dbPath, "file::memory:") {
		return "", fmt.Errorf("cannot back up database path %q", orig)
	}
	if strings.HasPrefix(dbPath, "file:") {
		dbPath = strings.TrimPrefix(dbPath, "file:")
	}
	if dbPath == "" || dbPath == ":memory:" {
		return "", fmt.Errorf("cannot back up database path %q", dbPath)
	}
	return filepath.Clean(dbPath), nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func BuildOPML(db *storage.Storage) opml.Folder {
	doc := opml.Folder{}

	feedsByFolderID := make(map[int64][]*storage.Feed)
	for _, feed := range db.ListFeeds() {
		feed := feed
		if feed.FolderId == nil {
			doc.Feeds = append(doc.Feeds, opml.Feed{
				Title:           feed.Title,
				FeedUrl:         feed.FeedLink,
				SiteUrl:         feed.Link,
				ContentSelector: feed.ContentSelector,
				IconURL:         feed.IconURL,
			})
		} else {
			id := *feed.FolderId
			feedsByFolderID[id] = append(feedsByFolderID[id], &feed)
		}
	}

	for _, folder := range db.ListFolders() {
		folderFeeds := feedsByFolderID[folder.Id]
		if len(folderFeeds) == 0 {
			continue
		}
		opmlfolder := opml.Folder{Title: folder.Title}
		for _, feed := range folderFeeds {
			opmlfolder.Feeds = append(opmlfolder.Feeds, opml.Feed{
				Title:           feed.Title,
				FeedUrl:         feed.FeedLink,
				SiteUrl:         feed.Link,
				ContentSelector: feed.ContentSelector,
				IconURL:         feed.IconURL,
			})
		}
		doc.Folders = append(doc.Folders, opmlfolder)
	}

	return doc
}

func (s *Server) startBackupScheduler() {
	if s.backups == nil {
		return
	}
	s.backupsOnce.Do(func() {
		go func() {
			for {
				now := time.Now()
				next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
				timer := time.NewTimer(time.Until(next))
				<-timer.C
				if err := s.runScheduledBackup(); err != nil {
					log.Print("backup failed: ", err)
				}
			}
		}()
	})
}

func (s *Server) runScheduledBackup() error {
	if s.db == nil || !s.db.GetSettingsValueBool("backup_enabled") {
		return nil
	}
	_, err := s.backups.Run()
	return err
}

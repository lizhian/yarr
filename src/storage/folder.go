package storage

import (
	"log"
)

type Folder struct {
	Id              int64  `json:"id"`
	Title           string `json:"title"`
	IsExpanded      bool   `json:"is_expanded"`
	AutoReadScroll  bool   `json:"auto_read_scroll"`
	UnreadFirst     bool   `json:"unread_first"`
	SortNewestFirst bool   `json:"sort_newest_first"`
}

func (s *Storage) CreateFolder(title string) *Folder {
	expanded := true
	row := s.db.QueryRow(`
		insert into folders (title, is_expanded, auto_read_scroll) values (?, ?, false)
		on conflict (title) do update set title = ?
		returning id, is_expanded, auto_read_scroll, unread_first, sort_newest_first`,
		title, expanded,
		// provide title again so that we can extract row id
		title,
	)
	var id int64
	var isExpanded, autoReadScroll, unreadFirst, sortNewestFirst bool
	err := row.Scan(&id, &isExpanded, &autoReadScroll, &unreadFirst, &sortNewestFirst)

	if err != nil {
		log.Print(err)
		return nil
	}
	return &Folder{Id: id, Title: title, IsExpanded: isExpanded, AutoReadScroll: autoReadScroll, UnreadFirst: unreadFirst, SortNewestFirst: sortNewestFirst}
}

func (s *Storage) DeleteFolder(folderId int64) bool {
	_, err := s.db.Exec(`delete from folders where id = ?`, folderId)
	if err != nil {
		log.Print(err)
	}
	return err == nil
}

func (s *Storage) RenameFolder(folderId int64, newTitle string) bool {
	_, err := s.db.Exec(`update folders set title = ? where id = ?`, newTitle, folderId)
	return err == nil
}

func (s *Storage) ToggleFolderExpanded(folderId int64, isExpanded bool) bool {
	_, err := s.db.Exec(`update folders set is_expanded = ? where id = ?`, isExpanded, folderId)
	return err == nil
}

func (s *Storage) UpdateFolderAutoReadScroll(folderId int64, enabled bool) bool {
	_, err := s.db.Exec(`update folders set auto_read_scroll = ? where id = ?`, enabled, folderId)
	return err == nil
}

func (s *Storage) UpdateFolderItemOrder(folderID int64, unreadFirst, sortNewestFirst *bool) bool {
	_, err := s.db.Exec(`
		update folders set
			unread_first = coalesce(?, unread_first),
			sort_newest_first = coalesce(?, sort_newest_first)
		where id = ?`, unreadFirst, sortNewestFirst, folderID)
	return err == nil
}

func (s *Storage) ListFolders() []Folder {
	result := make([]Folder, 0, 0)
	rows, err := s.db.Query(`
		select id, title, is_expanded, auto_read_scroll, unread_first, sort_newest_first
		from folders
		order by title collate nocase
	`)
	if err != nil {
		log.Print(err)
		return result
	}
	for rows.Next() {
		var f Folder
		err = rows.Scan(&f.Id, &f.Title, &f.IsExpanded, &f.AutoReadScroll, &f.UnreadFirst, &f.SortNewestFirst)
		if err != nil {
			log.Print(err)
			return result
		}
		result = append(result, f)
	}
	return result
}

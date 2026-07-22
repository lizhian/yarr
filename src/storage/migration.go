package storage

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

var migrations = []func(*sql.Tx) error{
	m01_initial,
	m02_feed_states_and_errors,
	m03_on_delete_actions,
	m04_item_podcasturl,
	m05_move_description_to_content,
	m06_fill_missing_dates,
	m07_add_feed_size,
	m08_normalize_datetime,
	m09_change_item_index,
	m10_add_item_medialinks,
	m11_add_feed_content_selector,
	m12_replace_feed_icon_with_icon_url,
	m13_add_feed_content_mode,
	m14_add_generated_rss,
	m15_add_feed_ranking_mode,
	m16_change_feed_ranking_mode_to_text,
	m17_add_item_list_indexes,
	m18_remove_feed_setting,
	m19_add_feed_last_ranking_state,
	m20_remove_frontend_settings,
	m21_add_auto_read_scroll,
	m22_add_feed_latest_item_arrived_at,
	m23_add_feed_custom_icon,
	m24_add_favorite_and_item_order,
	m25_add_feed_refresh_times,
}

var maxVersion = int64(len(migrations))

func migrate(db *sql.DB) error {
	var version int64
	if err := db.QueryRow("pragma user_version").Scan(&version); err != nil {
		return err
	}

	if version >= maxVersion {
		return nil
	}

	log.Printf("db version is %d. migrating to %d", version, maxVersion)

	for v := version + 1; v <= maxVersion; v++ {
		// Migrations altering schema using a sequence of steps due to SQLite limitations.
		// Must come with `pragma foreign_key_check` at the end. See:
		// "Making Other Kinds Of Table Schema Changes"
		// https://www.sqlite.org/lang_altertable.html
		trickyAlteration := (v == 3 || v == 16)

		log.Printf("[migration:%d] starting", v)

		if trickyAlteration {
			db.Exec("pragma foreign_keys=off;")
		}

		err := migrateVersion(v, db)

		if trickyAlteration {
			db.Exec("pragma foreign_keys=on;")
		}

		if err != nil {
			return err
		}

		log.Printf("[migration:%d] done", v)
	}
	return nil
}

func migrateVersion(v int64, db *sql.DB) error {
	var err error
	var tx *sql.Tx
	migratefunc := migrations[v-1]
	if tx, err = db.Begin(); err != nil {
		log.Printf("[migration:%d] failed to start transaction", v)
		return err
	}
	if err = migratefunc(tx); err != nil {
		log.Printf("[migration:%d] failed to migrate", v)
		tx.Rollback()
		return err
	}
	if _, err = tx.Exec(fmt.Sprintf("pragma user_version = %d", v)); err != nil {
		log.Printf("[migration:%d] failed to bump version", v)
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		log.Printf("[migration:%d] failed to commit changes", v)
		return err
	}
	return nil
}

func m01_initial(tx *sql.Tx) error {
	sql := `
		create table if not exists folders (
		 id             integer primary key autoincrement,
		 title          text not null,
		 is_expanded    boolean not null default false
		);

		create unique index if not exists idx_folder_title on folders(title);

		create table if not exists feeds (
		 id             integer primary key autoincrement,
		 folder_id      references folders(id),
		 title          text not null,
		 description    text,
		 link           text,
		 feed_link      text not null,
		 icon           blob
		);

		create index if not exists idx_feed_folder_id on feeds(folder_id);
		create unique index if not exists idx_feed_feed_link on feeds(feed_link);

		create table if not exists items (
		 id             integer primary key autoincrement,
		 guid           string not null,
		 feed_id        references feeds(id),
		 title          text,
		 link           text,
		 description    text,
		 content        text,
		 author         text,
		 date           datetime,
		 date_updated   datetime,
		 date_arrived   datetime,
		 status         integer,
		 image          text,
		 search_rowid   integer
		);

		create index if not exists idx_item_feed_id on items(feed_id);
		create index if not exists idx_item_status  on items(status);
		create index if not exists idx_item_search_rowid on items(search_rowid);
		create unique index if not exists idx_item_guid on items(feed_id, guid);

		create table if not exists settings (
		 key            string primary key,
		 val            blob
		);

		create virtual table if not exists search using fts4(title, description, content);

		create trigger if not exists del_item_search after delete on items begin
		  delete from search where rowid = old.search_rowid;
		end;
	`
	_, err := tx.Exec(sql)
	return err
}

func m02_feed_states_and_errors(tx *sql.Tx) error {
	sql := `
		create table if not exists http_states (
		 feed_id        references feeds(id) unique,
		 last_refreshed datetime not null,

		 -- http header fields --
		 last_modified  string not null,
		 etag           string not null
		);

		create table if not exists feed_errors (
		 feed_id        references feeds(id) unique,
		 error          string
		);
	`
	_, err := tx.Exec(sql)
	return err
}

func m03_on_delete_actions(tx *sql.Tx) error {
	sql := `
		-- 01. create altered tables
		create table if not exists new_feeds (
		 id             integer primary key autoincrement,
		 folder_id      references folders(id) on delete set null,
		 title          text not null,
		 description    text,
		 link           text,
		 feed_link      text not null,
		 icon           blob
		);
		create table if not exists new_items (
		 id             integer primary key autoincrement,
		 guid           string not null,
		 feed_id        references feeds(id) on delete cascade,
		 title          text,
		 link           text,
		 description    text,
		 content        text,
		 author         text,
		 date           datetime,
		 date_updated   datetime,
		 date_arrived   datetime,
		 status         integer,
		 image          text,
		 search_rowid   integer
		);
		create table if not exists new_http_states (
		 feed_id        references feeds(id) on delete cascade unique,
		 last_refreshed datetime not null,
		 last_modified  string not null,
		 etag           string not null
		);
		create table if not exists new_feed_errors (
		 feed_id        references feeds(id) on delete cascade unique,
		 error          string
		);

		-- 02. transfer data into new tables
		insert into new_feeds select * from feeds;
		insert into new_items select * from items;
		insert into new_http_states select * from http_states;
		insert into new_feed_errors select * from feed_errors;

		-- 03. drop old tables
		drop table feeds;
		drop table items;
		drop table http_states;
		drop table feed_errors;

		-- 04. rename new tables
		alter table new_feeds rename to feeds;
		alter table new_items rename to items;
		alter table new_http_states rename to http_states;
		alter table new_feed_errors rename to feed_errors;

		-- 05. reconstruct indexes & triggers
		create index if not exists idx_feed_folder_id on feeds(folder_id);
		create unique index if not exists idx_feed_feed_link on feeds(feed_link);
		create index if not exists idx_item_feed_id on items(feed_id);
		create index if not exists idx_item_status  on items(status);
		create index if not exists idx_item_search_rowid on items(search_rowid);
		create unique index if not exists idx_item_guid on items(feed_id, guid);
		create trigger if not exists del_item_search after delete on items begin
		  delete from search where rowid = old.search_rowid;
		end;

		-- 06. check consistency
		pragma foreign_key_check;
	`
	_, err := tx.Exec(sql)
	return err
}

func m04_item_podcasturl(tx *sql.Tx) error {
	sql := `
		alter table items add column podcast_url text
	`
	_, err := tx.Exec(sql)
	return err
}

func m05_move_description_to_content(tx *sql.Tx) error {
	sql := `
		update items set content=description
		where length(content) = 0 or content is null
	`
	_, err := tx.Exec(sql)
	return err
}

func m06_fill_missing_dates(tx *sql.Tx) error {
	sql := `
		update items set date = 0 where date is null;
	`
	_, err := tx.Exec(sql)
	return err
}

func m07_add_feed_size(tx *sql.Tx) error {
	sql := `
		create table if not exists feed_sizes (
		 feed_id        references feeds(id) on delete cascade unique,
		 size           integer not null default 0
		);
	`
	_, err := tx.Exec(sql)
	return err
}

func m08_normalize_datetime(tx *sql.Tx) error {
	rows, err := tx.Query(`select id, date_arrived from items;`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var dateArrived time.Time
		err = rows.Scan(&id, &dateArrived)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`update items set date_arrived = ? where id = ?;`, dateArrived.UTC(), id)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(`update items set date = strftime('%Y-%m-%d %H:%M:%f', date);`)
	return err
}

func m09_change_item_index(tx *sql.Tx) error {
	sql := `
        drop index if exists idx_item_status;
		create index if not exists idx_item__date_id_status on items(date,id,status);
	`
	_, err := tx.Exec(sql)
	return err
}

func m10_add_item_medialinks(tx *sql.Tx) error {
	sql := `
		alter table items add column media_links json;
		update items set media_links = case
			when coalesce(image, '') != '' and coalesce(podcast_url, '') != ''
			then json_array(json_object('type', 'image', 'url', image), json_object('type', 'audio', 'url', podcast_url))

			when coalesce(image, '') != ''
			then json_array(json_object('type', 'image', 'url', image))

			when coalesce(podcast_url, '') != ''
			then json_array(json_object('type', 'audio', 'url', podcast_url))

			else null
		end;
		alter table items drop column image;
		alter table items drop column podcast_url;
	`
	_, err := tx.Exec(sql)
	return err
}

func m11_add_feed_content_selector(tx *sql.Tx) error {
	sql := `
		alter table feeds add column content_selector text not null default '';
	`
	_, err := tx.Exec(sql)
	return err
}

func m12_replace_feed_icon_with_icon_url(tx *sql.Tx) error {
	sql := `
		alter table feeds add column icon_url text not null default '';
		alter table feeds drop column icon;
	`
	_, err := tx.Exec(sql)
	return err
}

func m13_add_feed_content_mode(tx *sql.Tx) error {
	sql := `
		alter table feeds add column content_mode text not null default 'normal';
	`
	_, err := tx.Exec(sql)
	return err
}

func m14_add_generated_rss(tx *sql.Tx) error {
	sql := `
		create table if not exists generated_rss_sources (
		 id             integer primary key autoincrement,
		 key            text not null unique,
		 title          text not null,
		 link           text not null,
		 description    text not null,
		 created_at     datetime not null,
		 updated_at     datetime not null
		);

		create table if not exists generated_rss_items (
		 id             integer primary key autoincrement,
		 source_id      references generated_rss_sources(id) on delete cascade,
		 guid           text not null,
		 title          text not null,
		 link           text not null,
		 content        text not null,
		 published_at   datetime not null,
		 created_at     datetime not null,
		 updated_at     datetime not null,
		 unique(source_id, guid)
		);

		create index if not exists idx_generated_rss_items_source_published
		on generated_rss_items(source_id, published_at desc, id desc);
	`
	_, err := tx.Exec(sql)
	return err
}

func m15_add_feed_ranking_mode(tx *sql.Tx) error {
	sql := `
		alter table feeds add column ranking_mode boolean not null default false;
		drop table if exists generated_rss_items;
		drop table if exists generated_rss_sources;
	`
	_, err := tx.Exec(sql)
	return err
}

func m16_change_feed_ranking_mode_to_text(tx *sql.Tx) error {
	sql := `
		create table if not exists new_feeds (
		 id               integer primary key autoincrement,
		 folder_id        references folders(id) on delete set null,
		 title            text not null,
		 description      text,
		 link             text,
		 feed_link        text not null,
		 content_selector text not null default '',
		 icon_url         text not null default '',
		 content_mode     text not null default 'normal',
		 ranking_mode     text not null default 'off'
		);

		insert into new_feeds (
			id, folder_id, title, description, link, feed_link,
			content_selector, icon_url, content_mode, ranking_mode
		)
		select
			id, folder_id, title, description, link, feed_link,
			content_selector, icon_url, content_mode,
			case
				when ranking_mode = true or ranking_mode = 'true' or ranking_mode = '1' then 'with_image'
				when ranking_mode in ('with_image', 'without_image') then ranking_mode
				else 'off'
			end
		from feeds;

		drop table feeds;
		alter table new_feeds rename to feeds;

		create index if not exists idx_feed_folder_id on feeds(folder_id);
		create unique index if not exists idx_feed_feed_link on feeds(feed_link);
		pragma foreign_key_check;
	`
	_, err := tx.Exec(sql)
	return err
}

func m17_add_item_list_indexes(tx *sql.Tx) error {
	sql := `
		create index if not exists idx_item_status_date_id on items(status, date desc, id desc);
		create index if not exists idx_item_feed_date_id on items(feed_id, date desc, id desc);
		create index if not exists idx_item_feed_status_date_id on items(feed_id, status, date desc, id desc);
	`
	_, err := tx.Exec(sql)
	return err
}

func m18_remove_feed_setting(tx *sql.Tx) error {
	_, err := tx.Exec(`delete from settings where key = 'feed';`)
	return err
}

func m19_add_feed_last_ranking_state(tx *sql.Tx) error {
	if err := addColumnIfMissing(tx, "feeds", "last_ranking_item", "last_ranking_item text not null default ''"); err != nil {
		return err
	}
	return addColumnIfMissing(tx, "feeds", "last_ranking_md5", "last_ranking_md5 text not null default ''")
}

func m20_remove_frontend_settings(tx *sql.Tx) error {
	_, err := tx.Exec(`
		delete from settings
		where key in ('theme_name', 'theme_font', 'toolbar_display', 'sort_newest_first');
	`)
	return err
}

func m21_add_auto_read_scroll(tx *sql.Tx) error {
	if err := addColumnIfMissing(tx, "feeds", "auto_read_scroll", "auto_read_scroll boolean not null default true"); err != nil {
		return err
	}
	return addColumnIfMissing(tx, "folders", "auto_read_scroll", "auto_read_scroll boolean not null default true")
}

func m22_add_feed_latest_item_arrived_at(tx *sql.Tx) error {
	if err := addColumnIfMissing(tx, "feeds", "latest_item_arrived_at", "latest_item_arrived_at datetime"); err != nil {
		return err
	}
	_, err := tx.Exec(`
		update feeds
		set latest_item_arrived_at = (
			select max(date_arrived)
			from items
			where items.feed_id = feeds.id
		);

		create index if not exists idx_feed_latest_item_arrived_at
		on feeds(latest_item_arrived_at desc);

		create trigger if not exists update_feed_latest_item_arrived_at
		after insert on items
		when new.date_arrived is not null
		begin
			update feeds
			set latest_item_arrived_at = case
				when latest_item_arrived_at is null or new.date_arrived > latest_item_arrived_at
				then new.date_arrived
				else latest_item_arrived_at
			end
			where id = new.feed_id;
		end;
	`)
	return err
}

func m23_add_feed_custom_icon(tx *sql.Tx) error {
	return addColumnIfMissing(tx, "feeds", "custom_icon", "custom_icon boolean not null default false")
}

func m24_add_favorite_and_item_order(tx *sql.Tx) error {
	if err := addColumnIfMissing(tx, "items", "favorite", "favorite boolean not null default false"); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "feeds", "unread_first", "unread_first boolean not null default true"); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "feeds", "sort_newest_first", "sort_newest_first boolean not null default true"); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "folders", "unread_first", "unread_first boolean not null default true"); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "folders", "sort_newest_first", "sort_newest_first boolean not null default true"); err != nil {
		return err
	}
	_, err := tx.Exec(`
		update items set status = 1, favorite = true where status = 2;
		update settings set val = '"favorite"' where key = 'filter' and val = '"starred"';

		create index if not exists idx_item_favorite_status_date_id
		on items(favorite, status, date desc, id desc);

		create index if not exists idx_item_feed_favorite_status_date_id
		on items(feed_id, favorite, status, date desc, id desc);
	`)
	return err
}

func m25_add_feed_refresh_times(tx *sql.Tx) error {
	if err := addColumnIfMissing(tx, "feeds", "last_refreshed_at", "last_refreshed_at datetime"); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "feeds", "last_refresh_succeeded_at", "last_refresh_succeeded_at datetime"); err != nil {
		return err
	}
	_, err := tx.Exec(`
		update feeds
		set last_refreshed_at = (
				select last_refreshed from http_states where http_states.feed_id = feeds.id
			),
			last_refresh_succeeded_at = (
				select last_refreshed from http_states where http_states.feed_id = feeds.id
			)
		where exists (select 1 from http_states where http_states.feed_id = feeds.id);

		create index if not exists idx_feed_last_refresh_succeeded_at
		on feeds(last_refresh_succeeded_at);
	`)
	return err
}

func addColumnIfMissing(tx *sql.Tx, table, column, definition string) error {
	rows, err := tx.Query(fmt.Sprintf(`pragma table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = tx.Exec(fmt.Sprintf(`alter table %s add column %s`, table, definition))
	return err
}

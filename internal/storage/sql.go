package storage

const tablesSQL = `
      CREATE TABLE IF NOT EXISTS dir (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT UNIQUE NOT NULL  -- Unique identifier for the zettel
      );

      CREATE TABLE IF NOT EXISTS zettel (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,            -- Name of the file
        title TEXT NOT NULL,           -- File body
        body TEXT NOT NULL,            -- File body
        mtime TEXT NOT NULL,           -- Last modification time
        dir_name TEXT NOT NULL,        -- Name of the directory this file belongs to
        FOREIGN KEY(dir_name) REFERENCES dir(name) -- Reference to parent directory
      );

      -- Table for storing zettel links
      CREATE TABLE IF NOT EXISTS link (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        content TEXT NOT NULL,
        from_zettel_id INTEGER NOT NULL,
        to_zettel_id INTEGER NOT NULL,
        UNIQUE(content, from_zettel_id, to_zettel_id),
        FOREIGN KEY(from_zettel_id) REFERENCES zettel(id) ON DELETE CASCADE,
        FOREIGN KEY(to_zettel_id) REFERENCES zettel(id) ON DELETE CASCADE
      );

      -- Table for storing zettel tag
      CREATE TABLE IF NOT EXISTS tag (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL UNIQUE
      );

      -- Many-to-many relationship table between zettels and tags
      CREATE TABLE IF NOT EXISTS zettel_tags (
        zettel_id INTEGER NOT NULL,            -- ID of the zettel
        tag_id INTEGER NOT NULL,               -- ID of the tag
        PRIMARY KEY(zettel_id, tag_id),        -- Composite primary key
        FOREIGN KEY(zettel_id) REFERENCES zettel(id) ON DELETE CASCADE,
        FOREIGN KEY(tag_id) REFERENCES tag(id) ON DELETE CASCADE
      );

      CREATE VIRTUAL TABLE IF NOT EXISTS zettel_fts USING fts5(
        title,
        body,
        tags,
        tokenize='porter'  -- This uses the Porter stemming algorithm
      );

      -- Insert trigger for zettel table
      CREATE TRIGGER IF NOT EXISTS ai_zettel AFTER INSERT ON zettel BEGIN
        INSERT INTO zettel_fts(rowid, title, body, tags) VALUES (new.id, new.title, new.body, (
            SELECT GROUP_CONCAT(name, ' ')
            FROM tag
            JOIN zettel_tags ON tag.id = zettel_tags.tag_id
            WHERE zettel_tags.zettel_id = new.id
          )
        );
      END;

      -- Update trigger for zettel table
      CREATE TRIGGER IF NOT EXISTS au_zettel AFTER UPDATE ON zettel BEGIN
          UPDATE zettel_fts SET title = new.title, body = new.body, tags = (
            SELECT GROUP_CONCAT(name, ' ')
            FROM tag
            JOIN zettel_tags ON tag.id = zettel_tags.tag_id
            WHERE zettel_tags.zettel_id = new.id
          )
          WHERE rowid = old.id;
      END;

      -- Delete trigger for zettel table
      CREATE TRIGGER IF NOT EXISTS ad_zettel AFTER DELETE ON zettel BEGIN
          DELETE FROM zettel_fts WHERE rowid = old.id;
      END;

      -- Insert trigger for zettel_tags table
      CREATE TRIGGER IF NOT EXISTS ai_zettel_tags AFTER INSERT ON zettel_tags BEGIN
          UPDATE zettel_fts SET tags = (
            SELECT GROUP_CONCAT(name, ' ')
            FROM tag
            JOIN zettel_tags ON tag.id = zettel_tags.tag_id
            WHERE zettel_tags.zettel_id = new.zettel_id
          )
          WHERE rowid = new.zettel_id;
      END;

      -- Update trigger for zettel_tags table
      CREATE TRIGGER IF NOT EXISTS au_zettel_tags AFTER UPDATE ON zettel_tags BEGIN
          UPDATE zettel_fts SET tags = (
            SELECT GROUP_CONCAT(name, ' ')
            FROM tag
            JOIN zettel_tags ON tag.id = zettel_tags.tag_id
            WHERE zettel_tags.zettel_id = new.zettel_id
          )
          WHERE rowid = new.zettel_id;
      END;

      -- Delete trigger for zettel_tags table
      CREATE TRIGGER IF NOT EXISTS ad_zettel_tags AFTER DELETE ON zettel_tags BEGIN
          UPDATE zettel_fts SET tags = (
            SELECT GROUP_CONCAT(name, ' ')
            FROM tag JOIN zettel_tags ON tag.id = zettel_tags.tag_id
            WHERE zettel_tags.zettel_id = old.zettel_id
          )
          WHERE rowid = old.zettel_id;
      END;

      PRAGMA foreign_keys = ON;
      `

// Package storage provides the functionality for interacting with the
// zet database.
package storage

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type Storage struct {
	db *sqlx.DB
}

type dir struct {
	ID   int    `db:"id"`   // unique id
	Name string `db:"name"` // unique directory name
}

type Zettel struct {
	ID      int    `db:"id"`    // unique id
	Name    string `db:"name"`  // name of file
	Title   string `db:"title"` // title of file
	Body    string `db:"body"`  // body of file
	Links   []Link // links to other zettels
	Tags    []Tag  // zettels tags
	Mtime   string `db:"mtime"`    // modification time
	DirName string `db:"dir_name"` // modification time
}

type Tag struct {
	ID   int    `db:"id"`   // unique tag id
	Name string `db:"name"` // unique tag name
}

type Link struct {
	ID           int    `db:"id"`             // unique link id
	Content      string `db:"content"`        // zettel link
	FromZettelID int    `db:"from_zettel_id"` // zettel id where link lives
	ToZettelID   int    `db:"to_zettel_id"`   // zettel id where link points to
}

// AllZettels returns all existing zettel files.
func (s *Storage) AllZettels() ([]Zettel, error) {
	const query = `SELECT * FROM zettel;`
	zettels := []Zettel{}
	if err := s.db.Select(&zettels, query); err != nil {
		return nil, fmt.Errorf("Error getting zettels records: %v", err)
	}
	// Fetch tags and links for this zettel
	for _, z := range zettels {
		if err := zettelTags(s.db, &z); err != nil {
			return nil, fmt.Errorf("Error getting tags: %v", err)
		}
		if err := zettelLinks(s.db, &z); err != nil {
			return nil, fmt.Errorf("Error getting links: %v", err)
		}
	}
	return zettels, nil
}

// SearchZettels searches the zettelkasten for zettels matching the query.
// It returns a slice of Zettels.
func (s *Storage) SearchZettels(term string) ([]Zettel, error) {
	var zIDs []int
	const query = `
  	SELECT id
  	FROM zettel_fts
  	WHERE zettel_fts MATCH $1
  	ORDER BY bm25(zettel_fts, 1.5, 1.0, 1.5);
  `
	if err := s.db.Select(&zIDs, query, term); err != nil {
		return nil, err
	}

	z := []Zettel{}
	// TODO execute sql query to fill zettel fields given list of ids
	return z, nil
}

// UpdateDB initializes the database, retrieve zet state from the
// database, and updates the database to sync the flat files and the
// data storage.
func UpdateDB(zetPath string) (*Storage, error) {
	s, err := Init()
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize database: %v.\n", err)
	}
	db := s.db

	zm, err := s.zettelsMap()
	if err != nil {
		return nil, fmt.Errorf("Failed to get zettels: %v.\n", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("Failed to create transaction: %v\n", err)
	}

	if err := processZettels(tx, zetPath, zm); err != nil {
		return nil, fmt.Errorf("Failed to process zettels: %v.\n", err)
	}

	return s, tx.Commit()
}

// Init creates the database if it doesn't exist and returns the
// database connection.
func Init() (*Storage, error) {
	// Resolve zet database path.
	dbPath := os.Getenv("ZET_DB_PATH")
	if dbPath == "" {
		return nil, errors.New("Environment variable ZET_DB_PATH must be set")
	}

	// Connect to SQLite database
	db, err := sqlx.Connect("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to database: %v", err)
	}

	// note: search history should probably be capped.
	const query = `
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

	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}

	return &Storage{db: db}, nil
}

func (s *Storage) Close() {
	s.db.Close()
}

// zettelsMap retrieves all existing zettels from the database
// and put them into a map. It returns a map that includes each zettel
// directory and all non-directory files. The value is a file struct.
func (s *Storage) zettelsMap() (map[string]map[string]Zettel, error) {
	// Map to hold existing zettels and their files from the database
	var zm = make(map[string]map[string]Zettel)
	zettels := []Zettel{}

	zettels, err := s.AllZettels()
	if err != nil {
		return zm, fmt.Errorf("Failed to get all zettels: %v", err)
	}

	for _, z := range zettels {
		if _, exists := zm[z.DirName]; !exists {
			zm[z.DirName] = make(map[string]Zettel)
		}

		zm[z.DirName][z.Name] = z
	}

	return zm, nil
}

// processZettels iterates over each zettel directory and its files to
// keep the flat files and database in sync.
func processZettels(tx *sqlx.Tx, zetPath string, zm map[string]map[string]Zettel) error {
	dirs, err := os.ReadDir(zetPath)
	if err != nil {
		return fmt.Errorf("Error reading root directory: %v", err)
	}

	// Scan the root directory
	for _, dir := range dirs {
		// Skip any files not directory type
		if !dir.IsDir() {
			continue
		}

		dirName := dir.Name()
		dirPath := filepath.Join(zetPath, dirName)

		// Check if this is a new or existing zettel.
		_, exists := zm[dirName]

		// For *new* zettels: Insert the dir and all its files (that
		// aren't a dir) into the database.
		if !exists {
			if err := addZettel(tx, dirPath); err != nil {
				log.Printf("Failed to insert a zettel: %v.\n", err)
			}
			continue // Move to the next directory
		}

		// For *existing* zettel, process its files.
		err = processFiles(tx, dirPath, zm)
		if err != nil {
			return err
		}

		// Mark this zettel directory as visited.
		delete(zm, dirName)
	}

	// Delete any remaining zettels
	if err := deleteZettels(tx, zm); err != nil {
		log.Printf("Failed to delete a zettel: %v.\n", err)
	}

	return nil
}

// addZettel inserts a zettel directory into the database. This is
// performed by inserting the zettel directory into the dirs table and
// all of its files into the files table.
func addZettel(tx *sqlx.Tx, dirPath string) error {
	dirName := path.Base(dirPath)
	if err := insertDir(tx, dirName); err != nil {
		return fmt.Errorf("Error inserting directory: %v", err)
	}

	// Fetch files inside this directory
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("Error reading sub-directory: %v", err)
	}

	// For each file that is NOT a directory:
	// If new file Add new files or update existing files in the database.
	for _, file := range files {
		z := Zettel{}
		z.Name = file.Name()
		// Filter out sub-directories and files that are not markdown.
		if !strings.HasSuffix(z.Name, ".md") || file.IsDir() {
			continue
		}
		z.DirName = dirName

		info, err := file.Info()
		if err != nil {
			return fmt.Errorf("Error reading file info: %v", err)
		}
		modTime := info.ModTime().Truncate(time.Second)
		z.Mtime = modTime.Format(time.RFC3339)

		fp := filepath.Join(dirPath, z.Name)
		contentBytes, err := os.ReadFile(fp)
		if err != nil {
			return err
		}

		content := string(contentBytes)
		splitZettel(tx, &z, content)

		if err := insertFile(tx, z); err != nil {
			return fmt.Errorf("Failed to insert new file: %v", err)
		}
	}

	return nil
}

// deleteZettels deletes given zettels from the database.
func deleteZettels(tx *sqlx.Tx, zm map[string]map[string]Zettel) error {
	// Delete the files in each dir and then delete the dir.
	for _, filesMap := range zm {
		if err := deleteFiles(tx, filesMap); err != nil {
			return fmt.Errorf("Failed to delete files: %v", err)
		}
	}
	if err := deleteDirs(tx, zm); err != nil {
		log.Printf("Failed to delete a zettel: %v.\n", err)
	}

	return nil
}

// insertDir inserts a directory into the dirs database table.
func insertDir(tx *sqlx.Tx, n string) error {
	const query = `
    INSERT INTO dir (name)
    VALUES ($1);
    `
	_, err := tx.Exec(query, n)
	return err
}

// processFiles iterates over a zettel directory and inserts new files,
// updates modified files, and remove deleted files.
func processFiles(tx *sqlx.Tx, dirPath string, zm map[string]map[string]Zettel) error {
	dirName := path.Base(dirPath)

	// Retrieve the map for all the files for a given zettel directory
	// and validate that the directory exists.
	existingFiles, exists := zm[dirName]
	// If the directory doesn't exist, return an error.
	if !exists {
		return fmt.Errorf("Failed to process files in %s: Directory doesn't exist.\n", dirPath)
	}

	// Fetch files inside this directory
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("Error reading sub-directory: %v", err)
	}
	// For each file that is NOT a directory:
	// If new file Add new files or update existing files in the database.
	for _, file := range files {
		z := Zettel{}
		// Filter sub-directories and out any files that are not markdown.
		z.Name = file.Name()
		if file.IsDir() || !strings.HasSuffix(z.Name, ".md") {
			continue
		}
		z.DirName = dirName

		info, err := file.Info()
		if err != nil {
			return fmt.Errorf("Error reading file info: %v", err)
		}
		modTime := info.ModTime().Truncate(time.Second)
		z.Mtime = modTime.Format(time.RFC3339)

		// Check if this is a new or existing file for this particular
		// zettel.
		f, exists := existingFiles[z.Name]
		ft, err := isoToTime(f.Mtime)
		if err != nil {
			return err
		}

		fp := filepath.Join(dirPath, z.Name)
		contentBytes, err := os.ReadFile(fp)
		if err != nil {
			return err
		}

		content := string(contentBytes)
		splitZettel(tx, &z, content)

		// If the file doesn't exist in this zettel, insert it into the
		// database.
		if !exists {
			if err := insertFile(tx, z); err != nil {
				return fmt.Errorf("Failed to insert new file: %v", err)
			}
			continue
		}

		// If the file has been modified since last recorded, make the
		// database update operation.
		if modTime.After(ft) {
			err := updateFile(tx, z)
			if err != nil {
				return fmt.Errorf("Failed to update file record: %v", err)
			}
		}

		// Mark this file in the zettel as visited.
		delete(zm[dirName], z.Name)
	}

	err = deleteFiles(tx, existingFiles)
	if err != nil {
		return fmt.Errorf("Failed to delete files: %v", err)
	}

	return nil
}

// splitZettel breaks a zettel's contents and assigns it's parts to
// associated fields: title, body, links, and tags.
func splitZettel(tx *sqlx.Tx, z *Zettel, content string) {
	var bodyLines []string
	isBody := false
	// Match lines that contain a link. E.g., `* [dir][../dir] title`
	linkRegex := regexp.MustCompile(`\[(.+)\]\(\.\./(.*?)/?\) (.+)`)
	tagRegex := regexp.MustCompile(`^(?:\t\t| {4})#[a-zA-Z]+`)

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		// Is line the title?
		if z.Title == "" && strings.HasPrefix(line, `# `) {
			z.Title = strings.TrimPrefix(line, `# `)
			isBody = true
			continue
		}

		// Is line a markdown link?
		matches := linkRegex.FindStringSubmatch(line)
		if len(matches) > 0 {
			iso := matches[1]
			id, err := zettelIdDir(tx, iso)
			if err != nil {
				// If referenced zettel id couldn't be found, skip link
				continue
			}
			l := Link{Content: matches[0], ToZettelID: id}
			z.Links = append(z.Links, l)
			continue
		}

		// Is line the tag line?
		if tagRegex.MatchString(line) {
			tagLine := strings.TrimPrefix(line, "\t\t")
			ts := strings.Split(tagLine, ` `)

			for _, t := range ts {
				if strings.HasPrefix(t, `#`) {
					tt := strings.TrimPrefix(t, `#`)
					z.Tags = append(z.Tags, Tag{Name: tt})
				}
				// If tag doesn't start with `#`, skip it.
			}
			continue
		}

		// Everything else is considered as body.
		if isBody {
			bodyLines = append(bodyLines, line)
		}
	}

	z.Body = strings.Join(bodyLines, "\n")
}

// zettelIdDir retrieves and returns the zettel using a given unique
// isosec (director name).
func zettelIdDir(tx *sqlx.Tx, iso string) (int, error) {
	const query = `SELECT id FROM zettel WHERE dir_name=$1 LIMIT 1;`
	var id int
	err := tx.Get(&id, query, iso)
	return id, err
}

// insertFile inserts a new file into the database.
func insertFile(tx *sqlx.Tx, z Zettel) error {
	const (
		insertZettelSQL = `
    INSERT INTO zettel (name, title, body, mtime, dir_name)
    VALUES ($1, $2, $3, $4, $5)
		RETURNING id;`
		insertLinksSQL = `
		INSERT INTO link (content, from_zettel_id, to_zettel_id)
		VALUES ($1, $2, $3);`
	)
	var id int
	err := tx.QueryRow(insertZettelSQL, z.Name, z.Title, z.Body, z.Mtime, z.DirName).Scan(&id)
	if err != nil {
		return fmt.Errorf("Error inserting zettel record: %v", err)
	}

	// Insert any new links
	for _, l := range z.Links {
		_, err = tx.Exec(insertLinksSQL, l.Content, id, l.ToZettelID)
		if err != nil {
			return fmt.Errorf("Error inserting links: %v", err)
		}
	}

	if err := insertTags(tx, id, z.Tags); err != nil {
		return fmt.Errorf("Error inserting tags: %v", err)
	}

	return nil
}

// updateFile updates a file in the database given directory and file
// name.
func updateFile(tx *sqlx.Tx, z Zettel) error {
	const (
		idQuery = `SELECT id FROM zettel
			WHERE name=$1 AND dir_name=$2`
		zettelQuery = `
    	UPDATE zettel SET title=$1, body=$2, mtime=$3
			WHERE id=$4;`
	)
	var id int
	err := tx.Get(&id, idQuery, z.Name, z.DirName)
	if err != nil {
		return fmt.Errorf("Failed to get zettel id: %v", err)
	}

	// Update zettel table record
	_, err = tx.Exec(zettelQuery, z.Title, z.Body, z.Mtime, id)
	if err != nil {
		return fmt.Errorf("Error updating zettel table record: %v", err)
	}
	if err := updateLinks(tx, z); err != nil {
		return fmt.Errorf("Error updating links: %v", err)
	}
	if err := updateTags(tx, z); err != nil {
		return fmt.Errorf("Error updating tags: %v", err)
	}

	return err
}

// updateLinks updates links for a given zettel.
func updateLinks(tx *sqlx.Tx, z Zettel) error {
	cl, err := currLinks(tx, z.ID)
	if err != nil {
		return fmt.Errorf("Error retrieving links: %v", err)
	}
	add, remove := diffLinks(cl, z.Links)
	if err := addLinks(tx, z.ID, add); err != nil {
		return fmt.Errorf("Error inserting links: %v", err)
	}
	if err := removeLinks(tx, z.ID, remove); err != nil {
		return fmt.Errorf("Error removing links: %v", err)
	}
	return nil
}

// currLinks retrieves the current links for a given zettel id.
func currLinks(tx *sqlx.Tx, zettelID int) ([]Link, error) {
	var l []Link
	const query = `SELECT * FROM link WHERE from_zettel_id=$1`
	if err := tx.Select(&l, query, zettelID); err != nil {
		return nil, fmt.Errorf("Error retrieving zettel links: %v", err)
	}
	return l, nil
}

// diffLinks determines which links to add and which to remove for a
// single zettel.
func diffLinks(cl, nl []Link) ([]Link, []Link) {
	var add, remove []Link

	// Create map of current links
	currLinksMap := make(map[string]bool)
	for _, link := range cl {
		currLinksMap[link.Content] = true
	}

	// Find links to add
	for _, link := range nl {
		if !currLinksMap[link.Content] {
			add = append(add, link)
		}
	}

	// Create map of new links
	newLinksMap := make(map[string]bool)
	for _, link := range nl {
		newLinksMap[link.Content] = true
	}

	// Find links to remove
	for _, link := range cl {
		if !newLinksMap[link.Content] {
			remove = append(remove, link)
		}
	}

	return add, remove
}

// addLinks inserts links for a given zettel id.
func addLinks(tx *sqlx.Tx, zettelID int, links []Link) error {
	const query = `
			INSERT INTO link (content, from_zettel_id, to_zettel_id)
			VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	for _, l := range links {
		_, err := tx.Exec(query, l.Content, zettelID, l.ToZettelID)
		if err != nil {
			return fmt.Errorf("Failed to insert zettel links: %v", err)
		}
	}
	return nil
}

// removeLinks deletes links for a given zettel id.
func removeLinks(tx *sqlx.Tx, fromZettelID int, links []Link) error {
	const query = `DELETE FROM link WHERE id=$1 AND from_zettel_id=$2`
	for _, l := range links {
		_, err := tx.Exec(query, l.ID, fromZettelID)
		if err != nil {
			return fmt.Errorf("Failed to remove zettel links: %v", err)
		}
	}
	return nil
}

// updateTags updates tags for a given zettel.
func updateTags(tx *sqlx.Tx, z Zettel) error {
	ct, err := currTags(tx, z.ID)
	if err != nil {
		return fmt.Errorf("Error retrieving tags: %v", err)
	}
	add, remove := diffTags(ct, z.Tags)
	if err := insertTags(tx, z.ID, add); err != nil {
		return fmt.Errorf("Error inserting tags: %v", err)
	}
	if err := removeTagLinks(tx, z.ID, remove); err != nil {
		return fmt.Errorf("Error removing zettel-tag association: %v", err)
	}
	if err := cleanTags(tx); err != nil {
		return err
	}
	return nil
}

// currTags retrieves the current tags for a given zettel id.
func currTags(tx *sqlx.Tx, id int) ([]Tag, error) {
	var ct []Tag
	const query = `
	SELECT t.* FROM tag t
	INNER JOIN zettel_tags zt ON t.id = zt.tag_id
	WHERE zt.zettel_id=$1;`
	if err := tx.Select(&ct, query, id); err != nil {
		return nil, err
	}
	return ct, nil
}

// diffTags determines which tags to add and which to remove for a
// single zettel.
func diffTags(ct, nt []Tag) ([]Tag, []Tag) {
	var add, remove []Tag

	// Create map of existing tags
	currTagsMap := make(map[string]bool)
	for _, tag := range ct {
		currTagsMap[tag.Name] = true
	}

	// Find tags to add
	for _, tag := range nt {
		if !currTagsMap[tag.Name] {
			add = append(add, tag)
		}
	}

	// Create map of new tags
	newTagsMap := make(map[string]bool)
	for _, tag := range nt {
		newTagsMap[tag.Name] = true
	}

	// Find tags to remove
	for _, tag := range ct {
		if !newTagsMap[tag.Name] {
			remove = append(remove, tag)
		}
	}

	return add, remove
}

// insertTags inserts new tags into tag table if they don't exist and
// creates associations in the zettel tags table. The given zettel id
// is used to create the zettel-tag associations.
func insertTags(tx *sqlx.Tx, zettelID int, tags []Tag) error {
	const (
		insertTagSQL = `INSERT INTO tag (name)
			VALUES ($1) ON CONFLICT(name)
			DO NOTHING RETURNING id`
		selectTagIDSQL = `SELECT id FROM tag
			WHERE name = $1`
		insertZettelTagSQL = `INSERT INTO zettel_tags (zettel_id, tag_id)
			VALUES ($1, $2)`
	)

	for _, tag := range tags {
		var tagID int

		// Try to insert the tag into the tag table. If it already exists, do nothing.
		// If the tag is successfully inserted, its ID will be returned.
		err := tx.QueryRow(insertTagSQL, tag.Name).Scan(&tagID)
		if err != nil && err != sql.ErrNoRows {
			// If the error is not 'no rows in result set' then it's an actual error
			return err
		}

		// If the tag already exists, its ID wasn't returned, so retrieve it
		if err == sql.ErrNoRows {
			err = tx.QueryRow(selectTagIDSQL, tag.Name).Scan(&tagID)
			if err != nil {
				return err
			}
		}

		// Insert the zettel-tag association into the zettel_tags table
		_, err = tx.Exec(insertZettelTagSQL, zettelID, tagID)
		if err != nil {
			return err
		}
	}

	return nil // Return nil if everything went smoothly
}

// removeTagLinks removes tag associations for a zettel by deleting
// the record from the zettel_tags table where the zettel_id matches and the
// tag_id corresponds to the tag name provided.
func removeTagLinks(tx *sqlx.Tx, zettelID int, tags []Tag) error {
	const query = `
		DELETE FROM zettel_tags
		WHERE zettel_id = $1 AND
			tag_id = (
				SELECT id
				FROM tag
				WHERE name = $2
			);
	`
	for _, t := range tags {
		if _, err := tx.Exec(query, zettelID, t.Name); err != nil {
			return fmt.Errorf("Error removing a zettel-tag link: %v", err)
		}
	}
	return nil
}

// deleteFiles deletes any remaining files in an existing files map
// from the database. This removes files from a single zettel directory.
//
// Removing a zettel file may result in a tag that is no longer
// associated with any zettels. Thus, this function performs a clean up
// process that removes any orphaned tags.
func deleteFiles(tx *sqlx.Tx, zm map[string]Zettel) error {
	const query = `DELETE FROM zettel WHERE id = $1;`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Iterate through each remaining file in the directory
	for _, z := range zm {
		_, err := stmt.Exec(z.ID)
		if err != nil {
			// Log the error but continue deleting other files
			log.Printf("Error deleting file with id %d: %v", z.ID, err)
		}
	}

	if err := cleanTags(tx); err != nil {
		return err
	}

	return nil
}

// cleanTags removes any tags that are no longer associated with any
// zettels.
func cleanTags(tx *sqlx.Tx) error {
	const delTags = `
		DELETE FROM tag
		WHERE id NOT IN (
    	SELECT DISTINCT tag_id
    	FROM zettel_tags
		);
		`
	if _, err := tx.Exec(delTags); err != nil {
		return fmt.Errorf("Error cleaning up any orphaned tags: %v", err)
	}
	return nil
}

// deleteDirs deletes any remaining directories in an existing zettels map
// from the database. This removes directories (zettels) from the zet
// directory.
func deleteDirs(tx *sqlx.Tx, zm map[string]map[string]Zettel) error {
	const query = `DELETE FROM dir WHERE name = $1;`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Iterate through each remaining directory
	for dirName := range zm {
		_, err := stmt.Exec(dirName)
		if err != nil {
			// Log the error but continue deleting other directories
			log.Printf("Error deleting file with name %s: %v", dirName, err)
		}
	}
	return nil
}

// isoToTime converts a given ISO8601 string back to time.Time object
func isoToTime(t string) (time.Time, error) {
	mt, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return time.Time{}, err
	}
	return mt, nil
}

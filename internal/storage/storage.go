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

type ResultZettel struct {
	Zettel

	// TitleSnippet holds a snippet of the zettel's title as returned by
	// the SQLite snippet function. It's typically a substring of the
	// title that matches the search query, often with added context for
	// highlighting support.
	TitleSnippet string `db:"title_snippet"`

	// BodySnippet contains a snippet of the zettel's body as returned by
	// the SQLite snippet function. Similar to TitleSnippet, it includes
	// a part of the body text that matches the search criteria. If a
	// match was found, it will be surrounded by additional text to
	// support highlighting.
	BodySnippet string `db:"body_snippet"`

	// TagsSnippet holds a snippet of the zettel's tag line returned by the
	// SQLite snippet function. If a match was found, it will be
	// surrounded by additional text to support highlighting.
	TagsSnippet string `db:"tags_snippet"`
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

// AllZettels returns all existing zettel files with optional sorting.
// Optional argument should be a valid SQL ORDER BY clause, e.g., "mtime DESC".
func (s *Storage) AllZettels(sort string) ([]Zettel, error) {
	zettels := []Zettel{}
	query := `SELECT * FROM zettel`
	if sort != "" {
		query = fmt.Sprintf("%s ORDER BY %s", query, sort)
	}

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

// SearchZettels searches the zettelkasten for zettels matching the
// query. The before and after arguments are used for wrapping any
// matching text. It returns a slice of Zettels.
func (s *Storage) SearchZettels(term, before, after string) ([]ResultZettel, error) {
	term = preprocessInput(term)
	var results []ResultZettel

	query := `
					SELECT z.id, z.name, z.title, z.body, z.mtime, z.dir_name,
						COALESCE(highlight(zettel_fts, 0, '` + before + `', '` + after + `'), '') AS title_snippet,
						COALESCE(highlight(zettel_fts, 1, '` + before + `', '` + after + `'), '') AS body_snippet,
		      	COALESCE(highlight(zettel_fts, 2, '` + before + `', '` + after + `'), '') AS tags_snippet
					FROM zettel_fts
					JOIN zettel z ON zettel_fts.rowid = z.id
					WHERE zettel_fts MATCH LOWER($1)
					ORDER BY bm25(zettel_fts, 1.5, 1.0, 1.5);
			`

	if err := s.db.Select(&results, query, strings.ToLower(term)); err != nil {
		return nil, err
	}

	for i := range results {
		z := &results[i]
		if err := zettelTags(s.db, &z.Zettel); err != nil {
			return nil, fmt.Errorf("Error getting tags: %v", err)
		}
		if err := zettelLinks(s.db, &z.Zettel); err != nil {
			return nil, fmt.Errorf("Error getting links: %v", err)
		}
		z.BodySnippet = createSnippets(z.BodySnippet, before, after)
	}
	return results, nil
}

// createSnippets returns all lines that contain a match as a single
// string.
func createSnippets(body, before, after string) string {
	var builder strings.Builder
	lines := strings.Split(body, "\n")

	for i, line := range lines {
		if strings.Contains(line, before) && strings.Contains(line, after) {
			snippet := fmt.Sprintf("%d: %s\n", i+2, line)
			builder.WriteString(snippet)
		}
	}

	return builder.String()
}

// preprocessInput processes user input for fts5 search.
func preprocessInput(s string) string {
	s = preprocessTags(s)
	return s
}

// preprocessTags handles the conversion of "#tag" syntax into a
// FTS-friendly string.
func preprocessTags(s string) string {
	re := regexp.MustCompile(`#\w+`)
	return re.ReplaceAllStringFunc(s, func(tag string) string {
		tag = strings.TrimPrefix(tag, "#")
		return "tags:" + tag
	})
}

// zettelTags retrieves and assigns tags to the given zettel.
func zettelTags(db *sqlx.DB, z *Zettel) error {
	const tagQuery = `
			SELECT t.*
			FROM tag t
			JOIN zettel_tags zt ON t.id = zt.tag_id
			WHERE zt.zettel_id = $1;
	`
	return db.Select(&z.Tags, tagQuery, z.ID)
}

// zettelLinks retrieves and assigns zettel links to the given zettel.
func zettelLinks(db *sqlx.DB, z *Zettel) error {
	const linkQuery = `
			SELECT * FROM link
			WHERE from_zettel_id = $1;
	`
	return db.Select(&z.Links, linkQuery, z.ID)
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
	dbPath := os.Getenv("ZET_DB_PATH")
	if dbPath == "" {
		return nil, errors.New("environment variable ZET_DB_PATH must be set")
	}
	db, err := sqlx.Connect("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to database: %v", err)
	}
	if _, err = db.Exec(tablesSQL); err != nil {
		return nil, err
	}
	return &Storage{db: db}, nil
}

// Close closes th database connection.
func (s *Storage) Close() {
	s.db.Close()
}

// zettelsMap retrieves all existing zettels from the database
// and put them into a map. It returns a map that includes each zettel
// directory and all non-directory files. The value is a file struct.
// Note: This may not reflect the flat files exactly since empty
// directories are excluded from the database.
func (s *Storage) zettelsMap() (map[string]map[string]Zettel, error) {
	var zm = make(map[string]map[string]Zettel)
	zettels, err := s.AllZettels("")
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
// keep the flat files and database in sync. If a zettel directory does
// not have any files in it, its excluded from the database.
func processZettels(tx *sqlx.Tx, zetPath string, zm map[string]map[string]Zettel) error {
	dirs, err := os.ReadDir(zetPath)
	if err != nil {
		return fmt.Errorf("Error reading root directory: %v", err)
	}

	// Scan the root directory
	for _, dir := range dirs {
		// Skip any files not directory type and skip git directory.
		if !dir.IsDir() || dir.Name() == `.git` {
			continue
		}

		dirName := dir.Name()
		dirPath := filepath.Join(zetPath, dirName)

		// Check if this is a new or existing zettel.
		_, exists := zm[dirName]

		// If zettel is new, insert the directory and all its files (that
		// aren't a directory) into the database.
		if !exists {
			files, err := os.ReadDir(dirPath)
			if err != nil {
				return fmt.Errorf("Error reading sub-directory: %v", err)
			}
			if err := addZettel(tx, dirPath, files); err != nil {
				log.Printf("Failed to insert a zettel: %v. Dir name: %s\n", err, dirName)
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
// then attempting to insert all of its files into the files table.
// If the given directory has zero files or does not contain any
// README.md files, then this function does nothing.
func addZettel(tx *sqlx.Tx, dirPath string, files []os.DirEntry) error {
	if len(files) == 0 || !ContainsMD(files) {
		return nil
	}

	dirName := path.Base(dirPath)
	if err := insertDir(tx, dirName); err != nil {
		return fmt.Errorf("Error inserting directory: %v", err)
	}

	// For each file that is NOT a directory:
	// If new file, add new files or update existing files in the database.
	for _, file := range files {
		// Filter out sub-directories and files that are not markdown.
		if !strings.HasSuffix(file.Name(), ".md") || file.IsDir() {
			continue
		}

		z := Zettel{}
		z.DirName = dirName
		z.Name = file.Name()
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

// ContainsMD checks if a slice of files contains a README.md file.
func ContainsMD(files []os.DirEntry) bool {
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".md") {
			return true
		}
	}
	return false
}

// deleteZettels deletes given zettels from the database. It deletes the
// files in each directory and then deletes the directory.
func deleteZettels(tx *sqlx.Tx, zm map[string]map[string]Zettel) error {
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

		f, exists := existingFiles[z.Name]
		if !exists {
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
			continue
		}

		z.ID = f.ID
		ft, err := isoToTime(f.Mtime)
		if err != nil {
			return err
		}

		// If the file has been modified since last recorded, make the
		// database update operation.
		if modTime.After(ft) {
			fp := filepath.Join(dirPath, z.Name)
			contentBytes, err := os.ReadFile(fp)
			if err != nil {
				return err
			}
			content := string(contentBytes)
			splitZettel(tx, &z, content)

			if err := updateFile(tx, z); err != nil {
				return fmt.Errorf("Failed to update file record: %v", err)
			}
		}

		// Mark this file in the zettel as visited.
		delete(zm[dirName], z.Name)
	}

	if err := deleteFiles(tx, existingFiles); err != nil {
		return fmt.Errorf("Failed to delete files: %v", err)
	}

	return nil
}

// SplitZettel breaks a zettel's contents and assigns it's parts to
// associated fields: title, body, links, and tags.
func splitZettel(tx *sqlx.Tx, z *Zettel, content string) {
	var bodyLines []string
	isBody := false
	// Match lines that contain a link. E.g., `* [dir][../dir] title`
	linkRegex := regexp.MustCompile(`\[(.+)\]\(\.\./(.*?)/?\) (.+)`)
	tagRegex := regexp.MustCompile(`^ {4,}#[a-zA-Z]+`)

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

	// Insert links
	for _, l := range z.Links {
		_, err = tx.Exec(insertLinksSQL, l.Content, id, l.ToZettelID)
		if err != nil {
			return fmt.Errorf("Error inserting links: %v", err)
		}
	}

	// Insert tags
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
	if err := tx.Get(&id, idQuery, z.Name, z.DirName); err != nil {
		return fmt.Errorf("Failed to get zettel id: %v", err)
	}

	// Update zettel table record
	_, err := tx.Exec(zettelQuery, z.Title, z.Body, z.Mtime, id)
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
		if _, err := tx.Exec(query, l.ID, fromZettelID); err != nil {
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
			VALUES ($1, $2) ON CONFLICT DO NOTHING`
	)

	for _, tag := range tags {
		var tagID int

		// First, try to find the ID of the tag if it already exists
		err := tx.QueryRow(selectTagIDSQL, tag.Name).Scan(&tagID)
		if err != nil {
			if err != sql.ErrNoRows {
				return fmt.Errorf("Failed to get tag id: %v", err)
			}
			// The tag doesn't exist, insert it and retrieve id.
			err = tx.QueryRow(insertTagSQL, tag.Name).Scan(&tagID)
			if err != nil {
				return fmt.Errorf("Error inserting tag: %v", err)
			}
		}

		// Insert the zettel-tag association into the zettel_tags table
		_, err = tx.Exec(insertZettelTagSQL, zettelID, tagID)
		if err != nil {
			return fmt.Errorf("Error inserting zettel-tag link: %v", err)
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
		if _, err := stmt.Exec(z.ID); err != nil {
			// Log the error but continue deleting other files
			log.Printf("Error deleting file with id %d: %v", z.ID, err)
		}
	}

	if err := cleanTags(tx); err != nil {
		return fmt.Errorf("Error cleaning tags: %v", err)
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
		if _, err := stmt.Exec(dirName); err != nil {
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

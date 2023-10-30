// Package storage provides the functionality for interacting with the
// zet database.
package storage

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type dir struct {
	Id   int    `db:"id"`   // unique id
	Name string `db:"name"` // unique directory name
}

type file struct {
	Id      int    `db:"id"`       // unique id
	Name    string `db:"name"`     // name of file
	Content string `db:"content"`  // contents of file
	Mtime   string `db:"mtime"`    // modification time
	DirName string `db:"dir_name"` // modification time
}

// UpdateDB initializes the database, retrieve zet state from the
// database, and updates the database to sync the flat files and the
// data storage.
func UpdateDB() error {
	db, err := Init()
	if err != nil {
		return fmt.Errorf("Failed to initialize database: %v.\n", err)
	}
	ez, err := getExistingZettels(db)
	if err != nil {
		return fmt.Errorf("Failed to get zettels: %v.\n", err)
	}
	if err := processZettels(db, ez); err != nil {
		return fmt.Errorf("Failed to process zettels: %v.\n", err)
	}
	return nil
}

// Init creates the database if it doesn't exist and returns the
// database connection.
func Init() (*sqlx.DB, error) {
	// Resolve zet database path.
	dbPath := os.Getenv("ZET_DB_PATH")
	if dbPath == "" {
		return nil, errors.New("Environment variable ZET_DB_PATH must be set")
	}

	// Connect to SQLite database
	db, err := sqlx.Connect("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	const query = `
      CREATE TABLE IF NOT EXISTS dirs (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT UNIQUE NOT NULL,  -- Unique identifier for the zettel
      );

      CREATE TABLE IF NOT EXISTS files (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,            -- Name of the file
        content TEXT NOT NULL,         -- File content
        mtime TEXT NOT NULL,           -- Last modification time
        dir_name TEXT UNIQUE NOT NULL, -- Name of the directory this file belongs to
				FOREIGN KEY(dir_name) REFERENCES Directories(name) -- Reference to parent directory
      );`

	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// GetExistingZettels retrieves all existing zettels from the database
// and put them into a map. It returns a map that includes each zettel
// directory and all non-directory files. The value is a file struct.
func getExistingZettels(db *sqlx.DB) (map[string]map[string]file, error) {
	// Map to hold existing zettels and their files from the database
	var existingZettels = make(map[string]map[string]file)
	files := []file{}

	// SQL query to retrieve zettels
	const query = `SELECT * FROM files;`

	err := db.Select(&files, query)
	if err != nil {
		return existingZettels, err
	}

	for _, f := range files {
		if _, exists := existingZettels[f.DirName]; !exists {
			existingZettels[f.DirName] = make(map[string]file)
		}

		existingZettels[f.DirName][f.Name] = f
	}

	return existingZettels, nil
}

// processZettels iterates over each zettel directory and its files to
// keep the flat files and database in sync.
func processZettels(db *sqlx.DB, existingZettels map[string]map[string]file) error {
	dir := "./zet"
	dirs, err := os.ReadDir(dir)
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
		dirPath := filepath.Join("./zet", dirName)

		// Check if this is a new or existing zettel.
		_, exists := existingZettels[dirName]

		// For *new* zettels: Insert the dir and all its files (that
		// aren't a dir) into the database.
		if !exists {
			if err := addZettel(db, dirPath); err != nil {
				log.Printf("Failed to insert a zettel: %v.\n", err)
			}
			continue // Move to the next directory
		}

		// For *existing* zettel, process its files.
		err = processFiles(db, dirPath, existingZettels)
		if err != nil {
			return err
		}

		// Mark this zettel directory as visited.
		delete(existingZettels, dirName)
	}

	// Perform deletion on remaining dirs
	if err := deleteDirs(db, existingZettels); err != nil {
		log.Printf("Failed to delete a zettel: %v.\n", err)
	}

	return nil
}

// addZettel inserts a zettel directory into the database. This is
// performed by inserting the zettel directory into the dirs table and
// all of its files into the files table.
func addZettel(db *sqlx.DB, dirPath string) error {
	dirName := path.Base(dirPath)
	if err := insertDir(db, dirName); err != nil {
		return err
	}

	// Fetch files inside this directory
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("Error reading sub-directory: %v", err)
	}
	// For each file that is NOT a directory:
	// If new file Add new files or update existing files in the database.
	for _, file := range files {
		if file.IsDir() { // Skip sub-directories
			continue
		}

		fileName := file.Name()
		info, err := file.Info()
		if err != nil {
			return fmt.Errorf("Error reading file info: %v", err)
		}
		modTime := info.ModTime()

		fp := filepath.Join(dirPath, fileName)
		contentBytes, err := os.ReadFile(fp)
		if err != nil {
			return err
		}

		content := string(contentBytes)

		// Note: Filter out any files that are not markdown.
		if strings.HasSuffix(fileName, ".md") {
			err := insertFile(db, dirName, fileName, content, modTime)
			if err != nil {
				return fmt.Errorf("Failed to insert new file: %v", err)
			}
			continue
		}
	}

	return nil
}

// InsertDir inserts a directory into the dirs database table.
func insertDir(db *sqlx.DB, name string) error {
	const query = `
    INSERT INTO dirs (name)
    VALUES ($1);
    `
	_, err := db.Exec(query, name)
	return err
}

// processFiles iterates over a zettel directory and inserts new files,
// updates modified files, and remove deleted files.
func processFiles(db *sqlx.DB, dirPath string, existingZettels map[string]map[string]file) error {
	dirName := path.Base(dirPath)

	// Retrieve the map for all the files for a given zettel directory
	// and validate that the directory exists.
	existingFiles, exists := existingZettels[dirName]
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
		if file.IsDir() { // Skip sub-directories
			continue
		}

		fileName := file.Name()
		info, err := file.Info()
		if err != nil {
			return fmt.Errorf("Error reading file info: %v", err)
		}
		modTime := info.ModTime()

		//Check if this is a new or existing file for this particular
		//zettel.
		f, exists := existingFiles[fileName]
		ft, err := ISOtoTime(f.Mtime)
		if err != nil {
			return err
		}

		fp := filepath.Join(dirPath, fileName)
		contentBytes, err := os.ReadFile(fp)
		if err != nil {
			return err
		}

		content := string(contentBytes)

		// If the file doesn't exist in this zettel, insert it into the
		// database.
		// Note: Filter out any files that are not markdown.
		if !exists && strings.HasSuffix(fileName, ".md") {
			err := insertFile(db, dirName, fileName, content, modTime)
			if err != nil {
				return fmt.Errorf("Failed to insert new file: %v", err)
			}
			continue
		}

		// If the file has been modified since last recorded, make the
		// database update operation.
		if modTime.After(ft) {
			err := updateFile(db, dirName, fileName, content, modTime)
			if err != nil {
				return fmt.Errorf("Failed to update file record: %v", err)
			}
		}

		// Mark this file in the zettel as visited.
		delete(existingZettels[dirName], fileName)
	}

	err = deleteFiles(db, existingFiles)
	if err != nil {
		return fmt.Errorf("Failed to delete files: %v", err)
	}

	return nil
}

// InsertFile inserts a new file into the database.
func insertFile(db *sqlx.DB, dirName string, fileName string, content string, mtime time.Time) error {
	mt := mtime.Format(time.RFC3339)
	const query = `
    INSERT INTO files (name, content, mtime, dir_name)
    VALUES ($1, $2, $3, $4);
    `

	// Execute the SQL query
	// This will insert a new row into the 'files' table with the provided values
	_, err := db.Exec(query, fileName, content, mt, dirName)
	return err
}

func updateFile(db *sqlx.DB, dirName string, fileName string, content string, mtime time.Time) error {
	mt := mtime.Format(time.RFC3339)
	const query = `
    UPDATE files SET
		name = $1, content = $2, mtime = $3, dir_name =	$4
		WHERE id = $5
    `
	// Execute the SQL query
	// This will insert a new row into the 'files' table with the provided values
	_, err := db.Exec(query, fileName, content, mt, dirName)
	return err
}

// DeleteFiles deletes any remaining files in an existing files map
// from the database. This removes files from a single zettel directory.
func deleteFiles(db *sqlx.DB, existingFiles map[string]file) error {
	const query = `
		DELETE FROM files WHERE id = $1;
	`
	stmt, err := db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Iterate through each remaining file in the directory
	for _, f := range existingFiles {
		_, err := stmt.Exec(f.Id)
		if err != nil {
			// Log the error but continue deleting other files
			log.Printf("Error deleting file with id %d: %v", f.Id, err)
		}
	}
	return nil
}

// DeleteDirs deletes any remaining files in an existing zettels map
// from the database. This removes directories (zettels) from the zet
// directory.
func deleteDirs(db *sqlx.DB, existingZettels map[string]map[string]file) error {
	const query = `
		DELETE FROM dirs WHERE name = $1;
	`
	stmt, err := db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Iterate through each remaining directory
	for dirName := range existingZettels {
		_, err := stmt.Exec(dirName)
		if err != nil {
			// Log the error but continue deleting other directories
			log.Printf("Error deleting file with name %s: %v", dirName, err)
		}
	}
	return nil
}

// ISOtoTime converts a given ISO8601 string back to time.Time object
func ISOtoTime(t string) (time.Time, error) {
	mt, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return time.Time{}, err
	}
	return mt, nil
}
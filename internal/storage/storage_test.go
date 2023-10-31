package storage

import (
	"fmt"
	"path/filepath"

	"github.com/jmoiron/sqlx"
)

func getDBConnection() (*sqlx.DB, error) {
	// Connect to the test database
	// Use an in-memory SQLite database for tests
	db, err := sqlx.Connect("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}

	// Setup the test database
	err = setupTestDB(db)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func getTestTransaction() (*sqlx.Tx, error) {
	// Connect to the test database
	// Use an in-memory SQLite database for tests
	db, err := sqlx.Connect("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	// Setup the test database
	err = setupTestDB(db)
	if err != nil {
		return nil, err
	}
	// Start a new transaction
	tx, err := db.Beginx()
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func setupTestDB(db *sqlx.DB) error {
	// Create your tables here
	// For example
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS dirs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE
        );
        CREATE TABLE IF NOT EXISTS files (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            content TEXT NOT NULL,
            mtime TEXT NOT NULL,
            dir_name TEXT NOT NULL,
            FOREIGN KEY(dir_name) REFERENCES directories(name)
        );
    `)

	return err
}

func insertDummyData(db *sqlx.DB) error {
	// Insert dummy data
	_, err := db.Exec(`
        -- Create dummy data for directories table
        INSERT INTO dirs(name) VALUES('20231028012959');
        INSERT INTO dirs(name) VALUES('20231028013010');
        INSERT INTO dirs(name) VALUES('20231028013031');
        INSERT INTO dirs(name) VALUES('20231028013045');
        INSERT INTO dirs(name) VALUES('20231028013100');

        -- Create dummy data for Files table
        -- For directory 20231028012959
        INSERT INTO Files(name, content, mtime, dir_name) VALUES('README.md', '# README', '2023-  10-28T01:29:59Z', '20231028012959');

        -- For directory 20231028013010
        INSERT INTO Files(name, content, mtime, dir_name) VALUES('README.md', '# README', '2023-  10-28T01:30:10Z', '20231028013010');

        -- For directory 20231028013031
        INSERT INTO Files(name, content, mtime, dir_name) VALUES('README.md', '# README', '2023-  10-28T01:30:31Z', '20231028013031');
        INSERT INTO Files(name, content, mtime, dir_name) VALUES('outline.md', '# Outline', '2023-10-28T01:30:31Z', '20231028013031');

        -- For directory 20231028013045
        INSERT INTO Files(name, content, mtime, dir_name) VALUES('README.md', '# README', '2023-  10-28T01:30:45Z', '20231028013045');
        INSERT INTO Files(name, content, mtime, dir_name) VALUES('outline.md', '# Outline', '2023-10-28T01:30:45Z', '20231028013045');

    `)
	if err != nil {
		return err
	}

	return nil
}

func ExampleGetExistingZettels() {
	db, err := getDBConnection()
	if err != nil {
		fmt.Printf("Failed to establish database connection: %v.\n", err)
		return
	}
	defer db.Close()

	if err := insertDummyData(db); err != nil {
		fmt.Printf("Failed to insert dummy data: %v\n", err)
		return
	}

	ez, err := getExistingZettels(db)
	if err != nil {
		fmt.Printf("Failed to fetch zettels: %v\n", err)
		return
	}

	// Iterate through each directory
	for _, filesMap := range ez {
		// Iterate through each file in the directory
		for _, f := range filesMap {
			fmt.Printf("./%s/%s id: %d\n", f.DirName, f.Name, f.Id)
		}
	}

	// Commenting out example output since dealing with maps will produce
	// inconsistent output.

	/// Output:
	// 	./20231028012959/README.md id: 1
	// ./20231028013010/README.md id: 2
	// ./20231028013031/README.md id: 3
	// ./20231028013031/outline.md id: 4
	// ./20231028013045/README.md id: 5
	// ./20231028013045/outline.md id: 6
}

func ExampleProcessFile() {
	tx, err := getTestTransaction()
	if err != nil {
		fmt.Printf("Failed to establish database connection: %v.\n", err)
		return
	}
	defer tx.Rollback()

	testZetDir := filepath.Join("..", "testdata", "zet")

	if err := processZettels(tx, testZetDir, make(map[string]map[string]file)); err != nil {
		fmt.Printf("Failed to process zettels: %v\n", err)
		return
	}

	// Validate zettel insertions.
	files := []file{}
	err = tx.Select(&files, "SELECT * FROM files;")
	if err != nil {
		fmt.Printf("Failed to select database zettels: %v\n", err)
		return
	}

	for _, f := range files {
		fmt.Printf("./%s/%s id: %d\n", f.DirName, f.Name, f.Id)
	}

	// Output:
	// then wot
}

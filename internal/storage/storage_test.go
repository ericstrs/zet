package storage

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

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

func ExampleProcessZettels_EmptyDB() {
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
	// ./20231028012959/README.md id: 1
	// ./20231028013010/README.md id: 2
	// ./20231028013031/README.md id: 3
	// ./20231028013031/outline.md id: 4
}

func ExampleProcessZettels_Update() {
	tx, err := getTestTransaction()
	if err != nil {
		fmt.Printf("Failed to establish database connection: %v.\n", err)
		return
	}
	defer tx.Rollback()

	testZetDir := filepath.Join("..", "testdata", "zet")
	existingZettels := make(map[string]map[string]file)

	// Dummy time for modification time
	dummyTime := time.Now().Add(-86 * time.Hour).Format(time.RFC3339)

	dirs := []string{"20231028012959", "20231028013010", "20231028013031"}
	for _, d := range dirs {
		existingZettels[d] = make(map[string]file)
	}

	// First Zettel directory: '20231028012959' with file 'README.md'
	existingZettels["20231028012959"]["README.md"] = file{
		Id:      1,
		Name:    "README.md",
		Content: "Content for README.md in 20231028012959",
		Mtime:   dummyTime,
		DirName: "20231028012959",
	}

	// Second Zettel directory: '20231028013010' with file 'README.md'
	existingZettels["20231028013010"]["README.md"] = file{
		Id:      2,
		Name:    "README.md",
		Content: "Content for README.md in 20231028013010",
		Mtime:   dummyTime,
		DirName: "20231028013010",
	}

	// Third Zettel directory: '20231028013031' with files 'README.md' and 'outline.md'
	existingZettels["20231028013031"]["README.md"] = file{
		Id:      3,
		Name:    "README.md",
		Content: "Content for README.md in 20231028013031",
		Mtime:   dummyTime,
		DirName: "20231028013031",
	}
	existingZettels["20231028013031"]["outline.md"] = file{
		Id:      4,
		Name:    "outline.md",
		Content: "Content for outline.md in 20231028013031",
		Mtime:   dummyTime,
		DirName: "20231028013031",
	}

	const dirsQuery = `
    INSERT INTO dirs (name)
    VALUES ($1);
    `

	for _, d := range dirs {
		_, err := tx.Exec(dirsQuery, d)
		if err != nil {
			fmt.Printf("Failed to insert existing dirs: %v\n", err)
			return
		}
	}

	const query = `
    INSERT INTO files (name, content, mtime, dir_name)
    VALUES ($1, $2, $3, $4);
    `

	// Create an array to hold all files before inserting them into the database
	var allFiles []file

	// Collect all files from existingZettels into the allFiles slice
	for _, filesMap := range existingZettels {
		for _, f := range filesMap {
			allFiles = append(allFiles, f)
		}
	}

	// Sort the allFiles slice by Id
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].Id < allFiles[j].Id
	})

	for _, f := range allFiles {
		// This will insert a new row into the 'files' table with the provided values
		_, err := tx.Exec(query, f.Name, f.Content, f.Mtime, f.DirName)
		if err != nil {
			fmt.Printf("Failed to insert existing files: %v\n", err)
			return
		}
	}

	if err := processZettels(tx, testZetDir, existingZettels); err != nil {
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
	// ./20231028012959/README.md id: 1
	// ./20231028013010/README.md id: 2
	// ./20231028013031/README.md id: 3
	// ./20231028013031/outline.md id: 4
}

func ExampleProcessZettels_Delete() {
	tx, err := getTestTransaction()
	if err != nil {
		fmt.Printf("Failed to establish database connection: %v.\n", err)
		return
	}
	defer tx.Rollback()

	testZetDir := filepath.Join("..", "testdata", "zet")
	existingZettels := make(map[string]map[string]file)

	// Dummy time for modification time
	dummyTime := time.Now().Add(-86 * time.Hour).Format(time.RFC3339)

	dirs := []string{"20231028012959", "20231028013010", "20231028013031", "20231031214058"}
	for _, d := range dirs {
		existingZettels[d] = make(map[string]file)
	}

	// First Zettel directory: '20231028012959' with file 'README.md'
	existingZettels["20231028012959"]["README.md"] = file{
		Id:      1,
		Name:    "README.md",
		Content: "Content for README.md in 20231028012959",
		Mtime:   dummyTime,
		DirName: "20231028012959",
	}

	// Second Zettel directory: '20231028013010' with file 'README.md'
	existingZettels["20231028013010"]["README.md"] = file{
		Id:      2,
		Name:    "README.md",
		Content: "Content for README.md in 20231028013010",
		Mtime:   dummyTime,
		DirName: "20231028013010",
	}

	// Third Zettel directory: '20231028013031' with files 'README.md',
	// 'outline.md', and 'foo.md'.
	existingZettels["20231028013031"]["README.md"] = file{
		Id:      3,
		Name:    "README.md",
		Content: "Content for README.md in 20231028013031",
		Mtime:   dummyTime,
		DirName: "20231028013031",
	}
	existingZettels["20231028013031"]["outline.md"] = file{
		Id:      4,
		Name:    "outline.md",
		Content: "Content for outline.md in 20231028013031",
		Mtime:   dummyTime,
		DirName: "20231028013031",
	}
	existingZettels["20231028013031"]["foo.md"] = file{
		Id:      5,
		Name:    "foo.md",
		Content: "Content for foo.md in 20231028013031",
		Mtime:   dummyTime,
		DirName: "20231028013031",
	}

	// Fourth Zettel directory: '20231031214058' with file 'README.md'
	existingZettels["20231031214058"]["README.md"] = file{
		Id:      6,
		Name:    "README.md",
		Content: "Content for README.md in 20231031214058",
		Mtime:   dummyTime,
		DirName: "20231031214058",
	}

	const dirsQuery = `
    INSERT INTO dirs (name)
    VALUES ($1);
    `

	for _, d := range dirs {
		_, err := tx.Exec(dirsQuery, d)
		if err != nil {
			fmt.Printf("Failed to insert existing dirs: %v\n", err)
			return
		}
	}

	const query = `
    INSERT INTO files (name, content, mtime, dir_name)
    VALUES ($1, $2, $3, $4);
    `

	// Create an array to hold all files before inserting them into the database
	var allFiles []file

	// Collect all files from existingZettels into the allFiles slice
	for _, filesMap := range existingZettels {
		for _, f := range filesMap {
			allFiles = append(allFiles, f)
		}
	}

	// Sort the allFiles slice by Id
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].Id < allFiles[j].Id
	})

	for _, f := range allFiles {
		// This will insert a new row into the 'files' table with the provided values
		_, err := tx.Exec(query, f.Name, f.Content, f.Mtime, f.DirName)
		if err != nil {
			fmt.Printf("Failed to insert existing files: %v\n", err)
			return
		}
	}

	if err := processZettels(tx, testZetDir, existingZettels); err != nil {
		fmt.Printf("Failed to process zettels: %v\n", err)
		return
	}

	// Validate zettel insertions.
	files := []file{}
	err = tx.Select(&files, "SELECT * FROM files;")
	if err != nil {
		fmt.Printf("Failed to select database files: %v\n", err)
		return
	}

	directories := []string{}
	err = tx.Select(&directories, "SELECT name FROM dirs;")
	if err != nil {
		fmt.Printf("Failed to select database dirs: %v\n", err)
		return
	}

	fmt.Println("Dirs")
	for _, d := range directories {
		fmt.Println(d)
	}

	fmt.Println("Files")
	for _, f := range files {
		fmt.Printf("./%s/%s id: %d\n", f.DirName, f.Name, f.Id)
	}

	// Output:
	// Dirs
	// 20231028012959
	// 20231028013010
	// 20231028013031
	// Files
	// ./20231028012959/README.md id: 1
	// ./20231028013010/README.md id: 2
	// ./20231028013031/README.md id: 3
	// ./20231028013031/outline.md id: 4
}

func ExampleAddZettel() {
	tx, err := getTestTransaction()
	if err != nil {
		fmt.Printf("Failed to establish database connection: %v.\n", err)
		return
	}
	defer tx.Rollback()

	testZetDir := filepath.Join("..", "testdata", "zet", "20231028013031")

	if err := addZettel(tx, testZetDir); err != nil {
		fmt.Printf("Failed to add zettel: %v\n", err)
		return
	}

	// Validate zettel insertion.
	files := []file{}
	err = tx.Select(&files, "SELECT * FROM files WHERE dir_name = 20231028013031;")
	if err != nil {
		fmt.Printf("Failed to select zettel: %v\n", err)
		return
	}

	for _, f := range files {
		fmt.Printf("./%s/%s id: %d\n", f.DirName, f.Name, f.Id)
	}

	// Output:
	// ./20231028013031/README.md id: 1
	// ./20231028013031/outline.md id: 2
}

func ExampleProcessFiles_EmptyDB() {
	tx, err := getTestTransaction()
	if err != nil {
		fmt.Printf("Failed to establish database connection: %v.\n", err)
		return
	}
	defer tx.Rollback()

	testZetDir := filepath.Join("..", "testdata", "zet", "20231028013031")

	if err := processFiles(tx, testZetDir, make(map[string]map[string]file)); err != nil {
		fmt.Printf("Failed to process zettel files: %v\n", err)
		return
	}

	// Validate zettel insertion.
	files := []file{}
	err = tx.Select(&files, "SELECT * FROM files WHERE dir_name = 20231028013031;")
	if err != nil {
		fmt.Printf("Failed to select zettel: %v\n", err)
		return
	}

	for _, f := range files {
		fmt.Printf("./%s/%s id: %d\n", f.DirName, f.Name, f.Id)
	}

	// Output:
	// Failed to process zettel files: Failed to process files in ../testdata/zet/20231028013031: Directory doesn't exist.
}

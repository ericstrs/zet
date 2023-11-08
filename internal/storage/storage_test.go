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
        FOREIGN KEY(dir_name) REFERENCES Directories(name) -- Reference to parent directory
      );

      -- Table for storing zettel links
      CREATE TABLE IF NOT EXISTS link (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        content TEXT NOT NULL,
        zettel_id INTEGER NOT NULL,
        FOREIGN KEY(zettel_id) REFERENCES zettel(id) ON DELETE CASCADE
      );

      -- Table for storing zettel tags
      CREATE TABLE IF NOT EXISTS tag (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL
      );

      -- Many-to-many relationship table between zettels and tags
      CREATE TABLE IF NOT EXISTS zettel_tags (
        zettel_id INTEGER NOT NULL,            -- ID of the zettel
        tag_id INTEGER NOT NULL,               -- ID of the tag
        PRIMARY KEY(zettel_id, tag_id),        -- Composite primary key
        FOREIGN KEY(zettel_id) REFERENCES zettels(id) ON DELETE CASCADE,
        FOREIGN KEY(tag_id) REFERENCES tags(id) ON DELETE CASCADE
      );`
	_, err := db.Exec(query)
	return err
}

func insertDummyData(db *sqlx.DB) error {
	// Insert dummy data
	_, err := db.Exec(`
        -- Create dummy data for directories table
        INSERT INTO dirs(name) VALUES('20231028012959');
        INSERT INTO dirs(name) VALUES('20231028013010');
        INSERT INTO dirs(name) VALUES('20231028013031');
        INSERT INTO dirs(name) VALUES('20231028013100');

				-- Insert zettels
				INSERT INTO zettel (name, title, body, mtime, dir_name) VALUES ('README.md', '# Zettel 1', 'This is the zettel body', '2023-10-28T01:29:59Z', '20231028012959');
				INSERT INTO zettel (name, title, body, mtime, dir_name) VALUES ('README.md', '# Zettel 2', 'This is the zettel body',  '2023-10-28T01:30:10Z', '20231028013010');
				INSERT INTO zettel (name, title, body, mtime, dir_name) VALUES ('README.md', '# Zettel 3', '',  '2023-10-28T01:30:31Z', '20231028013031');
				INSERT INTO zettel (name, title, body, mtime, dir_name) VALUES ('outline.md', '# Outline', '',  '2023-10-28T01:30:31Z', '20231028013031');

				-- Insert tags
				-- Note: 'productivity' and 'pkms' should be unique so they are not inserted multiple times.
				INSERT INTO tag (name) VALUES ('productivity');
				INSERT INTO tag (name) VALUES ('pkms');

				-- We assume that the IDs for the zettels and tags have been autoincremented in the order of insertion.
				-- Therefore, the 'zettel' table will have IDs 1 to 4, and the 'tag' table will have IDs 1 and 2 for 'productivity' and 'pkms', respectively.

				-- Insert zettel_tags associations
				INSERT INTO zettel_tags (zettel_id, tag_id) VALUES (2, 1); -- Zettel 2 with 'productivity' tag
				INSERT INTO zettel_tags (zettel_id, tag_id) VALUES (2, 2); -- Zettel 2 with 'pkms' tag
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

	s := Storage{db: db}

	ez, err := s.zettelsMap()
	if err != nil {
		fmt.Printf("Failed to fetch zettels: %v\n", err)
		return
	}

	// Iterate through each directory
	for _, filesMap := range ez {
		// Iterate through each file in the directory
		for _, f := range filesMap {
			fmt.Printf("./%s/%s id: %d\n", f.DirName, f.Name, f.ID)
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

	if err := processZettels(tx, testZetDir, make(map[string]map[string]Zettel)); err != nil {
		fmt.Printf("Failed to process zettels: %v\n", err)
		return
	}

	// Validate zettel insertions.
	files := []Zettel{}
	err = tx.Select(&files, "SELECT * FROM zettel;")
	if err != nil {
		fmt.Printf("Failed to select database zettels: %v\n", err)
		return
	}

	for _, f := range files {
		fmt.Printf("./%s/%s id: %d\n", f.DirName, f.Name, f.ID)
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
	existingZettels := make(map[string]map[string]Zettel)

	// Dummy time for modification time
	dummyTime := time.Now().Add(-200 * time.Hour).Format(time.RFC3339)

	dirs := []string{"20231028012959", "20231028013010", "20231028013031"}
	for _, d := range dirs {
		existingZettels[d] = make(map[string]Zettel)
	}

	// First Zettel directory: '20231028012959' with file 'README.md'
	existingZettels["20231028012959"]["README.md"] = Zettel{
		ID:    1,
		Name:  "README.md",
		Title: `# Zettel 1`,
		Body: `

		This is the zettel body`,
		Links:   []Link{},
		Tags:    []Tag{},
		Mtime:   dummyTime,
		DirName: "20231028012959",
	}

	// Second Zettel directory: '20231028013010' with file 'README.md'
	existingZettels["20231028013010"]["README.md"] = Zettel{
		ID:    2,
		Name:  "README.md",
		Title: `# Zettel 2`,
		Body: `

		This is the zettel body`,
		Links:   []Link{},
		Tags:    []Tag{Tag{Name: `productivity`}, Tag{Name: `pkms`}},
		Mtime:   dummyTime,
		DirName: "20231028013010",
	}

	// Third Zettel directory: '20231028013031' with files 'README.md' and 'outline.md'
	existingZettels["20231028013031"]["README.md"] = Zettel{
		ID:      3,
		Name:    "README.md",
		Title:   `# Zettel 3`,
		Body:    ``,
		Links:   []Link{},
		Tags:    []Tag{},
		Mtime:   dummyTime,
		DirName: "20231028013031",
	}
	existingZettels["20231028013031"]["outline.md"] = Zettel{
		ID:      4,
		Name:    "outline.md",
		Title:   `# Outline`,
		Body:    ``,
		Links:   []Link{},
		Tags:    []Tag{},
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
    INSERT INTO zettel (name, title, body, mtime, dir_name)
    VALUES ($1, $2, $3, $4, $5);
    `

	// Create an array to hold all files before inserting them into the database
	var allFiles []Zettel

	// Collect all files from existingZettels into the allFiles slice
	for _, filesMap := range existingZettels {
		for _, f := range filesMap {
			allFiles = append(allFiles, f)
		}
	}

	// Sort the allFiles slice by ID
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].ID < allFiles[j].ID
	})

	for _, f := range allFiles {
		// This will insert a new row into the 'files' table with the provided values
		_, err := tx.Exec(query, f.Name, f.Title, f.Body, f.Mtime, f.DirName)
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
	files := []Zettel{}
	err = tx.Select(&files, "SELECT * FROM files;")
	if err != nil {
		fmt.Printf("Failed to select database zettels: %v\n", err)
		return
	}

	for _, f := range files {
		fmt.Printf("./%s/%s id: %d\n", f.DirName, f.Name, f.ID)
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
	existingZettels := make(map[string]map[string]Zettel)

	// Dummy time for modification time
	dummyTime := time.Now().Add(-86 * time.Hour).Format(time.RFC3339)

	dirs := []string{"20231028012959", "20231028013010", "20231028013031", "20231031214058"}
	for _, d := range dirs {
		existingZettels[d] = make(map[string]Zettel)
	}

	// First Zettel directory: '20231028012959' with file 'README.md'
	existingZettels["20231028012959"]["README.md"] = Zettel{
		ID:    1,
		Name:  "README.md",
		Title: `# Zettel 1`,
		Body: `

      This is the zettel body`,
		Links:   []Link{},
		Tags:    []Tag{},
		Mtime:   dummyTime,
		DirName: "20231028012959",
	}

	// Second Zettel directory: '20231028013010' with file 'README.md'
	existingZettels["20231028013010"]["README.md"] = Zettel{
		ID:    2,
		Name:  "README.md",
		Title: `# Zettel 2`,
		Body: `

      This is the zettel body`,
		Links:   []Link{},
		Tags:    []Tag{Tag{Name: `productivity`}, Tag{Name: `pkms`}},
		Mtime:   dummyTime,
		DirName: "20231028013010",
	}

	// Third Zettel directory: '20231028013031' with files 'README.md' and 'outline.md'
	existingZettels["20231028013031"]["README.md"] = Zettel{
		ID:      3,
		Name:    "README.md",
		Title:   `# Zettel 3`,
		Body:    ``,
		Links:   []Link{},
		Tags:    []Tag{},
		Mtime:   dummyTime,
		DirName: "20231028013031",
	}
	existingZettels["20231028013031"]["outline.md"] = Zettel{
		ID:      4,
		Name:    "outline.md",
		Title:   `# Outline`,
		Body:    ``,
		Links:   []Link{},
		Tags:    []Tag{},
		Mtime:   dummyTime,
		DirName: "20231028013031",
	}
	existingZettels["20231028013031"]["foo.md"] = Zettel{
		ID:      5,
		Name:    "foo.md",
		Title:   `# Foo`,
		Body:    ``,
		Links:   []Link{},
		Tags:    []Tag{},
		Mtime:   dummyTime,
		DirName: "20231028013031",
	}

	// Fourth Zettel directory: '20231031214058' with file 'README.md'
	existingZettels["20231031214058"]["README.md"] = Zettel{
		ID:      6,
		Name:    "README.md",
		Title:   `# read`,
		Body:    ``,
		Links:   []Link{},
		Tags:    []Tag{},
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
    INSERT INTO files (name, title, body, mtime, dir_name)
    VALUES ($1, $2, $3, $4);
    `

	// Create an array to hold all files before inserting them into the database
	var allFiles []Zettel

	// Collect all files from existingZettels into the allFiles slice
	for _, filesMap := range existingZettels {
		for _, f := range filesMap {
			allFiles = append(allFiles, f)
		}
	}

	// Sort the allFiles slice by ID
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].ID < allFiles[j].ID
	})

	for _, f := range allFiles {
		// This will insert a new row into the 'files' table with the provided values
		_, err := tx.Exec(query, f.Name, f.Title, f.Body, f.Mtime, f.DirName)
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
	zettels := []Zettel{}
	err = tx.Select(&zettels, "SELECT * FROM zettel;")
	if err != nil {
		fmt.Printf("Failed to select database zettels: %v\n", err)
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
	for _, f := range zettels {
		fmt.Printf("./%s/%s id: %d\n", f.DirName, f.Name, f.ID)
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
	files := []Zettel{}
	err = tx.Select(&files, "SELECT * FROM files WHERE dir_name = 20231028013031;")
	if err != nil {
		fmt.Printf("Failed to select zettel: %v\n", err)
		return
	}

	for _, f := range files {
		fmt.Printf("./%s/%s id: %d\n", f.DirName, f.Name, f.ID)
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

	if err := processFiles(tx, testZetDir, make(map[string]map[string]Zettel)); err != nil {
		fmt.Printf("Failed to process zettel files: %v\n", err)
		return
	}

	// Validate zettel insertion.
	files := []Zettel{}
	err = tx.Select(&files, "SELECT * FROM files WHERE dir_name = 20231028013031;")
	if err != nil {
		fmt.Printf("Failed to select zettel: %v\n", err)
		return
	}

	for _, f := range files {
		fmt.Printf("./%s/%s id: %d\n", f.DirName, f.Name, f.ID)
	}

	// Output:
	// Failed to process zettel files: Failed to process files in ../testdata/zet/20231028013031: Directory doesn't exist.
}

func ExampleSplitZettel() {
	const e = `# Example Title

This is some body text.
It can span multiple lines.

See:

* [20231028013031](../20231028013031) Some linked Zettel
* [20231029013031](../20231029013031) Another linked Zettel

		#tag1 badTag #tag2`

	title, body, links, tags := splitZettel(e)
	fmt.Println("Title:", title)
	fmt.Println("Body:")
	fmt.Printf(body)
	fmt.Println("Links:")
	for _, l := range links {
		fmt.Printf("\t%s\n", l)
	}
	fmt.Println("Tags:")
	for _, t := range tags {
		fmt.Printf("\t%s\n", t)
	}

	// Output:
	// Title: Example Title
	// Body:
	//
	// This is some body text.
	// It can span multiple lines.
	//
	// See:
	//
	// Links:
	// 	[20231028013031](../20231028013031) Some linked Zettel
	// 	[20231029013031](../20231029013031) Another linked Zettel
	// Tags:
	// 	tag1
	// 	tag2
}

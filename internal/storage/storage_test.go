package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

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
	_, err := db.Exec(tablesSQL)
	return err
}

func insertDummyData(db *sqlx.DB) error {
	_, err := db.Exec(`
        -- Create dummy data for directories table
        INSERT INTO dir(name) VALUES('20231028012959');
        INSERT INTO dir(name) VALUES('20231028013010');
        INSERT INTO dir(name) VALUES('20231028013031');
        INSERT INTO dir(name) VALUES('20231028013100');

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

func ExampleZettelsMap() {
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
	existingZettels := getTestZettelMap()
	db, err := insertTestZettelMap(existingZettels)
	if err != nil {
		fmt.Printf("Error inserting zettel map: %v", err)
		return
	}
	defer db.Close()
	tx, err := db.Beginx()
	if err != nil {
		fmt.Printf("Error starting tx: %v", err)
		return
	}
	defer tx.Rollback()

	testZetDir := filepath.Join("..", "testdata", "zet")

	if err := processZettels(tx, testZetDir, existingZettels); err != nil {
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

func ExampleProcessZettels_Delete() {
	existingZettels := getTestZettelMap()
	db, err := insertTestZettelMap(existingZettels)
	if err != nil {
		fmt.Printf("Error inserting zettel map: %v", err)
		return
	}
	defer db.Close()
	tx, err := db.Beginx()
	if err != nil {
		return
	}
	defer tx.Rollback()

	testZetDir := filepath.Join("..", "testdata", "zet")

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
	err = tx.Select(&directories, "SELECT name FROM dir;")
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

func getTestZettelMap() map[string]map[string]Zettel {
	existingZettels := make(map[string]map[string]Zettel)

	// Dummy time for modification time
	dummyTime := `2023-11-06T22:36:00Z`

	dirs := []string{"20231028012959", "20231028013010", "20231028013031", "20231031214058"}
	for _, d := range dirs {
		existingZettels[d] = make(map[string]Zettel)
	}

	// First Zettel directory: '20231028012959' with file 'README.md'
	existingZettels["20231028012959"]["README.md"] = Zettel{
		ID:    1,
		Name:  "README.md",
		Title: `Zettel 1`,
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
		Title: `Zettel 2`,
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
		Title:   `Zettel 3`,
		Body:    ``,
		Links:   []Link{},
		Tags:    []Tag{},
		Mtime:   dummyTime,
		DirName: "20231028013031",
	}
	existingZettels["20231028013031"]["outline.md"] = Zettel{
		ID:      4,
		Name:    "outline.md",
		Title:   `Outline`,
		Body:    ``,
		Links:   []Link{},
		Tags:    []Tag{},
		Mtime:   dummyTime,
		DirName: "20231028013031",
	}
	existingZettels["20231028013031"]["foo.md"] = Zettel{
		ID:      5,
		Name:    "foo.md",
		Title:   `Foo`,
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
		Title:   `read`,
		Body:    ``,
		Links:   []Link{},
		Tags:    []Tag{},
		Mtime:   dummyTime,
		DirName: "20231031214058",
	}
	return existingZettels
}

func insertTestZettelMap(zm map[string]map[string]Zettel) (*sqlx.DB, error) {
	const (
		dirsSQL = `
      INSERT INTO dir (name)
      VALUES ($1);`
		zettelSQL = `
      INSERT INTO zettel (name, title, body, mtime, dir_name)
      VALUES ($1, $2, $3, $4, $5);`
		insertLinksSQL = `
    	INSERT INTO link (content, from_zettel_id, to_zettel_id)
    	VALUES ($1, $2, $3);`
		insertTagSQL = `INSERT INTO tag (name)
      VALUES ($1) ON CONFLICT(name)
      DO NOTHING RETURNING id`
		selectTagIDSQL = `SELECT id FROM tag
      WHERE name = $1`
		insertZettelTagSQL = `INSERT INTO zettel_tags (zettel_id, tag_id)
      VALUES ($1, $2)`
	)

	db, err := getDBConnection()
	if err != nil {
		return nil, fmt.Errorf("Failed to establish database connection: %v.\n", err)
	}

	dirs := []string{"20231028012959", "20231028013010", "20231028013031", "20231031214058"}
	for _, d := range dirs {
		_, err := db.Exec(dirsSQL, d)
		if err != nil {
			return nil, fmt.Errorf("Failed to insert existing dirs: %v\n", err)
		}
	}

	// Create an array to hold all files before inserting them into the database
	var zettels []Zettel

	// Collect all files from zettel map into the zettels slice
	for _, filesMap := range zm {
		for _, z := range filesMap {
			zettels = append(zettels, z)
		}
	}

	// Sort the zettels slice by ID
	sort.Slice(zettels, func(i, j int) bool {
		return zettels[i].ID < zettels[j].ID
	})

	for _, z := range zettels {
		// This will insert a new row into the 'files' table with the provided values
		_, err := db.Exec(zettelSQL, z.Name, z.Title, z.Body, z.Mtime, z.DirName)
		if err != nil {
			return nil, fmt.Errorf("Failed to insert existing files: %v\n", err)
		}
		// Insert links
		for _, l := range z.Links {
			if _, err = db.Exec(insertLinksSQL, l.Content, z.ID, l.ToZettelID); err != nil {
				return nil, fmt.Errorf("Error inserting links: %v", err)
			}
		}
		// Insert tags
		for _, tag := range z.Tags {
			var tagID int

			// First, try to find the ID of the tag if it already exists
			err := db.QueryRow(selectTagIDSQL, tag.Name).Scan(&tagID)
			if err != nil {
				if err != sql.ErrNoRows {
					// If the error is not 'no rows in result set' then it's an actual error
					return nil, err
				}
				// If tag doesn't exists, insert it and retrieve id.
				if err = db.QueryRow(insertTagSQL, tag.Name).Scan(&tagID); err != nil {
					return nil, err
				}
			}

			// Insert the zettel-tag association into the zettel_tags table
			_, err = db.Exec(insertZettelTagSQL, z.ID, tagID)
			if err != nil {
				return nil, err
			}
		}
	}
	return db, nil
}

func ExampleAddZettel() {
	testZetDir := filepath.Join("..", "testdata", "zet", "20231028013031")

	tx, err := getTestTransaction()
	if err != nil {
		fmt.Printf("Failed to establish database connection: %v.\n", err)
		return
	}
	defer tx.Rollback()

	if err := addZettel(tx, testZetDir); err != nil {
		fmt.Printf("Failed to add zettel: %v\n", err)
		return
	}

	// Validate zettel insertion.
	files := []Zettel{}
	err = tx.Select(&files, "SELECT * FROM zettel WHERE dir_name = 20231028013031;")
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
	err = tx.Select(&files, "SELECT * FROM zettel WHERE dir_name = 20231028013031;")
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
	existingZettels := getTestZettelMap()
	db, err := insertTestZettelMap(existingZettels)
	if err != nil {
		fmt.Printf("Error inserting zettel map: %v", err)
		return
	}
	defer db.Close()
	tx, err := db.Beginx()
	if err != nil {
		return
	}
	defer tx.Rollback()

	const e = `# Example Title

This is some body text.
It can span multiple lines.

See:

* [20231028013031](../20231028013031) Some linked Zettel
* [20231028013031](../20231028013031) Another linked Zettel
* [20240000003031](../20240000003031) Non-existent Zettel

		#tag1 badTag #tag2`

	z := &Zettel{}
	splitZettel(tx, z, e)
	fmt.Println("Title:", z.Title)
	fmt.Println("Body:")
	fmt.Printf(z.Body)
	fmt.Println("Links:")
	for _, l := range z.Links {
		fmt.Printf("\t%s\n", l.Content)
	}
	fmt.Println("Tags:")
	for _, t := range z.Tags {
		fmt.Printf("\t%s\n", t.Name)
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
	// 	[20231028013031](../20231028013031) Another linked Zettel
	// Tags:
	// 	tag1
	// 	tag2
}

func ExampleSearchZettels() {
	existingZettels := getTestZettelMap()
	db, err := insertTestZettelMap(existingZettels)
	if err != nil {
		fmt.Printf("Error inserting zettel map: %v", err)
		return
	}
	defer db.Close()

	s := Storage{db: db}

	term := `zettel productive`
	zettels, err := s.SearchZettels(term, `[red]`, `[white]`)
	if err != nil {
		fmt.Printf("Error searching zettels: %v", err)
		return
	}

	for _, z := range zettels {
		fmt.Println(z.DirName + " " + z.TitleSnippet)
		if z.BodySnippet != "" {
			fmt.Printf("%q\n", z.BodySnippet)
		}
		if z.TagsSnippet != "" {
			hashedTags := "\t\t#" + strings.ReplaceAll(z.TagsSnippet, " ", " #")
			fmt.Println(hashedTags)
		}
	}

	// Output:
	// 20231028013010 [red]Zettel[white] 2
	// "\n\n        This is the [red]zettel[white] body"
	//		#[red]productivity[white] #pkms
}

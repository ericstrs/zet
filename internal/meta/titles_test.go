package meta

import (
	"fmt"
	"os"
)

func ExampleParseTitle_md() {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		fmt.Printf("unable to create temporary file: %v", err)
		return
	}
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name()) // Clean up the file after we're done

	// Write some data to the file
	_, err = tmpFile.WriteString(`# This is the zettel title

This is the zettel body.
`)
	if err != nil {
		fmt.Printf("unable to write to temporary file: %v\n", err)
		return
	}

	// Rewind the file pointer
	_, err = tmpFile.Seek(0, os.SEEK_SET)
	if err != nil {
		fmt.Printf("unable to seek to beginning of file: %v", err)
		return
	}

	t, err := parseTitle(tmpFile, `# `)
	if err != nil {
		fmt.Printf("Failed to parse zettel title: %v", err)
		return
	}

	fmt.Printf("Title: %q\n", t)

	// Output:
	// Title: "This is the zettel title"
}

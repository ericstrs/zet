package zet

import (
	"fmt"
	"strings"
)

func ExampleMakeZettels() {
	content := `## Subtopic 1

Subtopic description.

## Subtopic 2

Subtopic description.

## Subtopic 3

Subtopic description.
`

	bodyLines := strings.Split(content, "\n")
	zettels := makeZettels(bodyLines)
	for i, z := range zettels {
		fmt.Printf("Zettel %q title: %s\n", i+1, z.Title)
		fmt.Printf("Body: %q", z.Body)
	}

	// Output:
	// 	Zettel '\x01' title: Subtopic 1
	// Body: "\nSubtopic description.\n\n"Zettel '\x02' title: Subtopic 2
	// Body: "\nSubtopic description.\n\n"Zettel '\x03' title: Subtopic 3
	// Body: "\nSubtopic description.\n"
}

package meta

import (
	"fmt"
	"strings"
)

func ExampleParseBody() {
	content := `# Computer science

## Algorithms

Algorithms are step-by-step computational procedures for solving problems or performing tasks.

An algorithm's efficiency is often measured in terms of its time and space complexity.

## Artificial Intelligence

Artificial Intelligence (AI) is a branch of computer science focused on building smart machines capable of performing tasks that typically require human intelligence.

AI is an interdisciplinary science with multiple approaches, but advancements in machine learning and deep learning are creating a paradigm shift in virtually every sector of the tech industry.

See more:

* [20231117232357](../20231117232357) Fake link

    #ignore me`

	body := strings.Join(ParseBody(content), "\n")
	fmt.Printf(body)

	// Output:
	//
	// 	## Algorithms
	//
	// Algorithms are step-by-step computational procedures for solving problems or performing tasks.
	//
	// An algorithm's efficiency is often measured in terms of its time and space complexity.
	//
	// ## Artificial Intelligence
	//
	// Artificial Intelligence (AI) is a branch of computer science focused on building smart machines capable of performing tasks that typically require human intelligence.
	//
	// AI is an interdisciplinary science with multiple approaches, but advancements in machine learning and deep learning are creating a paradigm shift in virtually every sector of the tech industry.
	//
	// See more:
}

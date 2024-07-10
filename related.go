package zet

import (
	"fmt"
	"strconv"

	"github.com/blevesearch/bleve/v2"
	"github.com/ericstrs/zet/internal/storage"
)

func CreateIndex(indexPath string) (bleve.Index, error) {
	mapping := bleve.NewIndexMapping()
	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		return nil, fmt.Errorf("error creating index: %v\n", err)
	}
	return index, nil
}

func IndexZettels(index bleve.Index, zettels []storage.Zettel) error {
	for _, zettel := range zettels {
		err := index.Index(strconv.Itoa(zettel.ID), zettel)
		if err != nil {
			return fmt.Errorf("error indexing zettels: %v\n", err)
		}
	}
	return nil
}

func RelatedZettels(index bleve.Index, content string, n int) ([]string, error) {
	// Create a query that searches across multiple fields
	titleQuery := bleve.NewMatchQuery(content)
	titleQuery.SetField("Title")
	titleQuery.SetBoost(2.0) // Give more weight to title matches

	bodyQuery := bleve.NewMatchQuery(content)
	bodyQuery.SetField("Body")

	// Combine the queries
	query := bleve.NewDisjunctionQuery(titleQuery, bodyQuery)

	// Build the search request
	//searchRequest := bleve.NewSearchRequestOptions(query, n, 0, false)
	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Fields = []string{"DirName"}

	// Perform the search
	searchResults, err := index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("error performing search for related zettels: %v\n", err)
	}

	relatedNotes := []string{}
	for _, hit := range searchResults.Hits {
		dir, ok := hit.Fields["DirName"].(string)
		if !ok {
			// handle the case where dir is not a string
			continue
		}
		relatedNotes = append(relatedNotes, dir)
	}

	return relatedNotes, nil
}

package meta

import (
	"fmt"

	"github.com/ericstrs/zet/internal/storage"
)

// List retrieves a list of zettels. It synchronizes the database and
// gets list of zettels.
func List(zetPath, dbPath, sort string) ([]storage.Zettel, error) {
	s, err := storage.UpdateDB(zetPath, dbPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to sync database: %v", err)
	}
	defer s.Close()
	zettels, err := s.AllZettels(sort)
	if err != nil {
		return nil, fmt.Errorf("Error getting all zettels: %v", err)
	}
	return zettels, nil
}

package main

import (
	"errors"
	"io/fs"
	"time"

	"github.com/function61/gokit/encoding/jsonfile"
)

const (
	localDBName = "barcode-db.json"
)

type productDetails struct {
	Name         string     `json:"name"`
	Link         string     `json:"link"`
	FirstScanned *time.Time `json:"first_scanned"`
	LastScanned  *time.Time `json:"last_scanned"`
}

type LocalDB map[string]productDetails

func loadDB() (*LocalDB, error) {
	db := &LocalDB{}
	if err := jsonfile.ReadDisallowUnknownFields(localDBName, db); err != nil {
		if errors.Is(err, fs.ErrNotExist) { // allowed to not exist - start from empty state then
			return db, nil
		} else { // some other error
			return nil, err
		}
	}
	return db, nil
}

func saveDB(db LocalDB) error {
	return jsonfile.Write(localDBName, db)
}

func localDBresolveProductByBarcode(barcode string, resolveDB *LocalDB) (productDetails, bool) {
	details, found := (*resolveDB)[barcode]
	return details, found
}

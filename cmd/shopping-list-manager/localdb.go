package main

import (
	"errors"
	"io/fs"

	"github.com/function61/gokit/encoding/jsonfile"
)

const (
	localDBName = "barcode-db.json"
)

type LocalDB map[string]string

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

func localDBresolveProductNameByBarcode(barcode string, resolveDB *LocalDB) (string, bool) {
	productName, found := (*resolveDB)[barcode]
	return productName, found
}

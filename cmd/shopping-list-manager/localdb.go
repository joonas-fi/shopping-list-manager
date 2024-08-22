package main

import (
	"github.com/function61/gokit/encoding/jsonfile"
)

const (
	localDBName = "barcode-db.json"
)

type LocalDB map[string]string

func loadDB() (*LocalDB, error) {
	db := &LocalDB{}
	return db, jsonfile.ReadDisallowUnknownFields(localDBName, db)
}

func saveDB(db LocalDB) error {
	return jsonfile.Write(localDBName, db)
}

func localDBresolveProductNameByBarcode(barcode string, resolveDB *LocalDB) (string, bool) {
	productName, found := (*resolveDB)[barcode]
	return productName, found
}

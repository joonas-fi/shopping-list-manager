package main

import (
	"testing"

	"github.com/function61/gokit/testing/assert"
)

func TestLocalDBresolveProductByBarcode(t *testing.T) {
	testDB := &LocalDB{
		"6408180733659": productDetails{Name: "Vaasan Voimallus Kaurasämpylä kaurainen sämpylä 480 g 8 kpl"},
	}

	resolve := func(barcode string) string {
		product, found := localDBresolveProductByBarcode(barcode, testDB)
		if !found {
			return "not found"
		}

		return product.Name
	}

	assert.Equal(t, resolve("123"), "not found")
	assert.Equal(t, resolve("6408180733659"), "Vaasan Voimallus Kaurasämpylä kaurainen sämpylä 480 g 8 kpl")
}

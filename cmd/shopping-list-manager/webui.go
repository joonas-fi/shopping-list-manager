package main

// The web UI is for entering unrecognized barcodes to the database.

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"

	. "github.com/function61/gokit/builtin"
	"github.com/function61/gokit/net/http/httputils"
	"github.com/joonas-fi/shopping-list-manager/pkg/todoist"
	"github.com/samber/lo"
)

//go:embed *.html
var templateFiles embed.FS

const (
	appHomeRoute = "/shopping-list-manager/"
)

func webUI(ctx context.Context, todo *todoist.Client, logger *log.Logger) error {
	templates, err := template.ParseFS(templateFiles, "*.html")
	if err != nil {
		return err
	}

	routes := http.NewServeMux()

	routes.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/shopping-list-manager/", http.StatusFound)
	})

	routes.HandleFunc(appHomeRoute, httputils.WrapWithErrorHandling(func(w http.ResponseWriter, r *http.Request) error {
		beep := r.URL.Query().Get("beep")

		if beep != "" {
			output := func() string {
				if _, err := handleBeep(r.Context(), beep, logger, todo); err != nil {
					return err.Error()
				} else {
					return "ok"
				}
			}()

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(output))
			return nil
		}

		db, err := loadDB()
		if err != nil {
			return err
		}
		type productDetailsWrapped struct {
			productDetails
			Barcode string
		}
		db_ := lo.MapToSlice(*db, func(key string, value productDetails) productDetailsWrapped {
			return productDetailsWrapped{
				productDetails: value,
				Barcode:        key,
			}
		})

		sort.Slice(db_, func(i, j int) bool {
			if db_[i].LastScanned == nil {
				return false
			}
			if db_[j].LastScanned == nil {
				return true
			}

			return !db_[i].LastScanned.Before(*db_[j].LastScanned)
		})

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return templates.ExecuteTemplate(w, "index.html", db_)
	}))

	routes.HandleFunc(appHomeRoute+"item/{barcode}", httputils.WrapWithErrorHandling(func(w http.ResponseWriter, r *http.Request) error {
		barcode := r.PathValue("barcode")
		db, err := loadDB()
		if err != nil {
			return err
		}

		if productName := r.URL.Query().Get("name"); productName != "" {
			if err := recordMissAndStoreToLocalDB(r.Context(), barcode, newProductDetails(productName, ""), todo); err != nil {
				return err
			}

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, err := fmt.Fprintf(w, "added to recognized barcodes: %s", productName)
			return err
		}

		item, found := (*db)[barcode]
		if !found {
			item = newProductDetails(taskNameForUnnamedBarcode(barcode), "")
		}
		type itemWrapped struct {
			productDetails
			Barcode string // since this is found from DB key only (not present in the actual item)
			Found   bool
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return templates.ExecuteTemplate(w, "item.html", itemWrapped{
			productDetails: item,
			Found:          found,
			Barcode:        barcode,
		})
	}))

	srv := &http.Server{
		Addr:              ":" + FirstNonEmpty(os.Getenv("PORT"), "80"),
		Handler:           routes,
		ReadHeaderTimeout: httputils.DefaultReadHeaderTimeout,
	}

	return httputils.CancelableServer(ctx, srv, srv.ListenAndServe)
}

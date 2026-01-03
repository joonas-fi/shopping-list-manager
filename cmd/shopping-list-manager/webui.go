package main

// The web UI is for entering unrecognized barcodes to the database.

import (
	"cmp"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"

	"github.com/function61/gokit/net/http/httputils"
	"github.com/joonas-fi/shopping-list-manager/pkg/todoist"
	"github.com/samber/lo"
)

//go:embed *.html
var templateFiles embed.FS

const (
	appHomeRoute = "/shopping-list-manager/"
)

func webUI(ctx context.Context, todo *todoist.Client, logger *slog.Logger) error {
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
			ViewURL string
		}
		db_ := lo.MapToSlice(*db, func(key string, value productDetails) productDetailsWrapped {
			return productDetailsWrapped{
				productDetails: value,
				Barcode:        key,
				ViewURL:        "item/" + url.PathEscape(key),
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

		if categoryFilter := r.URL.Query().Get("category"); categoryFilter != "" {
			db_ = lo.Filter(db_, func(product productDetailsWrapped, _ int) bool { return product.ProductCategory == categoryFilter })
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return templates.ExecuteTemplate(w, "index.html", db_)
	}))

	routes.HandleFunc("GET "+appHomeRoute+"item/{barcode}", httputils.WrapWithErrorHandling(func(w http.ResponseWriter, r *http.Request) error {
		barcode, err := url.PathUnescape(r.PathValue("barcode"))
		if err != nil {
			return err
		}

		db, err := loadDB()
		if err != nil {
			return err
		}

		item, found := (*db)[barcode]
		if !found {
			item = newProductDetails(taskNameForUnnamedBarcode(barcode), "")
		}
		type itemWrapped struct {
			productDetails
			Barcode           string // since this is found from DB key only (not present in the actual item)
			Found             bool
			ProductCategories []string
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return templates.ExecuteTemplate(w, "item.html", itemWrapped{
			productDetails:    item,
			Found:             found,
			Barcode:           barcode,
			ProductCategories: productCategories,
		})
	}))

	routes.HandleFunc("POST "+appHomeRoute+"item/{barcode}", httputils.WrapWithErrorHandling(func(w http.ResponseWriter, r *http.Request) error {
		barcode, err := url.PathUnescape(r.PathValue("barcode"))
		if err != nil {
			return err
		}

		if err := r.ParseForm(); err != nil {
			return err
		}

		db, err := loadDB()
		if err != nil {
			return err
		}

		item, found := (*db)[barcode]
		if !found {
			item = newProductDetails(taskNameForUnnamedBarcode(barcode), "")
		}

		item.Name = r.FormValue("name")
		item.Link = r.FormValue("link")
		item.ProductType = r.FormValue("product_type")
		item.ProductCategory = r.FormValue("product_category")
		item.Notes = r.FormValue("notes")

		if err := recordMissAndStoreToLocalDB(r.Context(), barcode, item, todo); err != nil {
			return err
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, err = fmt.Fprintf(w, "updated: %s", item.Name)
		return err
	}))

	srv := &http.Server{
		Addr:              ":" + cmp.Or(os.Getenv("PORT"), "80"),
		Handler:           routes,
		ReadHeaderTimeout: httputils.DefaultReadHeaderTimeout,
	}

	return httputils.CancelableServer(ctx, srv, srv.ListenAndServe)
}

package main

// The web UI is for entering unrecognized barcodes to the database.

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	. "github.com/function61/gokit/builtin"
	"github.com/function61/gokit/net/http/httputils"
	"github.com/gorilla/mux"
	"github.com/joonas-fi/shopping-list-manager/pkg/todoist"
)

//go:embed *.html
var templateFiles embed.FS

func webUI(ctx context.Context, todo *todoist.Client, _ *log.Logger) error {
	templates, err := template.ParseFS(templateFiles, "*.html")
	if err != nil {
		return err
	}

	routes := mux.NewRouter()
	routes.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/shopping-list-manager", http.StatusFound)
	})
	routes.PathPrefix("/shopping-list-manager").HandlerFunc(httputils.WrapWithErrorHandling(func(w http.ResponseWriter, r *http.Request) error {
		barcode := r.URL.Query().Get("barcode")
		productName := r.URL.Query().Get("name")

		if barcode != "" && productName != "" {
			if err := recordMissAndStoreToLocalDB(r.Context(), barcode, productName, todo); err != nil {
				return err
			}

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, err := fmt.Fprintf(w, "added to recognized barcodes: %s", productName)
			return err
		}
		misses, err := listMisses(ctx, todo)
		if err != nil {
			return err
		}

		if len(misses) == 0 {
			return errors.New("there are no missing barcodes")
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return templates.ExecuteTemplate(w, "index.html", struct{ Barcode string }{misses[0]})
	}))

	srv := &http.Server{
		Addr:              ":" + FirstNonEmpty(os.Getenv("PORT"), "80"),
		Handler:           routes,
		ReadHeaderTimeout: httputils.DefaultReadHeaderTimeout,
	}

	return httputils.CancelableServer(ctx, srv, srv.ListenAndServe)
}
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

	. "github.com/function61/gokit/builtin"
	"github.com/function61/gokit/net/http/httputils"
	"github.com/joonas-fi/shopping-list-manager/pkg/todoist"
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
		barcode := r.URL.Query().Get("barcode")
		productName := r.URL.Query().Get("name")

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

		if barcode != "" && productName != "" {
			if err := recordMissAndStoreToLocalDB(r.Context(), barcode, newProductDetails(productName, ""), todo); err != nil {
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

		missed := func() string {
			if len(misses) > 0 {
				return misses[0]
			} else {
				return ""
			}
		}()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return templates.ExecuteTemplate(w, "index.html", struct{ Barcode string }{missed})
	}))

	srv := &http.Server{
		Addr:              ":" + FirstNonEmpty(os.Getenv("PORT"), "80"),
		Handler:           routes,
		ReadHeaderTimeout: httputils.DefaultReadHeaderTimeout,
	}

	return httputils.CancelableServer(ctx, srv, srv.ListenAndServe)
}

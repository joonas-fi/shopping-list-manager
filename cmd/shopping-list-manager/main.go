package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/function61/gokit/app/cli"
	"github.com/function61/gokit/app/evdev"
	. "github.com/function61/gokit/builtin"
	"github.com/function61/gokit/os/osutil"
	"github.com/function61/gokit/sync/taskrunner"
	"github.com/joonas-fi/home-audio/pkg/homeaudioclient"
	"github.com/joonas-fi/shopping-list-manager/pkg/googlesearch"
	"github.com/joonas-fi/shopping-list-manager/pkg/todoist"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

func main() {
	app := &cobra.Command{
		Short: "Shopping list manager",
	}

	app.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Listen for barcode scans from a barcode reader and add their product names to shopping list",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			todo, err := getClient()
			if err != nil {
				return err
			}

			beep := make(chan string, 2)

			tasks := taskrunner.New(ctx, slog.Default())

			barcodeReaderDevicePath := cmp.Or(os.Getenv("BARCODE_READER"), "/dev/barcode-reader")

			if barcodeReaderDevicePath != "" && barcodeReaderDevicePath != "/dev/null" {
				barcodeReader, close_, err := evdev.Open(barcodeReaderDevicePath)
				if err != nil {
					return err
				}
				defer func() { _ = close_() }()

				tasks.Start("readBarcodes", func(ctx context.Context) error {
					return readBarcodes(ctx, barcodeReader, beep, slog.Default())
				})
			}

			tasks.Start("webui", func(ctx context.Context) error {
				return webUI(ctx, todo, slog.Default())
			})

			homeAudio := homeaudioclient.New(homeaudioclient.HomeFn61)

			for {
				select {
				case err := <-tasks.Done():
					return err
				case barcode := <-beep:
					details, err := handleBeep(ctx, barcode, slog.Default(), todo)

					audioFeedback := func() string {
						if err != nil {
							slog.Error("handleBeep", "err", err)

							if errors.Is(err, errItemAlreadyOnShoppingList) {
								return "Item not added because it was already on the shopping list"
							} else {
								return "Error handling scanned barcode"
							}
						} else {
							if details.IsUnrecognizedBarcode() {
								return "Item added but name is unrecognized"
							} else if type_ := details.ProductType; type_ != "" {
								return "Added " + type_
							} else {
								return "Item added"
							}
						}
					}()

					if err := homeAudio.Speak(ctx, audioFeedback); err != nil {
						slog.Error("Home audio", "err", err)
					}
				}
			}
		},
	})

	app.AddCommand(&cobra.Command{
		Use:   "pretend-scanned",
		Short: "Act as though a barcode was scanned. Example input: 6408180733659",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			todo, err := getClient()
			if err != nil {
				return err
			}
			_, err = handleBeep(cmd.Context(), args[0], slog.Default(), todo)
			return err
		},
	})

	app.AddCommand(&cobra.Command{
		Use:   "misses-ls",
		Short: "List misses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			todo, err := getClient()
			if err != nil {
				return err
			}

			misses, err := listMisses(cmd.Context(), todo)
			if err != nil {
				return err
			}

			for _, miss := range misses {
				fmt.Println(miss)
			}

			return nil
		},
	})

	app.AddCommand(&cobra.Command{
		Use:   "misses-record [barcode] [productName]",
		Short: "Record a miss to the local DB so we remember it later",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			barcode := args[0]
			productName := args[1]

			todo, err := getClient()
			if err != nil {
				return err
			}

			return recordMissAndStoreToLocalDB(cmd.Context(), barcode, newProductDetails(productName, ""), todo)
		},
	})

	cli.Execute(app)
}

func handleBeep(ctx context.Context, barcode string, logger *slog.Logger, todo *todoist.Client) (*productDetails, error) {
	withErr := func(err error) (*productDetails, error) { return nil, fmt.Errorf("handleBeep: %w", err) }

	// better reload this on every beep so that if DB has been updated, the changes are reflected
	db, err := loadDB()
	if err != nil {
		return withErr(err)
	}

	details, err := func() (productDetails, error) {
		details, err := resolveProductDetailsByBarcode(ctx, barcode, db, todo, logger)
		if err != nil {
			slog.Error("handleBeep: unable to resolve", "barcode", barcode, "err", err)

			return newProductDetails(taskNameForUnnamedBarcode(barcode), ""), nil
		} else { // found
			details.LastScanned = Pointer(time.Now().UTC())

			(*db)[barcode] = *details

			if err := saveDB(*db); err != nil {
				return *details, err
			}

			return *details, nil
		}
	}()
	if err != nil {
		return withErr(err)
	}

	slog.Info("adding", "ProductName", details.Name)

	if err := addProductNameToShoppingList(ctx, details, createDescriptionMarkdown(barcode), todo); err != nil {
		return withErr(err)
	}

	return &details, nil
}

func recordMissAndStoreToLocalDB(ctx context.Context, barcode string, product productDetails, todo *todoist.Client) error {
	projectID, err := getTodoistProjectID()
	if err != nil {
		return err
	}

	existingTasks, err := todo.TasksByProject(ctx, projectID, time.Now())
	if err != nil {
		return err
	}

	taskNameForUnnamed := taskNameForUnnamedBarcode(barcode)

	// rename current tasks that refer to this unnamed task
	for _, missing := range lo.Filter(existingTasks, func(t todoist.Task, _ int) bool { return t.Content == taskNameForUnnamed }) {
		missing.Content = product.Name

		if err := todo.UpdateTask(ctx, missing); err != nil {
			return err
		}
	}

	db, err := loadDB()
	if err != nil {
		return err
	}

	(*db)[barcode] = product

	// now next time we will remember the proper name for this
	return saveDB(*db)
}

func resolveProductDetailsByBarcode(ctx context.Context, barcode string, resolveDB *LocalDB, todo *todoist.Client, logger *slog.Logger) (*productDetails, error) {
	withErr := func(err error) (*productDetails, error) {
		return nil, fmt.Errorf("resolveProductDetailsByBarcode: %w", err)
	}

	if product, found := localDBresolveProductByBarcode(barcode, resolveDB); found {
		return &product, nil
	}
	slog.Info("localDBresolveProductByBarcode: not found. continuing with web search")

	// https://en.wikipedia.org/wiki/List_of_GS1_country_codes
	if strings.HasPrefix(barcode, "2") {
		return withErr(errors.New("barcode begins with 2 which implies store-internal barcode - bailing out"))
	}

	if strings.HasPrefix(barcode, "https:") || strings.HasPrefix(barcode, "http:") {
		return withErr(errors.New("barcode encodes an (unrecognized) URL - bailing out"))
	}

	if l := len(barcode); l < 10 { // EAN should be 13. UPC should be 12.
		// store-internal barcodes (like Lidl) are not very searchable as they are too short numbers
		// which would lead to ambiguities. just tested with a Lidl toast and that resulted in wedding ring..
		return withErr(fmt.Errorf("length of barcode so short (%d) it implies store-internal barcode - bailing out", l))
	}

	searchEngine, err := googlesearch.New()
	if err != nil {
		return withErr(err)
	}

	barcodeSearchResults, err := searchEngine.Search(ctx, barcode)
	if err != nil {
		return withErr(err)
	}

	if len(barcodeSearchResults.Items) == 0 { // next steps needs there to be search results
		return withErr(fmt.Errorf("no web search results for barcode '%s'", barcode))
	}

	searchResultTitles := lo.Map(barcodeSearchResults.Items, func(result googlesearch.Item, _ int) string { return result.Title })

	product := func() *productDetails {
		link := barcodeSearchResults.Items[0].Link
		result, err := useAIAssistantToGuessProductDetailsFromSearchResults(ctx, searchResultTitles, link, logger)
		if err != nil {
			productNameGuess := strings.Split(searchResultTitles[0], " - ")[0]
			slog.Warn("AI guess of product details failed; falling back to first search result", "err", err, "fallback", productNameGuess)
			return Pointer(newProductDetails(productNameGuess, link))
		}

		return result
	}()

	if err := recordMissAndStoreToLocalDB(ctx, barcode, *product, todo); err != nil {
		// this is not critical error in context of this function's task
		logger.Error("recordMissAndStoreToLocalDB", "err", err)
	}

	return product, nil
}

var (
	errItemAlreadyOnShoppingList = errors.New("requested productName already on the list")
)

func addProductNameToShoppingList(ctx context.Context, product productDetails, description string, todo *todoist.Client) error {
	projectID, err := getTodoistProjectID()
	if err != nil {
		return err
	}

	existingTasks, err := todo.TasksByProject(ctx, projectID, time.Now())
	if err != nil {
		return err
	}

	if _, alreadyOnList := lo.Find(existingTasks, func(t todoist.Task) bool { return t.Content == product.Name }); alreadyOnList {
		return errItemAlreadyOnShoppingList
	}

	order := 0
	if idx := slices.Index(productCategories, product.ProductCategory); idx != -1 {
		order = 10000 + (idx * 100)
	}

	return todo.CreateTask(ctx, todoist.Task{
		Content:     product.Name,
		Description: description,
		ProjectID:   strconv.Itoa(int(projectID)),
		Order:       order,
	})
}

func listMisses(ctx context.Context, todo *todoist.Client) ([]string, error) {
	projectID, err := getTodoistProjectID()
	if err != nil {
		return nil, err
	}

	existingTasks, err := todo.TasksByProject(ctx, projectID, time.Now())
	if err != nil {
		return nil, err
	}

	return lo.FilterMap(existingTasks, func(t todoist.Task, _ int) (string, bool) {
		match := identifyMissRe.FindStringSubmatch(t.Content)
		if match == nil {
			return "", false
		}

		return match[1], true
	}), nil
}

func taskNameForUnnamedBarcode(barcode string) string {
	return fmt.Sprintf("unrecognized barcode[%s]", barcode)
}

// use as description (which supports Markdown) a link to the item, so we have access to all the details
// (like barcode, web search etc.) in the task
func createDescriptionMarkdown(barcode string) string {
	// searchURL := fmt.Sprintf("https://google.com/search?q=%s", url.QueryEscape(barcode))
	baseURL := os.Getenv("WEBAPP_BASEURL")
	linkToWebui := baseURL + appHomeRoute + "item/" + url.PathEscape(barcode)
	return fmt.Sprintf("[Details](%s)", linkToWebui)
}

var identifyMissRe = regexp.MustCompile(`^unrecognized barcode\[([0-9]+)\]$`)

func getClient() (*todoist.Client, error) {
	tok, err := osutil.GetenvRequired("TODOIST_TOKEN")

	return todoist.NewClient(tok), err
}

func getTodoistProjectID() (int64, error) {
	projectIDStr, err := osutil.GetenvRequired("TODOIST_PROJECT_ID")
	if err != nil {
		return 0, err
	}

	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		return 0, err
	}

	return int64(projectID), nil
}

func newProductDetails(productName string, link string) productDetails {
	return productDetails{
		Name:         productName,
		Link:         link,
		FirstScanned: Pointer(time.Now().UTC()),
	}
}

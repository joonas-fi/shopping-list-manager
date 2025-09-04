package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/function61/gokit/app/cli"
	"github.com/function61/gokit/app/dynversion"
	"github.com/function61/gokit/app/evdev"
	. "github.com/function61/gokit/builtin"
	"github.com/function61/gokit/log/logex"
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
		Use:     os.Args[0],
		Short:   "Shopping list manager",
		Version: dynversion.Version,
	}

	app.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Listen for barcode scans from a barcode reader and add their product names to shopping list",
		Args:  cobra.NoArgs,
		Run: cli.RunnerNoArgs(func(ctx context.Context, logger *log.Logger) error {
			todo, err := getClient()
			if err != nil {
				return err
			}

			barcodeReaderDevicePath := FirstNonEmpty(os.Getenv("BARCODE_READER"), "/dev/barcode-reader")

			barcodeReader, close_, err := evdev.Open(barcodeReaderDevicePath)
			if err != nil {
				return err
			}
			defer func() { _ = close_() }()

			beep := make(chan string, 2)

			tasks := taskrunner.New(ctx, slog.Default())

			tasks.Start("readBarcodes", func(ctx context.Context) error {
				return readBarcodes(ctx, barcodeReader, beep, logger)
			})

			tasks.Start("webui", func(ctx context.Context) error {
				return webUI(ctx, todo, logger)
			})

			homeAudio := homeaudioclient.New(homeaudioclient.HomeFn61)

			for {
				select {
				case err := <-tasks.Done():
					return err
				case barcode := <-beep:
					wasUnrecognized, err := handleBeep(ctx, barcode, logger, todo)

					audioFeedback := func() string {
						if err != nil {
							logex.Levels(logger).Error.Println(err.Error())

							if errors.Is(err, errItemAlreadyOnShoppingList) {
								return "Item not added because it was already on the shopping list"
							} else {
								return "Error handling scanned barcode"
							}
						} else {
							if wasUnrecognized {
								return "Item added but name is unrecognized"
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
		}),
	})

	app.AddCommand(&cobra.Command{
		Use:   "pretend-scanned",
		Short: "Act as though a barcode was scanned. Example input: 6408180733659",
		Args:  cobra.ExactArgs(1),
		Run: cli.Runner(func(ctx context.Context, args []string, logger *log.Logger) error {
			todo, err := getClient()
			if err != nil {
				return err
			}
			_, err = handleBeep(ctx, args[0], logger, todo)
			return err
		}),
	})

	app.AddCommand(&cobra.Command{
		Use:   "misses-ls",
		Short: "List misses",
		Args:  cobra.NoArgs,
		Run: cli.RunnerNoArgs(func(ctx context.Context, logger *log.Logger) error {
			todo, err := getClient()
			if err != nil {
				return err
			}

			misses, err := listMisses(ctx, todo)
			if err != nil {
				return err
			}

			for _, miss := range misses {
				fmt.Println(miss)
			}

			return nil
		}),
	})

	app.AddCommand(&cobra.Command{
		Use:   "misses-record [barcode] [productName]",
		Short: "Record a miss to the local DB so we remember it later",
		Args:  cobra.ExactArgs(2),
		Run: cli.Runner(func(ctx context.Context, args []string, logger *log.Logger) error {
			barcode := args[0]
			productName := args[1]

			todo, err := getClient()
			if err != nil {
				return err
			}

			return recordMissAndStoreToLocalDB(ctx, barcode, newProductDetails(productName, ""), todo)
		}),
	})

	osutil.ExitIfError(app.Execute())
}

func handleBeep(ctx context.Context, barcode string, logger *log.Logger, todo *todoist.Client) (bool, error) {
	withErr := func(err error) (bool, error) { return true, fmt.Errorf("handleBeep: %w", err) }

	// better reload this on every beep so that if DB has been updated, the changes are reflected
	db, err := loadDB()
	if err != nil {
		return withErr(err)
	}

	details, wasUnrecognized, err := func() (productDetails, bool, error) {
		details, err := resolveProductDetailsByBarcode(ctx, barcode, db, todo, logger)
		if err != nil {
			logex.Levels(logger).Error.Printf("unable to resolve '%s' to product name: %v", barcode, err)

			details = Pointer(newProductDetails(taskNameForUnnamedBarcode(barcode), ""))

			return *details, true, nil
		} else { // found
			details.LastScanned = Pointer(time.Now().UTC())

			(*db)[barcode] = *details

			if err := saveDB(*db); err != nil {
				return *details, false, err
			}

			return *details, false, nil
		}
	}()
	if err != nil {
		return withErr(err)
	}

	logex.Levels(logger).Info.Printf("adding '%s'", details.Name)

	if err := addProductNameToShoppingList(ctx, details.Name, createDescriptionMarkdown(barcode), todo); err != nil {
		return withErr(err)
	}

	return wasUnrecognized, nil
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

func resolveProductDetailsByBarcode(ctx context.Context, barcode string, resolveDB *LocalDB, todo *todoist.Client, logger *log.Logger) (*productDetails, error) {
	withErr := func(err error) (*productDetails, error) {
		return nil, fmt.Errorf("resolveProductDetailsByBarcode: %w", err)
	}

	if product, found := localDBresolveProductByBarcode(barcode, resolveDB); found {
		return &product, nil
	}
	logex.Levels(logger).Info.Println("localDBresolveProductByBarcode: not found. continuing with web search")

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

	productNameGuess, err := useAIAssistantToGuessProductNameFromSearchResults(ctx, searchResultTitles, logger)
	if err != nil {
		productNameGuess = strings.Split(searchResultTitles[0], " - ")[0]
		slog.Warn("AI guess of product name failed; falling back to first search result", "err", err, "fallback", productNameGuess)
	}

	product := newProductDetails(productNameGuess, barcodeSearchResults.Items[0].Link)

	if err := recordMissAndStoreToLocalDB(ctx, barcode, product, todo); err != nil {
		// this is not critical error in context of this function's task
		logex.Levels(logger).Error.Println(err.Error())
	}

	return &product, nil
}

var (
	errItemAlreadyOnShoppingList = errors.New("requested productName already on the list")
)

func addProductNameToShoppingList(ctx context.Context, productName string, description string, todo *todoist.Client) error {
	projectID, err := getTodoistProjectID()
	if err != nil {
		return err
	}

	existingTasks, err := todo.TasksByProject(ctx, projectID, time.Now())
	if err != nil {
		return err
	}

	_, alreadyOnList := lo.Find(existingTasks, func(t todoist.Task) bool { return t.Content == productName })
	if alreadyOnList {
		return errItemAlreadyOnShoppingList
	}

	return todo.CreateTask(ctx, todoist.Task{
		Content:     productName,
		Description: description,
		ProjectID:   strconv.Itoa(int(projectID)),
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
	return productDetails{Name: productName, Link: link, FirstScanned: Pointer(time.Now().UTC())}
}

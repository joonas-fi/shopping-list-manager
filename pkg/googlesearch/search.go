// Google search using the custom search JSON API: https://developers.google.com/custom-search/v1/overview
//
// At time of implementation:
// > provides 100 search queries per day for free.
package googlesearch

import (
	"context"
	"fmt"
	"net/url"

	"github.com/function61/gokit/net/http/ezhttp"
	"github.com/function61/gokit/os/osutil"
)

type googleCustomSearchClient struct {
	customSearchEngineID string
	apiKey               string
}

func New() (*googleCustomSearchClient, error) {
	customSearchEngineID, err := osutil.GetenvRequired("GOOGLE_SEARCH_CUSTOM_SEARCH_ENGINE_ID")
	if err != nil {
		return nil, err
	}

	apiKey, err := osutil.GetenvRequired("GOOGLE_SEARCH_API_KEY")
	if err != nil {
		return nil, err
	}

	return &googleCustomSearchClient{
		customSearchEngineID: customSearchEngineID,
		apiKey:               apiKey,
	}, nil
}

func (g *googleCustomSearchClient) Search(ctx context.Context, query string) (*CustomSearch, error) {
	withErr := func(err error) (*CustomSearch, error) { return nil, fmt.Errorf("googlesearch.Search: %w", err) }

	v := func(val string) []string { return []string{val} }

	queryParams := url.Values{
		"cx":  v(g.customSearchEngineID),
		"key": v(g.apiKey),
		"q":   v(query),
	}

	cs := &CustomSearch{}
	if _, err := ezhttp.Get(ctx, "https://www.googleapis.com/customsearch/v1?"+queryParams.Encode(), ezhttp.RespondsJSONAllowUnknownFields(cs)); err != nil {
		return withErr(err)
	}

	return cs, nil
}

// datatypes (generated from actual search result with ChatGPT)

type CustomSearch struct {
	Kind              string            `json:"kind"`
	URL               URL               `json:"url"`
	Queries           Queries           `json:"queries"`
	Context           Context           `json:"context"`
	SearchInformation SearchInformation `json:"searchInformation"`
	Items             []Item            `json:"items"`
}

type URL struct {
	Type     string `json:"type"`
	Template string `json:"template"`
}

type Queries struct {
	Request  []PageInfo `json:"request"`
	NextPage []PageInfo `json:"nextPage"`
}

type PageInfo struct {
	Title          string `json:"title"`
	TotalResults   string `json:"totalResults"`
	SearchTerms    string `json:"searchTerms"`
	Count          int    `json:"count"`
	StartIndex     int    `json:"startIndex"`
	InputEncoding  string `json:"inputEncoding"`
	OutputEncoding string `json:"outputEncoding"`
	Safe           string `json:"safe"`
	Cx             string `json:"cx"`
}

type Context struct {
	Title string `json:"title"`
}

type SearchInformation struct {
	SearchTime            float64 `json:"searchTime"`
	FormattedSearchTime   string  `json:"formattedSearchTime"`
	TotalResults          string  `json:"totalResults"`
	FormattedTotalResults string  `json:"formattedTotalResults"`
}

type Item struct {
	Kind             string  `json:"kind"`
	Title            string  `json:"title"`
	HTMLTitle        string  `json:"htmlTitle"`
	Link             string  `json:"link"`
	DisplayLink      string  `json:"displayLink"`
	Snippet          string  `json:"snippet"`
	HTMLSnippet      string  `json:"htmlSnippet"`
	FormattedURL     string  `json:"formattedUrl"`
	HTMLFormattedURL string  `json:"htmlFormattedUrl"`
	Pagemap          Pagemap `json:"pagemap"`
}

type Pagemap struct {
	Offer        []Offer        `json:"offer"`
	CSEThumbnail []CSEThumbnail `json:"cse_thumbnail"`
	Product      []Product      `json:"product"`
	Metatags     []Metatag      `json:"metatags"`
	CSEImage     []CSEImage     `json:"cse_image"`
	HProduct     []HProduct     `json:"hproduct"`
	ListItem     []ListItem     `json:"listitem"`
}

type Offer struct {
	PriceCurrency string `json:"pricecurrency"`
	Price         string `json:"price"`
	Availability  string `json:"availability"`
	URL           string `json:"url"`
}

type CSEThumbnail struct {
	Src    string `json:"src"`
	Width  string `json:"width"`
	Height string `json:"height"`
}

type Product struct {
	Image       string `json:"image"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Metatag struct {
	OgImage                    string `json:"og:image"`
	OgType                     string `json:"og:type"`
	OgSiteName                 string `json:"og:site_name"`
	Viewport                   string `json:"viewport"`
	OgTitle                    string `json:"og:title"`
	OgLocale                   string `json:"og:locale"`
	YandexVerification         string `json:"yandex-verification"`
	OgURL                      string `json:"og:url"`
	OgDescription              string `json:"og:description"`
	FormatDetection            string `json:"format-detection"`
	AppleItunesApp             string `json:"apple-itunes-app,omitempty"`
	NextHeadCount              string `json:"next-head-count,omitempty"`
	FacebookDomainVerification string `json:"facebook-domain-verification,omitempty"`
	TwitterCard                string `json:"twitter:card,omitempty"`
	TwitterTitle               string `json:"twitter:title,omitempty"`
	TwitterSite                string `json:"twitter:site,omitempty"`
	TwitterDescription         string `json:"twitter:description,omitempty"`
	OgLocaleAlternate          string `json:"og:locale:alternate,omitempty"`
	MsapplicationTileColor     string `json:"msapplication-tilecolor,omitempty"`
	ThemeColor                 string `json:"theme-color,omitempty"`
	Msvalidate01               string `json:"msvalidate.01,omitempty"`
	Version                    string `json:"version,omitempty"`
}

type CSEImage struct {
	Src string `json:"src"`
}

type HProduct struct {
	Fn          string `json:"fn"`
	Description string `json:"description"`
	Photo       string `json:"photo"`
	Currency    string `json:"currency"`
	CurrencyISO string `json:"currency_iso4217"`
}

type ListItem struct {
	Item     string `json:"item"`
	Name     string `json:"name"`
	Position string `json:"position"`
}

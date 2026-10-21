package online

import (
    "cmp"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "html"
    "net/http"
    "net/url"
    "regexp"
    "slices"
    "strings"
    "time"
)

const (
    FewRatings     = 100
    requestTimeout = 15 * time.Second
    searchResults  = 25
    responseGroups = "rating,product_desc,product_attrs,product_extended_attrs,contributors"
    userAgent      = "AudiobookPlayer/1.0 (+https://github.com/izzzicos/audiobook-player)"
)

type Store struct {
    Code string
    Name string
    Host string
    Site string
}

var Stores = []Store{
    {"us", "United States", "api.audible.com", "www.audible.com"},
    {"uk", "United Kingdom", "api.audible.co.uk", "www.audible.co.uk"},
    {"ca", "Canada", "api.audible.ca", "www.audible.ca"},
    {"au", "Australia", "api.audible.com.au", "www.audible.com.au"},
    {"in", "India", "api.audible.in", "www.audible.in"},
    {"de", "Germany", "api.audible.de", "www.audible.de"},
    {"fr", "France", "api.audible.fr", "www.audible.fr"},
    {"it", "Italy", "api.audible.it", "www.audible.it"},
    {"es", "Spain", "api.audible.es", "www.audible.es"},
    {"jp", "Japan", "api.audible.co.jp", "www.audible.co.jp"},
}

func StoreFor(code string) Store {
    for _, store := range Stores {
        if store.Code == code {
            return store
        }
    }

    return Stores[0]
}

func StoreNamed(name string) Store {
    for _, store := range Stores {
        if store.Name == name {
            return store
        }
    }

    return Stores[0]
}

type Edition struct {
    ASIN        string   `json:"asin"`
    Title       string   `json:"title"`
    Subtitle    string   `json:"subtitle,omitempty"`
    Authors     []string `json:"authors,omitempty"`
    Narrators   []string `json:"narrators,omitempty"`
    Released    string   `json:"released,omitempty"`
    Minutes     int      `json:"minutes,omitempty"`
    Rating      float64  `json:"rating,omitempty"`
    Ratings     int      `json:"ratings,omitempty"`
    Description string   `json:"description,omitempty"`
}

func (e Edition) available() bool {
    return e.ASIN != "" && e.Title != ""
}

func (e Edition) Rated() bool {
    return e.Ratings > 0
}

func (e Edition) FewRatings() bool {
    return e.Ratings < FewRatings
}

func (e Edition) Stars() string {
    if !e.Rated() {
        return ""
    }

    return fmt.Sprintf("★ %.1f · %s", e.Rating, ShortCount(e.Ratings))
}

func (e Edition) RatingText() string {
    if !e.Rated() {
        return "No ratings on Audible yet"
    }

    text := fmt.Sprintf("★ %.1f · %s %s on Audible", e.Rating, GroupedCount(e.Ratings), plural(e.Ratings, "rating", "ratings"))
    if e.FewRatings() {
        text += " · few ratings, so this may not be reliable"
    }

    return text
}

func (e Edition) Year() string {
    if len(e.Released) >= 4 {
        return e.Released[:4]
    }

    return ""
}

func (e Edition) Link(store Store) string {
    return "https://" + store.Site + "/pd/" + url.PathEscape(e.ASIN)
}

func ShortCount(count int) string {
    switch {
    case count >= 1_000_000:
        return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(count)/1_000_000), ".0") + "M"
    case count >= 10_000:
        return fmt.Sprintf("%dk", count/1000)
    case count >= 1000:
        return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(count)/1000), ".0") + "k"
    default:
        return fmt.Sprint(count)
    }
}

func GroupedCount(count int) string {
    digits := fmt.Sprint(count)
    var grouped strings.Builder
    for i, digit := range digits {
        if i > 0 && (len(digits)-i)%3 == 0 {
            grouped.WriteByte(',')
        }
        grouped.WriteRune(digit)
    }

    return grouped.String()
}

func plural(count int, singular, many string) string {
    if count == 1 {
        return singular
    }

    return many
}

var asinPattern = regexp.MustCompile(`^[0-9A-Z]{10}$`)

func ValidASIN(asin string) bool {
    return asinPattern.MatchString(asin)
}

var ErrUnreachable = errors.New("Audible couldn't be reached. Check your internet connection and try again")

type unreachable struct {
    cause error
}

func (u unreachable) Error() string {
    return ErrUnreachable.Error()
}

func (u unreachable) Unwrap() []error {
    return []error{ErrUnreachable, u.cause}
}

type Client struct {
    HTTP    *http.Client
    BaseURL func(Store) string
}

func NewClient() *Client {
    return &Client{HTTP: &http.Client{Timeout: requestTimeout}}
}

func (c *Client) base(store Store) string {
    if c.BaseURL != nil {
        return c.BaseURL(store)
    }

    return "https://" + store.Host
}

func (c *Client) Search(ctx context.Context, store Store, title, author string) ([]Edition, error) {
    query := url.Values{
        "num_results":      {fmt.Sprint(searchResults)},
        "products_sort_by": {"Relevance"},
        "response_groups":  {responseGroups},
    }
    if title = strings.TrimSpace(title); title != "" {
        query.Set("title", title)
    }
    if author = strings.TrimSpace(author); author != "" {
        query.Set("author", author)
    }

    var response struct {
        Products []product `json:"products"`
    }
    if err := c.get(ctx, store, "/1.0/catalog/products", query, &response); err != nil {
        return nil, err
    }

    var editions []Edition
    for _, product := range response.Products {
        if edition := product.edition(); edition.available() {
            editions = append(editions, edition)
        }
    }
    slices.SortStableFunc(editions, func(a, b Edition) int {
        return cmp.Compare(b.Ratings, a.Ratings)
    })

    return editions, nil
}

func (c *Client) Lookup(ctx context.Context, store Store, asin string) (Edition, error) {
    if !ValidASIN(asin) {
        return Edition{}, fmt.Errorf("%q isn't an Audible product number", asin)
    }

    var response struct {
        Product product `json:"product"`
    }
    if err := c.get(ctx, store, "/1.0/catalog/products/"+asin, url.Values{"response_groups": {responseGroups}}, &response); err != nil {
        return Edition{}, err
    }

    edition := response.Product.edition()
    if !edition.available() {
        return Edition{}, fmt.Errorf("Audible has no book with the number %s in its %s store", asin, store.Name)
    }

    return edition, nil
}

func (c *Client) get(ctx context.Context, store Store, path string, query url.Values, value any) error {
    request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base(store)+path+"?"+query.Encode(), nil)
    if err != nil {
        return err
    }
    request.Header.Set("Accept", "application/json")
    request.Header.Set("User-Agent", userAgent)

    client := c.HTTP
    if client == nil {
        client = &http.Client{Timeout: requestTimeout}
    }

    response, err := client.Do(request)
    if err != nil {
        return unreachable{cause: err}
    }
    defer response.Body.Close()

    switch {
    case response.StatusCode == http.StatusNotFound:
        return fmt.Errorf("Audible's %s store doesn't have this book", store.Name)
    case response.StatusCode == http.StatusTooManyRequests:
        return errors.New("Audible is getting too many requests right now. Try again in a few minutes")
    case response.StatusCode != http.StatusOK:
        return fmt.Errorf("Audible couldn't answer just now (error %d). Try again later", response.StatusCode)
    }

    if err := json.NewDecoder(response.Body).Decode(value); err != nil {
        return fmt.Errorf("Audible's answer couldn't be read: %w", err)
    }

    return nil
}

type person struct {
    Name string `json:"name"`
}

type product struct {
    ASIN                 string   `json:"asin"`
    Title                string   `json:"title"`
    Subtitle             string   `json:"subtitle"`
    Authors              []person `json:"authors"`
    Narrators            []person `json:"narrators"`
    ReleaseDate          string   `json:"release_date"`
    Runtime              int      `json:"runtime_length_min"`
    PublisherSummary     string   `json:"publisher_summary"`
    MerchandisingSummary string   `json:"merchandising_summary"`
    Rating               struct {
        Overall struct {
            Average float64 `json:"average_rating"`
            Count   int     `json:"num_ratings"`
        } `json:"overall_distribution"`
    } `json:"rating"`
}

func (p product) edition() Edition {
    return Edition{
        ASIN:        p.ASIN,
        Title:       p.Title,
        Subtitle:    p.Subtitle,
        Authors:     names(p.Authors),
        Narrators:   names(p.Narrators),
        Released:    p.ReleaseDate,
        Minutes:     p.Runtime,
        Rating:      p.Rating.Overall.Average,
        Ratings:     p.Rating.Overall.Count,
        Description: PlainText(cmp.Or(p.PublisherSummary, p.MerchandisingSummary)),
    }
}

func names(people []person) []string {
    var result []string
    for _, someone := range people {
        if name := strings.TrimSpace(someone.Name); name != "" {
            result = append(result, name)
        }
    }

    return result
}

var (
    breaks     = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</li>|</div>|</h\d>`)
    listItems  = regexp.MustCompile(`(?i)<li[^>]*>`)
    tags       = regexp.MustCompile(`<[^>]*>`)
    spaces     = regexp.MustCompile(`[ \t\x{00a0}]+`)
    blankLines = regexp.MustCompile(`\n\s*\n+`)
    openSpace  = regexp.MustCompile(`([(\[]) +`)
    closeSpace = regexp.MustCompile(` +([)\]])`)
)

func PlainText(markup string) string {
    text := breaks.ReplaceAllString(markup, "\n")
    text = listItems.ReplaceAllString(text, "• ")
    text = tags.ReplaceAllString(text, "")
    text = html.UnescapeString(text)
    text = spaces.ReplaceAllString(text, " ")
    text = openSpace.ReplaceAllString(text, "$1")
    text = closeSpace.ReplaceAllString(text, "$1")

    lines := strings.Split(text, "\n")
    for i, line := range lines {
        lines[i] = strings.TrimSpace(line)
    }
    text = strings.Join(lines, "\n")

    return strings.TrimSpace(blankLines.ReplaceAllString(text, "\n\n"))
}

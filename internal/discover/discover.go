package discover

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bedri/open-directory-crawler/internal/classify"
	"github.com/bedri/open-directory-crawler/internal/envutil"
	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/parser"
	"github.com/bedri/open-directory-crawler/internal/soundclick"
)

const maxResponseSize = 10 << 20

func readAllLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxResponseSize))
}

type Result struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Source   string `json:"source"`
	Verified bool   `json:"verified"`
}

type Finder struct {
	client     *http.Client
	userAgents []string
	googleKey  string
	googleCX   string
	bingKey    string
	shodanKey  string
	searxngURL string
}

func New() *Finder {
	f := &Finder{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; rv:133.0) Gecko/20100101 Firefox/133.0",
		},
		googleKey:  envutil.Get("ODK_GOOGLE_KEY", ""),
		googleCX:   envutil.Get("ODK_GOOGLE_CX", ""),
		bingKey:    envutil.Get("ODK_BING_KEY", ""),
		shodanKey:  envutil.Get("ODK_SHODAN_KEY", ""),
		searxngURL: envutil.Get("ODK_SEARXNG_URL", "https://searx.be"),
	}
	return f
}

func (f *Finder) randomUA() string {
	if len(f.userAgents) == 0 {
		return "ODK/1.0"
	}
	return f.userAgents[rand.Intn(len(f.userAgents))]
}

var TypeDorks = map[string][]string{
	"audio": {
		`intitle:"index of" mp3 -inurl:(jsp|php|asp|aspx|cgi)`,
		`intitle:"index of" flac -inurl:(jsp|php|asp|aspx|cgi)`,
		`"Index of /" music -inurl:(jsp|php|asp)`,
		`intitle:"index of" "parent directory" "audio"`,
		`"Index of /" albums -inurl:(jsp|php|asp)`,
		`"Index of /" Audio -inurl:(jsp|php|asp)`,
	},
	"video": {
		`intitle:"index of" mp4 -inurl:(jsp|php|asp|aspx|cgi)`,
		`intitle:"index of" mkv -inurl:(jsp|php|asp|aspx|cgi)`,
		`"Index of /" movies -inurl:(jsp|php|asp)`,
		`"Index of /" video -inurl:(jsp|php|asp)`,
		`intitle:"index of" "parent directory" avi`,
		`"Index of /" tv -inurl:(jsp|php|asp)`,
	},
	"image": {
		`intitle:"index of" jpg -inurl:(jsp|php|asp|aspx|cgi)`,
		`intitle:"index of" png -inurl:(jsp|php|asp|aspx|cgi)`,
		`"Index of /" images -inurl:(jsp|php|asp)`,
		`"Index of /" pictures -inurl:(jsp|php|asp)`,
		`"Index of /" photos -inurl:(jsp|php|asp)`,
	},
	"document": {
		`intitle:"index of" pdf -inurl:(jsp|php|asp|aspx|cgi)`,
		`"Index of /" books -inurl:(jsp|php|asp)`,
		`"Index of /" documents -inurl:(jsp|php|asp)`,
		`"Index of /" ebooks -inurl:(jsp|php|asp)`,
		`intitle:"index of" epub -inurl:(jsp|php|asp)`,
	},
	"archive": {
		`intitle:"index of" zip -inurl:(jsp|php|asp|aspx|cgi)`,
		`"Index of /" archives -inurl:(jsp|php|asp)`,
		`"Index of /" backup -inurl:(jsp|php|asp)`,
		`"Index of /" iso -inurl:(jsp|php|asp)`,
	},
	"code": {
		`intitle:"index of" "parent directory" "src"`,
		`"Index of /" source -inurl:(jsp|php|asp)`,
		`"Index of /" src -inurl:(jsp|php|asp)`,
		`intitle:"index of" "parent directory" "code"`,
	},
}

var FTPDorks = []string{
	`"Index of /" ftp -inurl:(jsp|php|asp)`,
	`intitle:"index of" ftp://`,
	`"Index of /" "ftp" "parent directory"`,
	`"Directory Listing" ftp://`,
}

var Dorks = []string{
	`intitle:"index of" "parent directory" -inurl:(jsp|php|asp|aspx|cgi)`,
	`intitle:"index of /" "name" "last modified" "size"`,
	`"Index of /" "Apache" "Parent Directory"`,
	`"Index of /" "nginx" "Parent Directory"`,
	`intitle:"Directory Listing" -inurl:(jsp|php|asp)`,
	`"Index of /" mp4 -inurl:(php|asp|jsp)`,
	`"Index of /" movies -inurl:(php|asp|jsp)`,
	`"Index of /" books -inurl:(php|asp|jsp)`,
}

func (f *Finder) DiscoverAll() ([]Result, error) {
	var all []Result
	seen := map[string]bool{}

	addUnique := func(r Result) {
		u := strings.TrimRight(r.URL, "/")
		if !seen[u] {
			seen[u] = true
			all = append(all, r)
		}
	}

	results, _ := f.SearchGoogle()
	for _, r := range results {
		addUnique(r)
	}

	results, _ = f.SearchBing()
	for _, r := range results {
		addUnique(r)
	}

	results, _ = f.SearchDuckDuckGo()
	for _, r := range results {
		addUnique(r)
	}

	results, _ = f.SearchSearXNG()
	for _, r := range results {
		addUnique(r)
	}

	results, _ = f.ScrapeAggregators()
	for _, r := range results {
		addUnique(r)
	}

	results, _ = f.SearchShodan()
	for _, r := range results {
		addUnique(r)
	}

	results, _ = f.SearchSoundClick("")
	for _, r := range results {
		addUnique(r)
	}

	results, _ = f.SearchFTP()
	for _, r := range results {
		addUnique(r)
	}

	var verified []Result
	for _, r := range all {
		if f.IsOpenDirectory(r.URL) {
			r.Verified = true
			verified = append(verified, r)
		}
	}
	return verified, nil
}

func (f *Finder) SearchGoogle() ([]Result, error) {
	if f.googleKey != "" && f.googleCX != "" {
		return f.searchGoogleAPI()
	}
	var all []Result
	for _, dork := range Dorks {
		results, err := f.searchGoogleScrape(dork, 1)
		if err != nil {
			continue
		}
		all = append(all, results...)
		time.Sleep(time.Duration(2+rand.Intn(3)) * time.Second)
	}
	return all, nil
}

func (f *Finder) searchGoogleAPI() ([]Result, error) {
	var all []Result
	for _, dork := range Dorks {
		url := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s",
			f.googleKey, f.googleCX, url.QueryEscape(dork))
		req, _ := http.NewRequest("GET", url, nil)
		resp, err := f.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := readAllLimited(resp.Body)
		resp.Body.Close()

		var gRes googleAPIResponse
		if json.Unmarshal(body, &gRes) != nil {
			continue
		}
		for _, item := range gRes.Items {
			all = append(all, Result{URL: item.Link, Source: "google", Title: item.Title})
		}
		time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
	}
	return all, nil
}

type googleAPIResponse struct {
	Items []struct {
		Link  string `json:"link"`
		Title string `json:"title"`
	} `json:"items"`
}

func (f *Finder) searchGoogleScrape(query string, pages int) ([]Result, error) {
	var all []Result
	for page := 0; page < pages; page++ {
		start := page * 10
		searchURL := fmt.Sprintf("https://www.google.com/search?q=%s&start=%d",
			url.QueryEscape(query), start)

		req, err := http.NewRequest("GET", searchURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", f.randomUA())
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Referer", "https://www.google.com/")

		resp, err := f.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := readAllLimited(resp.Body)
		resp.Body.Close()

		results := parseGoogleResults(string(body), query)
		all = append(all, results...)
		time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
	}
	return all, nil
}

func parseGoogleResults(body, query string) []Result {
	var results []Result
	re := regexp.MustCompile(`<a href="/url\?q=([^"&]+)`)
	matches := re.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		decoded, err := url.QueryUnescape(m[1])
		if err != nil {
			continue
		}
		results = append(results, Result{
			URL:    decoded,
			Source: "google",
		})
	}
	return results
}

func (f *Finder) SearchBing() ([]Result, error) {
	if f.bingKey != "" {
		return f.searchBingAPI()
	}
	return f.searchBingScrape()
}

func (f *Finder) searchBingAPI() ([]Result, error) {
	var all []Result
	queries := []string{
		`"index of" "parent directory"`,
		`"Index of /" mp4`,
		`"Index of /" movies`,
		`intitle:"index of"`,
	}

	for _, q := range queries {
		url := fmt.Sprintf("https://api.bing.microsoft.com/v7.0/search?q=%s&count=50",
			url.QueryEscape(q))
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Ocp-Apim-Subscription-Key", f.bingKey)

		resp, err := f.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := readAllLimited(resp.Body)
		resp.Body.Close()

		var bRes bingAPIResponse
		if json.Unmarshal(body, &bRes) != nil {
			continue
		}
		for _, w := range bRes.WebPages.Value {
			all = append(all, Result{URL: w.URL, Source: "bing", Title: w.Name})
		}
		time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
	}
	return all, nil
}

type bingAPIResponse struct {
	WebPages struct {
		Value []struct {
			URL  string `json:"url"`
			Name string `json:"name"`
		} `json:"value"`
	} `json:"webPages"`
}

func (f *Finder) searchBingScrape() ([]Result, error) {
	var all []Result
	queries := []string{
		`"index of" "parent directory"`,
		`"Index of /" mp4`,
		`"Index of /" movies`,
		`intitle:"index of"`,
	}

	bingLinkRe := regexp.MustCompile(`href="(/ck/[^"]*\bu=([A-Za-z0-9+/=_-]+)[^"]*)"`)

	for _, q := range queries {
		searchURL := fmt.Sprintf("https://www.bing.com/search?q=%s", url.QueryEscape(q))
		req, _ := http.NewRequest("GET", searchURL, nil)
		req.Header.Set("User-Agent", f.randomUA())
		req.Header.Set("Accept", "text/html,application/xhtml+xml")

		resp, err := f.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := readAllLimited(resp.Body)
		resp.Body.Close()

		matches := bingLinkRe.FindAllStringSubmatch(string(body), -1)
		for _, m := range matches {
			if len(m) < 3 {
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(m[2])
			if err != nil {
				decoded, err = base64.RawStdEncoding.DecodeString(m[2])
				if err != nil {
					continue
				}
			}
			decodedURL := string(decoded)
			if !strings.HasPrefix(decodedURL, "http") {
				continue
			}
			all = append(all, Result{
				URL:    decodedURL,
				Source: "bing",
			})
		}
		time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
	}
	return all, nil
}

func (f *Finder) SearchDuckDuckGo() ([]Result, error) {
	var all []Result
	queries := []string{
		`"index of" "parent directory"`,
		`"Index of /" mp4`,
		`"Index of /" movies`,
		`"Index of /" books`,
		`intitle:"index of"`,
	}

	ddgRe := regexp.MustCompile(`<a[^>]+class="result__a"[^>]+href="(https?://[^"]+)"`)

	for _, q := range queries {
		searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(q))
		req, _ := http.NewRequest("GET", searchURL, nil)
		req.Header.Set("User-Agent", f.randomUA())
		req.Header.Set("Accept", "text/html,application/xhtml+xml")

		resp, err := f.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := readAllLimited(resp.Body)
		resp.Body.Close()

		matches := ddgRe.FindAllStringSubmatch(string(body), -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			decoded, err := url.QueryUnescape(m[1])
			if err != nil {
				continue
			}
			all = append(all, Result{
				URL:    decoded,
				Source: "duckduckgo",
			})
		}
		time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
	}
	return all, nil
}

func (f *Finder) SearchSearXNG() ([]Result, error) {
	var all []Result
	queries := []string{
		`intitle:"index of" "parent directory"`,
		`"Index of /" mp4`,
		`"Index of /" movies`,
		`"Index of /" books`,
		`"Index of /" music`,
		`intitle:"Directory Listing"`,
	}

	for _, q := range queries {
		url := fmt.Sprintf("%s/search?q=%s&format=json&language=en-US&categories=general",
			f.searxngURL, url.QueryEscape(q))
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", f.randomUA())
		req.Header.Set("Accept", "application/json")

		resp, err := f.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := readAllLimited(resp.Body)
		resp.Body.Close()

		var sRes searxngResponse
		if json.Unmarshal(body, &sRes) != nil {
			continue
		}
		for _, r := range sRes.Results {
			all = append(all, Result{URL: r.URL, Source: "searxng", Title: r.Title})
		}
		time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
	}
	return all, nil
}

type searxngResponse struct {
	Results []struct {
		URL     string `json:"url"`
		Title   string `json:"title"`
		Content string `json:"content"`
	} `json:"results"`
}

func (f *Finder) SearchShodan() ([]Result, error) {
	if f.shodanKey == "" {
		return nil, nil
	}

	queries := []string{
		`"Index of /" "Parent Directory"`,
		`"Directory Listing"`,
		`"Apache" "Index of"`,
	}

	var all []Result
	for _, q := range queries {
		url := fmt.Sprintf("https://api.shodan.io/shodan/host/search?key=%s&query=%s&limit=100",
			f.shodanKey, url.QueryEscape(q))
		req, _ := http.NewRequest("GET", url, nil)
		resp, err := f.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := readAllLimited(resp.Body)
		resp.Body.Close()

		var sRes shodanResponse
		if json.Unmarshal(body, &sRes) != nil {
			continue
		}
		for _, m := range sRes.Matches {
			u := fmt.Sprintf("http://%s/", m.IPStr)
			if m.Port != 80 {
				u = fmt.Sprintf("http://%s:%d/", m.IPStr, m.Port)
			}
			all = append(all, Result{URL: u, Source: "shodan"})
		}
		time.Sleep(1 * time.Second)
	}
	return all, nil
}

type shodanResponse struct {
	Matches []struct {
		IPStr string `json:"ip_str"`
		Port  int    `json:"port"`
	} `json:"matches"`
}

type aggregator struct {
	name string
	url  string
}

var aggregators = []aggregator{
	{name: "opendirsearch", url: "https://opendirsearch.abifog.com/"},
	{name: "odfinder", url: "https://odfinder.github.io/"},
}

func (f *Finder) ScrapeAggregators() ([]Result, error) {
	var all []Result
	for _, agg := range aggregators {
		req, _ := http.NewRequest("GET", agg.url, nil)
		req.Header.Set("User-Agent", f.randomUA())

		resp, err := f.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := readAllLimited(resp.Body)
		resp.Body.Close()

		re := regexp.MustCompile(`(https?://[^\s"<>]+)`)
		matches := re.FindAllString(string(body), -1)
		seen := map[string]bool{}
		for _, rawURL := range matches {
			cleanURL := strings.TrimRight(rawURL, "),./;'\"")
			if !strings.HasPrefix(cleanURL, "http") {
				continue
			}
			if seen[cleanURL] {
				continue
			}
			seen[cleanURL] = true
			all = append(all, Result{
				URL:    cleanURL,
				Source: agg.name,
			})
		}
		time.Sleep(1 * time.Second)
	}
	return all, nil
}

func (f *Finder) IsOpenDirectory(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}

	req, _ := http.NewRequest("GET", parsed.String(), nil)
	req.Header.Set("User-Agent", f.randomUA())
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := f.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, _ := readAllLimited(resp.Body)
	bodyStr := string(body)

	indicators := []string{
		"Index of", "Parent Directory", "Directory Listing",
		"last modified", "parent directory",
	}
	for _, ind := range indicators {
		if strings.Contains(bodyStr, ind) {
			return true
		}
	}
	return false
}

func (f *Finder) SearchCommonPaths(baseURL string) []string {
	paths := []string{
		"./", "/", "/files/", "/downloads/", "/shared/",
		"/public/", "/media/", "/movies/", "/music/", "/books/",
		"/backup/", "/storage/", "/data/", "/upload/", "/d/",
	}
	var found []string
	for _, p := range paths {
		fullURL := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(p, "/")
		if f.IsOpenDirectory(fullURL) {
			found = append(found, fullURL)
		}
	}
	return found
}

func (f *Finder) QuickProfile(rawURL string) (map[models.FileCategory]int, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}

	req, err := http.NewRequest("GET", parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", f.randomUA())
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := readAllLimited(resp.Body)
	if err != nil {
		return nil, err
	}

	_, links, err := parser.ParseDirectoryListing(parsed.String(), string(body))
	if err != nil {
		return nil, err
	}

	counts := make(map[models.FileCategory]int)
	for _, link := range links {
		if link.IsDir {
			continue
		}
		cat := classify.FileEntry(link.Name, link.Size)
		counts[cat]++
	}
	return counts, nil
}

func (f *Finder) CheckDensity(rawURL string, target models.FileCategory, threshold float64) bool {
	counts, err := f.QuickProfile(rawURL)
	if err != nil {
		return false
	}

	var total, targetCount int
	for cat, count := range counts {
		total += count
		if cat == target {
			targetCount = count
		}
	}

	if total == 0 {
		return false
	}

	return float64(targetCount)/float64(total) >= threshold
}

func (f *Finder) DiscoverByType(category string) ([]Result, error) {
	dorks, ok := TypeDorks[category]
	if !ok {
		return nil, fmt.Errorf("unsupported type: %s (supported: audio, video, image, document, archive, code)", category)
	}

	var all []Result
	seen := map[string]bool{}

	addUnique := func(r Result) {
		u := strings.TrimRight(r.URL, "/")
		if !seen[u] {
			seen[u] = true
			all = append(all, r)
		}
	}

	bingLinkRe := regexp.MustCompile(`href="(/ck/[^"]*\bu=([A-Za-z0-9+/=_-]+)[^"]*)"`)
	ddgRe := regexp.MustCompile(`<a[^>]+class="result__a"[^>]+href="(https?://[^"]+)"`)

	if f.googleKey != "" && f.googleCX != "" {
		for _, dork := range dorks {
			url := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s",
				f.googleKey, f.googleCX, url.QueryEscape(dork))
			req, _ := http.NewRequest("GET", url, nil)
			resp, err := f.client.Do(req)
			if err != nil {
				continue
			}
			body, _ := readAllLimited(resp.Body)
			resp.Body.Close()
			var gRes googleAPIResponse
			if json.Unmarshal(body, &gRes) == nil {
				for _, item := range gRes.Items {
					addUnique(Result{URL: item.Link, Source: "google", Title: item.Title})
				}
			}
			time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
		}
	} else {
		for _, dork := range dorks {
			results, _ := f.searchGoogleScrape(dork, 1)
			for _, r := range results {
				addUnique(r)
			}
			time.Sleep(time.Duration(2+rand.Intn(3)) * time.Second)
		}
	}

	bingQueries := []string{
		fmt.Sprintf(`"Index of /" %s`, category),
		fmt.Sprintf(`intitle:"index of" %s`, category),
		fmt.Sprintf(`"Index of /" %ss -inurl:(php|asp|jsp)`, category),
	}
	if f.bingKey != "" {
		for _, q := range bingQueries {
			url := fmt.Sprintf("https://api.bing.microsoft.com/v7.0/search?q=%s&count=50",
				url.QueryEscape(q))
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("Ocp-Apim-Subscription-Key", f.bingKey)
			resp, err := f.client.Do(req)
			if err != nil {
				continue
			}
			body, _ := readAllLimited(resp.Body)
			resp.Body.Close()
			var bRes bingAPIResponse
			if json.Unmarshal(body, &bRes) == nil {
				for _, w := range bRes.WebPages.Value {
					addUnique(Result{URL: w.URL, Source: "bing", Title: w.Name})
				}
			}
			time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
		}
	} else {
		for _, q := range bingQueries {
			searchURL := fmt.Sprintf("https://www.bing.com/search?q=%s", url.QueryEscape(q))
			req, _ := http.NewRequest("GET", searchURL, nil)
			req.Header.Set("User-Agent", f.randomUA())
			req.Header.Set("Accept", "text/html,application/xhtml+xml")

			resp, err := f.client.Do(req)
			if err != nil {
				continue
			}
			body, _ := readAllLimited(resp.Body)
			resp.Body.Close()

			matches := bingLinkRe.FindAllStringSubmatch(string(body), -1)
			for _, m := range matches {
				if len(m) < 3 {
					continue
				}
				decoded, err := base64.StdEncoding.DecodeString(m[2])
				if err != nil {
					decoded, err = base64.RawStdEncoding.DecodeString(m[2])
					if err != nil {
						continue
					}
				}
				decodedURL := string(decoded)
				if !strings.HasPrefix(decodedURL, "http") {
					continue
				}
				addUnique(Result{URL: decodedURL, Source: "bing"})
			}
			time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
		}
	}

	ddgQueries := []string{
		fmt.Sprintf(`"Index of /" %s`, category),
		fmt.Sprintf(`intitle:"index of" %s`, category),
		fmt.Sprintf(`"Index of /" %ss`, category),
	}
	for _, q := range ddgQueries {
		searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(q))
		req, _ := http.NewRequest("GET", searchURL, nil)
		req.Header.Set("User-Agent", f.randomUA())
		req.Header.Set("Accept", "text/html,application/xhtml+xml")

		resp, err := f.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := readAllLimited(resp.Body)
		resp.Body.Close()

		matches := ddgRe.FindAllStringSubmatch(string(body), -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			decoded, err := url.QueryUnescape(m[1])
			if err != nil {
				continue
			}
			addUnique(Result{URL: decoded, Source: "duckduckgo"})
		}
		time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
	}

	searxngQueries := []string{
		fmt.Sprintf(`intitle:"index of" %s`, category),
		fmt.Sprintf(`"Index of /" %s`, category),
		fmt.Sprintf(`"Index of /" %ss`, category),
	}
	for _, q := range searxngQueries {
		url := fmt.Sprintf("%s/search?q=%s&format=json&language=en-US&categories=general",
			f.searxngURL, url.QueryEscape(q))
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", f.randomUA())
		req.Header.Set("Accept", "application/json")
		resp, err := f.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := readAllLimited(resp.Body)
		resp.Body.Close()
		var sRes searxngResponse
		if json.Unmarshal(body, &sRes) == nil {
			for _, r := range sRes.Results {
				addUnique(Result{URL: r.URL, Source: "searxng", Title: r.Title})
			}
		}
		time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
	}

	if category == "audio" {
		results, _ := f.SearchSoundClick("")
		for _, r := range results {
			addUnique(r)
		}
	}

	targetCat := models.FileCategory(category)
	var verified []Result
	for _, r := range all {
		if !f.IsOpenDirectory(r.URL) {
			continue
		}
		if f.CheckDensity(r.URL, targetCat, 0.15) {
			r.Verified = true
			verified = append(verified, r)
		}
	}

	return verified, nil
}

func (f *Finder) SearchFTP() ([]Result, error) {
	var all []Result
	seen := map[string]bool{}

	addFTP := func(r Result) {
		u := strings.TrimRight(r.URL, "/")
		if !seen[u] {
			seen[u] = true
			all = append(all, r)
		}
	}

	for _, dork := range FTPDorks {
		results, _ := f.searchGoogleScrape(dork, 1)
		for _, r := range results {
			if strings.HasPrefix(r.URL, "ftp://") || strings.HasPrefix(r.URL, "ftps://") {
				addFTP(r)
			}
		}
		time.Sleep(time.Duration(2+rand.Intn(3)) * time.Second)
	}

	for _, dork := range FTPDorks {
		searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(dork))
		req, _ := http.NewRequest("GET", searchURL, nil)
		req.Header.Set("User-Agent", f.randomUA())
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		resp, err := f.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := readAllLimited(resp.Body)
		resp.Body.Close()
		ddgRe := regexp.MustCompile(`<a[^>]+class="result__a"[^>]+href="(https?://[^"]+)"`)
		matches := ddgRe.FindAllStringSubmatch(string(body), -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			decoded, _ := url.QueryUnescape(m[1])
			if strings.HasPrefix(decoded, "ftp://") || strings.HasPrefix(decoded, "ftps://") {
				addFTP(Result{URL: decoded, Source: "duckduckgo"})
			}
		}
		time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)
	}

	return all, nil
}

func (f *Finder) SearchSoundClick(keyword string) ([]Result, error) {
	client := soundclick.New()
	if keyword == "" {
		keyword = "mp3"
	}
	tracks, err := client.Search(keyword, 30)
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, t := range tracks {
		audioURL := t.AudioURL
		if audioURL == "" {
			info, err := client.GetTrackInfo(t.SongID)
			if err == nil && info.AudioURL != "" {
				audioURL = info.AudioURL
			}
			if audioURL == "" {
				u, err := client.GetAudioURL(t.SongID)
				if err == nil {
					audioURL = u
				}
			}
		}
		if audioURL != "" {
			results = append(results, Result{
				URL:    audioURL,
				Source: "soundclick",
				Title:  fmt.Sprintf("%s - %s", t.Artist, t.Title),
			})
		}
	}
	return results, nil
}

func (f *Finder) SaveResults(results []Result, path string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

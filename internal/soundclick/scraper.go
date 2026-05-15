package soundclick

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const maxResponseSize = 10 << 20

type Track struct {
	SongID   int    `json:"song_id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Genre    string `json:"genre,omitempty"`
	AudioURL string `json:"audio_url,omitempty"`
	PageURL  string `json:"page_url"`
}

type Client struct {
	http *http.Client
	ua   string
}

func New() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
		ua: randomUA(),
	}
}

func randomUA() string {
	agents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	}
	return agents[rand.Intn(len(agents))]
}

func (c *Client) Search(keyword string, limit int) ([]Track, error) {
	if limit <= 0 {
		limit = 50
	}

	searchURL := fmt.Sprintf("https://www.soundclick.com/search/index.cfm?q=%s&SearchType=music",
		url.QueryEscape(keyword))

	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}

	tracks := parseSearchResults(string(body), limit)
	return tracks, nil
}

func parseSearchResults(html string, limit int) []Track {
	var tracks []Track
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	doc.Find("div[id^=\"song_\"]").Each(func(_ int, s *goquery.Selection) {
		if len(tracks) >= limit {
			return
		}

		idStr, _ := s.Attr("id")
		idStr = strings.TrimPrefix(idStr, "song_")
		songID, _ := strconv.Atoi(idStr)
		if songID == 0 {
			return
		}

		title := strings.TrimSpace(s.Find(".songTitle").Text())
		if title == "" {
			title = strings.TrimSpace(s.Find("a[href*=\"/music/\"]").Text())
		}

		artist := strings.TrimSpace(s.Find(".artistName").Text())
		if artist == "" {
			artist = strings.TrimSpace(s.Find("a[href*=\"/artist/\"]").Text())
		}

		genre := strings.TrimSpace(s.Find(".genre").Text())
		if genre == "" {
			s.Find("a").Each(func(_ int, a *goquery.Selection) {
				href, _ := a.Attr("href")
				if strings.Contains(href, "/genre/") {
					genre = strings.TrimSpace(a.Text())
				}
			})
		}

		pageURL := fmt.Sprintf("https://www.soundclick.com/music/%s&songid=%d", url.PathEscape(title), songID)

		track := Track{
			SongID:  songID,
			Title:   title,
			Artist:  artist,
			Genre:   genre,
			PageURL: pageURL,
		}
		tracks = append(tracks, track)
	})

	if len(tracks) == 0 {
		tracks = parseSearchResultsFallback(html, limit)
	}

	return tracks
}

func parseSearchResultsFallback(html string, limit int) []Track {
	var tracks []Track
	re := regexp.MustCompile(`songid[=:](\d+)`)
	matches := re.FindAllStringSubmatch(html, -1)
	seen := make(map[int]bool)

	for _, m := range matches {
		if len(tracks) >= limit {
			break
		}
		if len(m) < 2 {
			continue
		}
		songID, _ := strconv.Atoi(m[1])
		if songID == 0 || seen[songID] {
			continue
		}
		seen[songID] = true
		tracks = append(tracks, Track{
			SongID:  songID,
			PageURL: fmt.Sprintf("https://www.soundclick.com/music?songid=%d", songID),
		})
	}
	return tracks
}

func (c *Client) GetTrackInfo(songID int) (*Track, error) {
	pageURL := fmt.Sprintf("https://www.soundclick.com/music?songid=%d", songID)

	req, _ := http.NewRequest("GET", pageURL, nil)
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Referer", "https://www.soundclick.com/")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("track page request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read track page: %w", err)
	}

	return parseTrackPage(songID, string(body)), nil
}

func parseTrackPage(songID int, html string) *Track {
	track := &Track{
		SongID:  songID,
		PageURL: fmt.Sprintf("https://www.soundclick.com/music?songid=%d", songID),
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return track
	}

	track.Title = strings.TrimSpace(doc.Find("title").First().Text())
	track.Title = strings.TrimSuffix(track.Title, " | SoundClick")

	track.Artist = strings.TrimSpace(doc.Find(".artist-name, .artistName").First().Text())

	doc.Find("a[href*=\"/genre/\"]").Each(func(_ int, a *goquery.Selection) {
		if track.Genre == "" {
			track.Genre = strings.TrimSpace(a.Text())
		}
	})

	re := regexp.MustCompile(`(https?://[^"'\s]+\.(mp3|wav|flac|ogg|m4a)[^"'\s]*)`)
	if m := re.FindString(html); m != "" {
		track.AudioURL = m
	}

	if track.AudioURL == "" {
		re2 := regexp.MustCompile(`(https?://[^"'\s]+\.(mp3|wav|flac|ogg|m4a))`)
		if m := re2.FindString(html); m != "" {
			track.AudioURL = m
		}
	}

	re3 := regexp.MustCompile(`https?://[^"'\s]*soundclick[^"'\s]*\.mp3[^"'\s]*`)
	if m := re3.FindString(html); m != "" {
		track.AudioURL = m
	}

	return track
}

func (c *Client) GetAudioURL(songID int) (string, error) {
	re := regexp.MustCompile(`https?://[^"'\s]*cdn[^"'\s]*soundclick[^"'\s]*/*\.mp3`)

	xmlURL := fmt.Sprintf("https://www.soundclick.com/util/xmlsong.cfm?songid=%d", songID)
	req, _ := http.NewRequest("GET", xmlURL, nil)
	req.Header.Set("User-Agent", c.ua)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("xml request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if m := re.FindString(string(body)); m != "" {
		return m, nil
	}

	return "", fmt.Errorf("no audio URL found for song %d", songID)
}

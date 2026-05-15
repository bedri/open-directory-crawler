package crawler

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/bedri/open-directory-crawler/internal/parser"
	"github.com/jlaffaye/ftp"
)

func listFTPDirectory(rawURL string) ([]parser.FileLink, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("ftp parse url: %w", err)
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		host = host + ":21"
	}

	user := u.User
	username := "anonymous"
	password := "anonymous@"
	if user != nil {
		u := user.Username()
		p, _ := user.Password()
		if u != "" {
			username = u
		}
		if p != "" {
			password = p
		}
	}

	path := u.Path
	if path == "" {
		path = "/"
	}

	isTLS := strings.HasPrefix(rawURL, "ftps://")

	var c *ftp.ServerConn
	if isTLS {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         u.Hostname(),
		}
		c, err = ftp.Dial(host, ftp.DialWithTimeout(30*time.Second), ftp.DialWithExplicitTLS(tlsCfg))
		if err != nil {
			c, err = ftp.Dial(host, ftp.DialWithTimeout(30*time.Second), ftp.DialWithTLS(tlsCfg))
		}
	} else {
		c, err = ftp.Dial(host, ftp.DialWithTimeout(30*time.Second))
	}
	if err != nil {
		return nil, err
	}
	defer c.Quit()

	if err := c.Login(username, password); err != nil {
		return nil, fmt.Errorf("ftp login: %w", err)
	}

	entries, err := c.List(path)
	if err != nil {
		return nil, fmt.Errorf("ftp list %s: %w", path, err)
	}

	var links []parser.FileLink
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		linkURL := rawURL
		if !strings.HasSuffix(linkURL, "/") {
			linkURL += "/"
		}
		linkURL += e.Name

		link := parser.FileLink{
			Name:  e.Name,
			URL:   linkURL,
			IsDir: e.Type == ftp.EntryTypeFolder,
			Size:  int64(e.Size),
		}
		if !e.Time.IsZero() {
			link.LastModified = e.Time
		}
		links = append(links, link)
	}

	return links, nil
}



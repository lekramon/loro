package sources

import (
	"context"
	"encoding/xml"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/lekramon/piratarr/internal/browser"
)

func test1337x(ctx context.Context, config Config) (Capabilities, error) {
	result := browser.Fetch(ctx, strings.TrimRight(config.URL, "/")+"/cat/Movies/1/", "search")
	if result.Error != "" {
		return Capabilities{}, failed(result.Error)
	}
	count := 20
	return Capabilities{Search: SearchCapability{SupportedParams: []string{}}, MovieSearch: SearchCapability{Available: true, SupportedParams: []string{"q", "year"}}, MaxLimit: &count, DefaultLimit: &count, Categories: []Category{{ID: 2000, Name: "Movies"}}}, nil
}

func search1337x(ctx context.Context, config Config, query url.Values, limit, offset int) (ReleasePage, error) {
	if limit != 20 || offset%20 != 0 {
		return ReleasePage{}, ErrValidation
	}
	title := query.Get("q")
	if query.Get("year") != "" {
		title += " " + query.Get("year")
	}
	address := strings.TrimRight(config.URL, "/") + "/category-search/" + url.PathEscape(title) + "/Movies/" + strconv.Itoa(offset/40+1) + "/"
	result := browser.Fetch(ctx, address, "search")
	if result.Error != "" {
		return ReleasePage{}, failed(result.Error)
	}
	return normalize1337x(config, result, limit, offset)
}

func normalize1337x(config Config, result browser.Result, limit, offset int) (ReleasePage, error) {
	feed := feedXML{XMLName: xml.Name{Local: "rss"}, Channels: []channelXML{{}}}
	for _, row := range result.Rows {
		if row.Position < offset%40 || row.Position >= offset%40+20 {
			continue
		}
		if !browser.ValidDetail(row.Path) || row.Title == "" {
			return ReleasePage{}, failed("source-invalid-response")
		}
		detail := strings.TrimRight(config.URL, "/") + row.Path
		item := releaseXML{Title: row.Title, GUID: detail, Link: "louro-1337x:" + detail}
		if size := nativeSize(row.Size); size != "" {
			item.Size = size
		}
		if numberEvidence([]string{row.Seeders}) != nil {
			item.Attributes = append(item.Attributes, attributeXML{XMLName: xml.Name{Space: torznabNamespace, Local: "attr"}, Name: "seeders", Value: row.Seeders})
		}
		feed.Channels[0].Items = append(feed.Channels[0].Items, item)
	}
	data, err := xml.Marshal(feed)
	if err != nil {
		return ReleasePage{}, failed("source-invalid-response")
	}
	page, err := parseReleases(data, config, limit, offset)
	if err != nil {
		return ReleasePage{}, err
	}
	page.HasMore = result.RawCount > offset%40+20 || result.HasMore
	page.NextOffset = nil
	if page.HasMore && offset+20 <= 10000 {
		next := offset + 20
		page.NextOffset = &next
	}
	return page, nil
}

var sizePattern = regexp.MustCompile(`(?i)^\s*([0-9]+(?:\.[0-9]+)?)\s*(B|KB|MB|GB|TB|KiB|MiB|GiB|TiB)\b`)

func nativeSize(text string) string {
	match := sizePattern.FindStringSubmatch(text)
	if len(match) != 3 {
		return ""
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return ""
	}
	unit := strings.ToUpper(strings.ReplaceAll(strings.ToLower(match[2]), "i", ""))
	power := map[string]int{"B": 0, "KB": 1, "MB": 2, "GB": 3, "TB": 4}[unit]
	for i := 0; i < power; i++ {
		value *= 1024
	}
	if value < 0 || value > 9007199254740991 {
		return ""
	}
	return strconv.FormatInt(int64(value), 10)
}

// Resolve1337x resolves only a selected, private same-origin details reference.
func Resolve1337x(ctx context.Context, sourceURL, reference string) (string, string) {
	base, err := url.Parse(sourceURL)
	if err != nil {
		return "", "unsupported-reference"
	}
	detail, err := url.Parse(strings.TrimPrefix(reference, "louro-1337x:"))
	if err != nil || detail.User != nil || detail.Scheme != base.Scheme || detail.Host != base.Host || (detail.Scheme != "http" && detail.Scheme != "https") || detail.RawQuery != "" || detail.Fragment != "" || !browser.ValidDetail(detail.Path) {
		return "", "unsupported-reference"
	}
	result := browser.Fetch(ctx, detail.String(), "detail")
	if result.Error != "" {
		return "", result.Error
	}
	if !strings.HasPrefix(result.Magnet, "magnet:") {
		return "", "missing-reference"
	}
	return result.Magnet, ""
}

func (s *Service) ResolveReference(ctx context.Context, sourceURL, reference string) (string, string) {
	return Resolve1337x(ctx, sourceURL, reference)
}

package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/lekramon/piratarr/internal/settings"
)

type attributeXML struct {
	XMLName xml.Name
	Name    string `xml:"name,attr"`
	Value   string `xml:"value,attr"`
}
type releaseXML struct {
	Title     string `xml:"title"`
	GUID      string `xml:"guid"`
	Link      string `xml:"link"`
	Date      string `xml:"pubDate"`
	Size      string `xml:"size"`
	Enclosure struct {
		URL    string `xml:"url,attr"`
		Length string `xml:"length,attr"`
	} `xml:"enclosure"`
	Attributes []attributeXML `xml:"attr"`
}
type feedXML struct {
	XMLName  xml.Name
	Code     int          `xml:"code,attr"`
	Channels []channelXML `xml:"channel"`
}

type channelXML struct {
	Response []struct {
		XMLName xml.Name
		Offset  string `xml:"offset,attr"`
		Total   string `xml:"total,attr"`
	} `xml:"response"`
	Items []releaseXML `xml:"item"`
}

const torznabNamespace = "http://torznab.com/schemas/2015/feed"
const newznabNamespace = "http://www.newznab.com/DTD/2010/feeds/attributes/"

func parseReleases(data []byte, config Config, limit, offset int) (ReleasePage, error) {
	invalid := failed("source-invalid-response")
	if err := validateXML(data); err != nil {
		return ReleasePage{}, err
	}
	var feed feedXML
	if xml.Unmarshal(data, &feed) != nil {
		return ReleasePage{}, invalid
	}
	if feed.XMLName.Local == "error" {
		if feed.Code == 100 || feed.Code == 101 || feed.Code == 102 {
			return ReleasePage{}, failed("source-authentication-failed")
		}
		return ReleasePage{}, failed("source-unavailable")
	}
	if feed.XMLName.Local != "rss" || len(feed.Channels) != 1 {
		return ReleasePage{}, invalid
	}
	channel := feed.Channels[0]
	if len(channel.Items) > limit {
		return ReleasePage{}, invalid
	}
	page := ReleasePage{Limit: limit, Offset: offset, Items: []Candidate{}}
	responses := 0
	for _, r := range channel.Response {
		if r.XMLName.Space != torznabNamespace && r.XMLName.Space != newznabNamespace {
			continue
		}
		responses++
		o, err := strconv.Atoi(r.Offset)
		if responses > 1 || err != nil || o != offset {
			return ReleasePage{}, invalid
		}
		if r.Total != "" {
			page.Total = numberEvidence([]string{r.Total})
			if page.Total == nil {
				return ReleasePage{}, invalid
			}
		}
	}
	rawCount := len(channel.Items)
	if page.Total != nil && rawCount > 0 && int64(offset+rawCount) > *page.Total {
		return ReleasePage{}, invalid
	}
	page.HasMore = rawCount == limit
	if page.Total != nil {
		page.HasMore = rawCount > 0 && int64(offset+rawCount) < *page.Total
	}
	if page.HasMore && offset+rawCount <= 10000 {
		next := offset + rawCount
		page.NextOffset = &next
	}
	seen := map[string]bool{}
	for _, item := range channel.Items {
		title := strings.TrimSpace(item.Title)
		if title == "" || utf8.RuneCountInString(title) > 500 {
			return ReleasePage{}, invalid
		}
		for _, r := range title {
			if unicode.IsControl(r) {
				return ReleasePage{}, invalid
			}
		}
		attributes := map[string][]string{}
		for _, attr := range item.Attributes {
			if attr.XMLName.Space == torznabNamespace || attr.XMLName.Space == newznabNamespace {
				attributes[strings.ToLower(attr.Name)] = append(attributes[strings.ToLower(attr.Name)], attr.Value)
			}
		}
		identity := strings.TrimSpace(item.GUID)
		if identity == "" {
			if guid := unique(attributes["guid"]); guid != nil {
				identity = strings.TrimSpace(*guid)
			}
		}
		if identity == "" {
			link := strings.TrimSpace(item.Enclosure.URL)
			if link == "" {
				link = strings.TrimSpace(item.Link)
			}
			if link == "" {
				return ReleasePage{}, invalid
			}
			identity = title + "\x00" + link
		}
		hash := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s", config.Revision, identity)))
		id := hex.EncodeToString(hash[:])
		if seen[id] {
			continue
		}
		seen[id] = true
		if config.APIKey != "" {
			title = strings.ReplaceAll(title, config.APIKey, "[redacted]")
		}
		sizes := append([]string{}, attributes["size"]...)
		if item.Size != "" {
			sizes = append(sizes, item.Size)
		}
		if item.Enclosure.Length != "" {
			sizes = append(sizes, item.Enclosure.Length)
		}
		candidate := Candidate{ID: id, Title: title, SizeBytes: numberEvidence(sizes), Seeders: numberEvidence(attributes["seeders"]), AudioLanguages: audioLanguages(attributes["language"])}
		candidate.DownloadReference = strings.TrimSpace(item.Enclosure.URL)
		if candidate.DownloadReference == "" {
			candidate.DownloadReference = strings.TrimSpace(item.Link)
		}
		if magnets := attributes["magneturl"]; len(magnets) > 0 {
			if magnet := unique(magnets); magnet != nil {
				candidate.DownloadReference = *magnet
			} else {
				candidate.ReferenceIssue = "ambiguous-reference"
			}
		}
		if len(candidate.DownloadReference) > 8192 {
			candidate.DownloadReference = ""
			candidate.ReferenceIssue = "unsupported-reference"
		}
		if candidate.DownloadReference == "" && candidate.ReferenceIssue == "" {
			candidate.ReferenceIssue = "missing-reference"
		}
		if claims := attributes["infohash"]; len(claims) > 0 {
			if claim := unique(claims); claim != nil {
				candidate.HashClaim = *claim
				if len(candidate.HashClaim) > 128 {
					candidate.HashClaim = ""
					candidate.ReferenceIssue = "identity-mismatch"
				}
			} else {
				candidate.ReferenceIssue = "identity-mismatch"
			}
		}
		if date, err := time.Parse(time.RFC1123Z, item.Date); err == nil {
			at := date.UTC().Format(time.RFC3339)
			candidate.PublishedAt = &at
		} else if date, err = time.Parse(time.RFC1123, item.Date); err == nil {
			at := date.UTC().Format(time.RFC3339)
			candidate.PublishedAt = &at
		}
		candidate.Resolution = resolutionEvidence(title, attributes["resolution"])
		candidate.Format = formatEvidence(title)
		candidate.Container = uniqueToken(containerPattern, title)
		page.Items = append(page.Items, candidate)
	}
	return page, nil
}

// Empty, malformed or conflicting evidence stays unknown rather than picking a value.
func numberEvidence(values []string) *int64 {
	var found *int64
	for _, value := range values {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n < 0 || n > 9007199254740991 || found != nil && *found != n {
			return nil
		}
		found = &n
	}
	return found
}

var resolutionPattern = regexp.MustCompile(`(?i)\b(480p|576p|720p|1080p|2160p)\b`)
var containerPattern = regexp.MustCompile(`(?i)\b(mkv|mp4|avi|mov|m4v|ts)\b`)
var formatPattern = regexp.MustCompile(`(?i)\b(remux|blu[ .-]?ray|web[ .-]?dl|web[ .-]?rip|hdtv|dvd)\b`)

func unique(values []string) *string {
	slices.Sort(values)
	values = slices.Compact(values)
	if len(values) != 1 {
		return nil
	}
	return &values[0]
}
func uniqueToken(pattern *regexp.Regexp, title string) *string {
	values := []string{}
	for _, match := range pattern.FindAllStringSubmatch(title, -1) {
		values = append(values, strings.ToLower(match[1]))
	}
	return unique(values)
}
func resolutionEvidence(title string, attributes []string) *string {
	values := []string{}
	for _, match := range resolutionPattern.FindAllStringSubmatch(title, -1) {
		values = append(values, strings.ToLower(match[1]))
	}
	for _, attr := range attributes {
		value := strings.ToLower(strings.TrimSpace(attr))
		if resolutionPattern.MatchString(value) && resolutionPattern.FindString(value) == value {
			values = append(values, value)
		}
	}
	return unique(values)
}
func formatEvidence(title string) *string {
	values := []string{}
	for _, match := range formatPattern.FindAllStringSubmatch(title, -1) {
		value := strings.NewReplacer(" ", "", ".", "", "-", "").Replace(strings.ToLower(match[1]))
		if value == "webdl" {
			value = "web-dl"
		}
		values = append(values, value)
	}
	if slices.Contains(values, "remux") {
		values = slices.DeleteFunc(values, func(s string) bool { return s == "bluray" })
	}
	return unique(values)
}
func audioLanguages(values []string) []string {
	result := []string{}
	names := map[string]string{}
	for _, language := range settings.Languages() {
		names[strings.ToLower(language.Name)] = language.Code
		names[language.Code] = language.Code
	}
	names["pt-br"] = "pt"
	names["brazilian portuguese"] = "pt"
	for _, value := range values {
		for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '/' }) {
			code := names[strings.ToLower(strings.TrimSpace(part))]
			if code == "" {
				return []string{}
			}
			result = append(result, code)
		}
	}
	slices.Sort(result)
	return slices.Compact(result)
}

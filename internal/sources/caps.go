package sources

import (
	"bytes"
	"encoding/xml"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type searchXML struct {
	Available string `xml:"available,attr"`
	Params    string `xml:"supportedParams,attr"`
}
type categoryXML struct {
	ID       int           `xml:"id,attr"`
	Name     string        `xml:"name,attr"`
	Children []categoryXML `xml:"subcat"`
}
type capsXML struct {
	XMLName   xml.Name
	Code      int `xml:"code,attr"`
	Searching *struct {
		Search searchXML `xml:"search"`
		Movie  searchXML `xml:"movie-search"`
	} `xml:"searching"`
	Limits struct {
		Max     string `xml:"max,attr"`
		Default string `xml:"default,attr"`
	} `xml:"limits"`
	Categories []categoryXML `xml:"categories>category"`
}

func parseCapabilities(data []byte, key string) (Capabilities, error) {
	invalid := failed("source-invalid-response")
	decoder := xml.NewDecoder(bytes.NewReader(data))
	depth, tokens, roots := 0, 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Capabilities{}, invalid
		}
		tokens++
		if tokens > 20000 {
			return Capabilities{}, invalid
		}
		switch value := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
			}
			depth++
			if depth > 32 || len(value.Attr) > 64 {
				return Capabilities{}, invalid
			}
		case xml.EndElement:
			depth--
		case xml.Directive:
			return Capabilities{}, invalid
		case xml.ProcInst:
			if value.Target != "xml" {
				return Capabilities{}, invalid
			}
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(value)) != "" {
				return Capabilities{}, invalid
			}
		}
	}
	if roots != 1 || depth != 0 {
		return Capabilities{}, invalid
	}
	var dto capsXML
	if xml.Unmarshal(data, &dto) != nil {
		return Capabilities{}, invalid
	}
	if dto.XMLName.Local == "error" {
		if dto.Code == 100 || dto.Code == 101 || dto.Code == 102 {
			return Capabilities{}, failed("source-authentication-failed")
		}
		return Capabilities{}, failed("source-unavailable")
	}
	if dto.XMLName.Local != "caps" || dto.Searching == nil {
		return Capabilities{}, invalid
	}
	normalize := func(input searchXML) (SearchCapability, bool) {
		result := SearchCapability{Available: strings.EqualFold(input.Available, "yes"), SupportedParams: []string{}}
		if len(input.Params) > 1024 {
			return result, false
		}
		if input.Available != "" && !strings.EqualFold(input.Available, "yes") && !strings.EqualFold(input.Available, "no") {
			return result, false
		}
		known := map[string]bool{"q": true, "imdbid": true, "tmdbid": true, "year": true, "genre": true, "traktid": true, "doubanid": true, "limit": true, "offset": true}
		seen := map[string]bool{}
		for _, part := range strings.Split(input.Params, ",") {
			value := strings.ToLower(strings.TrimSpace(part))
			if known[value] && !seen[value] {
				result.SupportedParams = append(result.SupportedParams, value)
				seen[value] = true
			}
		}
		sort.Strings(result.SupportedParams)
		return result, true
	}
	result := Capabilities{Categories: []Category{}}
	var ok bool
	result.Search, ok = normalize(dto.Searching.Search)
	if !ok {
		return result, invalid
	}
	result.MovieSearch, ok = normalize(dto.Searching.Movie)
	if !ok {
		return result, invalid
	}
	number := func(value string) (*int, bool) {
		if value == "" {
			return nil, true
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 1000000 {
			return nil, false
		}
		return &n, true
	}
	result.MaxLimit, ok = number(dto.Limits.Max)
	if !ok {
		return result, invalid
	}
	result.DefaultLimit, ok = number(dto.Limits.Default)
	if !ok || result.MaxLimit != nil && result.DefaultLimit != nil && *result.DefaultLimit > *result.MaxLimit {
		return result, invalid
	}
	seen := map[int]bool{}
	var flatten func([]categoryXML) bool
	flatten = func(values []categoryXML) bool {
		for _, value := range values {
			if value.ID < 1 || value.ID > 1000000 || utf8.RuneCountInString(value.Name) > 100 {
				return false
			}
			if !seen[value.ID] {
				name := strings.TrimSpace(value.Name)
				if key != "" {
					name = strings.ReplaceAll(name, key, "[redacted]")
				}
				result.Categories = append(result.Categories, Category{ID: value.ID, Name: name})
				seen[value.ID] = true
				if len(result.Categories) > 256 {
					return false
				}
			}
			if !flatten(value.Children) {
				return false
			}
		}
		return true
	}
	if !flatten(dto.Categories) {
		return result, invalid
	}
	return result, nil
}

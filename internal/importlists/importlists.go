// Package importlists previews and records user-exported catalog lists.
package importlists

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lekramon/piratarr/internal/catalog"
)

var ErrInvalid = errors.New("import-list-invalid")
var ErrNotFound = errors.New("import-list-not-found")
var ErrChanged = errors.New("import-list-changed")
var ErrUnavailable = errors.New("import-list-unavailable")

const MaxRows = 200
const MaxCSVBytes = 60000

type PreviewInput struct {
	Source   string `json:"source"`
	ListKey  string `json:"listKey"`
	Name     string `json:"name"`
	CSV      string `json:"csv"`
	Language string `json:"language,omitempty"`
}
type Row struct {
	RowNumber     int    `json:"rowNumber"`
	SourceItemKey string `json:"sourceItemKey"`
	Title         string `json:"title"`
	Year          *int   `json:"year,omitempty"`
	TMDBID        *int64 `json:"tmdbId,omitempty"`
	Status        string `json:"status"`
	ErrorCode     string `json:"errorCode,omitempty"`
}
type Preview struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	ListKey   string `json:"listKey"`
	Name      string `json:"name"`
	Language  string `json:"language"`
	InputHash string `json:"-"`
	State     string `json:"state"`
	Revision  int    `json:"revision"`
	CreatedAt string `json:"createdAt"`
	Rows      []Row  `json:"rows"`
}
type CommitInput struct {
	Revision int `json:"revision"`
}
type Link struct {
	SourceItemKey string
	Title         string
	Year          *int
	TMDBID        int64
	MovieID       int64
}
type CommitResult struct {
	PreviewID  string `json:"previewId"`
	ListID     int64  `json:"listId"`
	Revision   int    `json:"revision"`
	Linked     int    `json:"linked"`
	Created    int    `json:"created"`
	Duplicates int    `json:"duplicates"`
	State      string `json:"state"`
}
type ListItem struct {
	ID        int64  `json:"id"`
	Source    string `json:"source"`
	ListKey   string `json:"listKey"`
	Name      string `json:"name"`
	Revision  int    `json:"revision"`
	Items     int    `json:"items"`
	UpdatedAt string `json:"updatedAt"`
}

type Repository interface {
	SaveImportListPreview(context.Context, Preview) error
	ImportListPreview(context.Context, string) (Preview, error)
	CommitImportListPreview(context.Context, Preview, []Link, int, int) (CommitResult, error)
	ListImportLists(context.Context) ([]ListItem, error)
}

type Service struct {
	Repo    Repository
	Catalog *catalog.Service
	Now     func() time.Time
}

var listKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func (s *Service) Preview(ctx context.Context, input PreviewInput) (Preview, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Language == "" {
		input.Language = "en-US"
	}
	if (input.Source != "letterboxd-csv" && input.Source != "imdb-csv") || !listKeyPattern.MatchString(input.ListKey) || input.Name == "" || utf8.RuneCountInString(input.Name) > 200 || len(input.CSV) == 0 || len(input.CSV) > MaxCSVBytes || s.Catalog == nil {
		return Preview{}, ErrInvalid
	}
	if _, ok := catalog.Language(input.Language); !ok {
		return Preview{}, ErrInvalid
	}
	rows, err := parse(input.Source, input.CSV)
	if err != nil {
		return Preview{}, err
	}
	for index := range rows {
		row := &rows[index]
		if row.Status == "invalid" {
			continue
		}
		result, searchErr := s.Catalog.Search(ctx, row.Title, value(row.Year), 1, input.Language)
		if searchErr != nil {
			return Preview{}, searchErr
		}
		matches := exactMatches(result.Results, row.Title, row.Year)
		switch len(matches) {
		case 0:
			row.Status = "missing"
		case 1:
			id := matches[0].TMDBID
			row.TMDBID = &id
			row.Status = "addition"
			if _, lookupErr := s.Catalog.Repo.MovieByTMDB(ctx, id); lookupErr == nil {
				row.Status = "duplicate"
			} else if !errors.Is(lookupErr, catalog.ErrNotFound) {
				return Preview{}, lookupErr
			}
		default:
			row.Status = "ambiguous"
		}
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	preview := Preview{ID: identifier(), Source: input.Source, ListKey: input.ListKey, Name: input.Name, Language: input.Language, InputHash: hash(input), State: "preview", Revision: 1, CreatedAt: now().UTC().Format(time.RFC3339Nano), Rows: rows}
	if preview.ID == "" {
		return Preview{}, ErrUnavailable
	}
	if err = s.Repo.SaveImportListPreview(ctx, preview); err != nil {
		return Preview{}, err
	}
	return preview, nil
}

func (s *Service) Commit(ctx context.Context, id string, revision int) (CommitResult, error) {
	if len(id) != 32 || revision < 1 || s.Catalog == nil {
		return CommitResult{}, ErrInvalid
	}
	preview, err := s.Repo.ImportListPreview(ctx, id)
	if err != nil {
		return CommitResult{}, err
	}
	if preview.State == "committed" {
		if revision != preview.Revision && revision+1 != preview.Revision {
			return CommitResult{}, ErrChanged
		}
		return s.Repo.CommitImportListPreview(ctx, preview, nil, 0, 0)
	}
	if preview.Revision != revision {
		return CommitResult{}, ErrChanged
	}
	links := make([]Link, 0, len(preview.Rows))
	created, duplicates := 0, 0
	for _, row := range preview.Rows {
		if row.TMDBID == nil || (row.Status != "addition" && row.Status != "duplicate") {
			continue
		}
		movie, added, registerErr := s.Catalog.Register(ctx, *row.TMDBID, preview.Language)
		if registerErr != nil {
			return CommitResult{}, registerErr
		}
		if added {
			created++
		} else {
			duplicates++
		}
		links = append(links, Link{SourceItemKey: row.SourceItemKey, Title: row.Title, Year: row.Year, TMDBID: *row.TMDBID, MovieID: movie.ID})
	}
	return s.Repo.CommitImportListPreview(ctx, preview, links, created, duplicates)
}

func parse(source, content string) ([]Row, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(content, "\ufeff")))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, ErrInvalid
	}
	columns := map[string]int{}
	for index, name := range header {
		columns[strings.ToLower(strings.TrimSpace(name))] = index
	}
	required := []string{"name", "year"}
	if source == "imdb-csv" {
		required = []string{"const", "title", "year"}
	}
	for _, name := range required {
		if _, ok := columns[name]; !ok {
			return nil, ErrInvalid
		}
	}
	rows, seen := []Row{}, map[string]bool{}
	for number := 2; ; number++ {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil || len(rows) >= MaxRows {
			return nil, ErrInvalid
		}
		titleColumn := "name"
		keyColumn := "letterboxd uri"
		if source == "imdb-csv" {
			titleColumn, keyColumn = "title", "const"
		}
		title := field(record, columns[titleColumn])
		yearText := field(record, columns["year"])
		year, yearErr := strconv.Atoi(yearText)
		key := ""
		if keyIndex, exists := columns[keyColumn]; exists {
			key = field(record, keyIndex)
		}
		if key == "" {
			key = strings.ToLower(strings.Join(strings.Fields(title), " ")) + "|" + yearText
		}
		row := Row{RowNumber: number, SourceItemKey: key, Title: title, Status: "pending"}
		if title == "" || utf8.RuneCountInString(title) > 300 || yearErr != nil || year < 1870 || year > 2100 || key == "" || len(key) > 512 {
			row.Status, row.ErrorCode = "invalid", "invalid-row"
		} else {
			row.Year = &year
		}
		if source == "imdb-csv" {
			if typeIndex, exists := columns["title type"]; exists {
				titleType := strings.ToLower(field(record, typeIndex))
				if titleType != "" && titleType != "movie" && titleType != "tvmovie" {
					row.Status, row.ErrorCode = "invalid", "unsupported-title-type"
				}
			}
		}
		if seen[key] {
			row.Status, row.ErrorCode = "invalid", "duplicate-row"
		}
		seen[key] = true
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, ErrInvalid
	}
	return rows, nil
}

func field(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}
func value(year *int) int {
	if year == nil {
		return 0
	}
	return *year
}
func exactMatches(values []catalog.Metadata, title string, year *int) []catalog.Metadata {
	wanted := strings.ToLower(strings.Join(strings.Fields(title), " "))
	result := []catalog.Metadata{}
	for _, candidate := range values {
		candidateTitle := strings.ToLower(strings.Join(strings.Fields(candidate.Title), " "))
		original := strings.ToLower(strings.Join(strings.Fields(candidate.OriginalTitle), " "))
		if candidateTitle != wanted && original != wanted {
			continue
		}
		if year != nil && (candidate.Year == nil || *candidate.Year != *year) {
			continue
		}
		result = append(result, candidate)
	}
	return result
}
func hash(input PreviewInput) string {
	sum := sha256.Sum256([]byte(input.Source + "\x00" + input.ListKey + "\x00" + input.Name + "\x00" + input.Language + "\x00" + input.CSV))
	return hex.EncodeToString(sum[:])
}
func identifier() string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return ""
	}
	return hex.EncodeToString(data)
}

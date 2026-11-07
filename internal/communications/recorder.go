// Package communications records bounded, credential-free integration evidence.
package communications

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const Capacity = 2000
const FileLimit = 2 << 20

var integrations = map[string]bool{"api": true, "metadata": true, "source": true, "torrent": true, "download-client": true, "browser": true, "jobs": true, "system": true}
var phases = map[string]bool{"request.started": true, "response.received": true, "request.failed": true, "operation.started": true, "operation.completed": true, "operation.failed": true, "browser.started": true, "browser.navigation": true, "browser.response": true, "browser.challenge.detected": true, "browser.widget.found": true, "browser.interaction": true, "browser.challenge.cleared": true, "browser.parsed": true, "browser.stopped": true, "browser.failed": true, "storage.degraded": true}
var labelPattern = regexp.MustCompile(`^[a-z][a-z0-9.-]{0,79}$`)
var codePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var requestPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var eventPattern = regexp.MustCompile(`^[a-f0-9]{16}-[0-9]+$`)
var sequencePattern = regexp.MustCompile(`^[0-9]+$`)
var sessionPattern = regexp.MustCompile(`^[a-f0-9]{16}$`)

type Event struct {
	ID          string  `json:"id"`
	Timestamp   string  `json:"timestamp"`
	RequestID   string  `json:"requestId"`
	Integration string  `json:"integration"`
	Operation   string  `json:"operation"`
	Phase       string  `json:"phase"`
	Level       string  `json:"level"`
	Status      *int    `json:"status"`
	ElapsedMS   *int64  `json:"elapsedMs"`
	RetryAfter  *int    `json:"retryAfterSeconds"`
	Code        *string `json:"code"`
	Count       *int64  `json:"count"`
}
type entry struct {
	event    Event
	sequence uint64
}
type Page struct {
	SessionID        string  `json:"sessionId"`
	Entries          []Event `json:"entries"`
	Cursor           string  `json:"cursor"`
	Reset            bool    `json:"reset"`
	Truncated        bool    `json:"truncated"`
	StorageAvailable bool    `json:"storageAvailable"`
}
type Recorder struct {
	mu        sync.Mutex
	session   string
	sequence  uint64
	items     []entry
	directory string
	available bool
	day       string
	fileLimit int64
}

func token(size int) string {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		panic("random identity unavailable")
	}
	return hex.EncodeToString(b)
}
func ValidIntegration(s string) bool { return integrations[s] }
func ValidRequestID(s string) bool   { return requestPattern.MatchString(s) }
func ValidPhase(s string) bool       { return phases[s] }
func valid(e Event) bool {
	if !integrations[e.Integration] || !phases[e.Phase] || !labelPattern.MatchString(e.Operation) || !ValidRequestID(e.RequestID) || len(e.ID) > 64 {
		return false
	}
	if e.Level != "info" && e.Level != "warning" && e.Level != "error" {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, e.Timestamp); err != nil {
		return false
	}
	return (e.Code == nil || codePattern.MatchString(*e.Code)) && (e.Status == nil || *e.Status >= 100 && *e.Status <= 599) && (e.ElapsedMS == nil || *e.ElapsedMS >= 0 && *e.ElapsedMS <= 9007199254740991) && (e.RetryAfter == nil || *e.RetryAfter >= 0 && *e.RetryAfter <= 3600) && (e.Count == nil || *e.Count >= 0 && *e.Count <= 9007199254740991)
}
func New(dataDirectory string) *Recorder {
	r := &Recorder{session: token(8), fileLimit: FileLimit, day: time.Now().UTC().Format("2006-01-02")}
	if dataDirectory == "" {
		return r
	}
	r.directory = filepath.Join(dataDirectory, "logs")
	if os.MkdirAll(r.directory, 0700) != nil {
		return r
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for index := 2; index >= 0; index-- {
		f, err := os.Open(r.path(index))
		if err != nil {
			continue
		}
		scan := bufio.NewScanner(io.LimitReader(f, FileLimit))
		scan.Buffer(make([]byte, 4096), 4096)
		for scan.Scan() {
			var e Event
			if json.Unmarshal(scan.Bytes(), &e) != nil || !eventPattern.MatchString(e.ID) || !valid(e) {
				continue
			}
			stamp, _ := time.Parse(time.RFC3339Nano, e.Timestamp)
			if stamp.Before(cutoff) {
				continue
			}
			r.sequence++
			r.items = append(r.items, entry{e, r.sequence})
			r.bound()
		}
		f.Close()
	}
	r.available = r.compact() == nil
	return r
}
func (r *Recorder) path(index int) string {
	return filepath.Join(r.directory, fmt.Sprintf("communications.%d.ndjson", index))
}
func (r *Recorder) bound() {
	if len(r.items) > Capacity {
		r.items = r.items[len(r.items)-Capacity:]
	}
}
func (r *Recorder) compact() error {
	if r.directory == "" {
		return os.ErrNotExist
	}
	temp := filepath.Join(r.directory, "communications.tmp")
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	kept := r.items[:0]
	for _, item := range r.items {
		stamp, _ := time.Parse(time.RFC3339Nano, item.event.Timestamp)
		if !stamp.Before(cutoff) {
			kept = append(kept, item)
		}
	}
	r.items = kept
	// Keep the compacted file within the same hard bound as every rotated file.
	data := make([][]byte, 0, len(r.items))
	total := 0
	for i := len(r.items) - 1; i >= 0; i-- {
		line, _ := json.Marshal(r.items[i].event)
		line = append(line, '\n')
		if int64(total+len(line)) > r.fileLimit {
			break
		}
		data = append(data, line)
		total += len(line)
	}
	for i := len(data) - 1; i >= 0; i-- {
		if _, err = f.Write(data[i]); err != nil {
			break
		}
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(temp, r.path(0)); err != nil {
		return err
	}
	for _, index := range []int{1, 2} {
		if err = os.Remove(r.path(index)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
func (r *Recorder) append(e Event) error {
	if r.directory == "" {
		return os.ErrNotExist
	}
	line, _ := json.Marshal(e)
	line = append(line, '\n')
	if info, err := os.Stat(r.path(0)); err == nil && info.Size()+int64(len(line)) > r.fileLimit {
		if err = os.Remove(r.path(2)); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err = os.Rename(r.path(1), r.path(2)); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err = os.Rename(r.path(0), r.path(1)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	f, err := os.OpenFile(r.path(0), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(line)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}
func (r *Recorder) Add(e Event) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if e.RequestID == "" {
		e.RequestID = token(16)
	}
	if e.Level == "" {
		e.Level = "info"
	}
	if !valid(e) {
		return
	}
	r.sequence++
	e.ID = fmt.Sprintf("%s-%d", r.session, r.sequence)
	r.items = append(r.items, entry{e, r.sequence})
	r.bound()
	day := time.Now().UTC().Format("2006-01-02")
	if day != r.day {
		r.day = day
		r.available = r.compact() == nil
	} else {
		r.available = r.append(e) == nil
	}
}
func ParseCursor(cursor string) (string, uint64, error) {
	session, number, ok := strings.Cut(cursor, ":")
	if !ok || len(cursor) > 40 || !sessionPattern.MatchString(session) || !sequencePattern.MatchString(number) {
		return "", 0, fmt.Errorf("invalid cursor")
	}
	sequence, err := strconv.ParseUint(number, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid cursor")
	}
	return session, sequence, nil
}
func (r *Recorder) Read(cursor string, limit int, integration, requestID string) Page {
	r.mu.Lock()
	defer r.mu.Unlock()
	page := Page{SessionID: r.session, Entries: []Event{}, StorageAvailable: r.available}
	after := uint64(0)
	hasAfter := cursor != ""
	if hasAfter {
		session, value, _ := ParseCursor(cursor)
		after = value
		if session != r.session || after > r.sequence {
			page.Reset = true
			hasAfter = false
			after = 0
		}
	}
	if hasAfter && len(r.items) > 0 && after+1 < r.items[0].sequence {
		page.Truncated = true
	}
	matches := []entry{}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for _, item := range r.items {
		stamp, _ := time.Parse(time.RFC3339Nano, item.event.Timestamp)
		if stamp.Before(cutoff) || hasAfter && item.sequence <= after || integration != "" && item.event.Integration != integration || requestID != "" && item.event.RequestID != requestID {
			continue
		}
		matches = append(matches, item)
	}
	if !hasAfter && len(matches) > limit {
		matches = matches[len(matches)-limit:]
	}
	cursorSequence := r.sequence
	if len(matches) > limit {
		matches = matches[:limit]
		cursorSequence = matches[len(matches)-1].sequence
	}
	for _, item := range matches {
		page.Entries = append(page.Entries, item.event)
	}
	page.Cursor = fmt.Sprintf("%s:%d", r.session, cursorSequence)
	return page
}

type contextKey int

const recorderKey contextKey = 0
const requestKey contextKey = 1

func WithRecorder(ctx context.Context, r *Recorder) context.Context {
	return context.WithValue(ctx, recorderKey, r)
}
func Correlate(ctx context.Context) context.Context {
	if RequestID(ctx) != "" {
		return ctx
	}
	return context.WithValue(ctx, requestKey, token(16))
}
func RequestID(ctx context.Context) string { value, _ := ctx.Value(requestKey).(string); return value }
func Emit(ctx context.Context, e Event) {
	r, _ := ctx.Value(recorderKey).(*Recorder)
	if r == nil {
		return
	}
	e.RequestID = RequestID(ctx)
	r.Add(e)
}
func Ptr[T any](v T) *T { return &v }

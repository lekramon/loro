package submissions

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/url"
	"strings"

	"github.com/lekramon/piratarr/internal/acquisitions"
	"github.com/lekramon/piratarr/internal/downloadclient"
)

func invalid() error     { return &downloadclient.ProviderError{Code: "download-client-invalid-response"} }
func tag(job Job) string { return "louro-acquisition-" + job.ID }
func observe(ctx context.Context, s *downloadclient.Session, j Job) (string, error) {
	body, err := s.Exchange(ctx, "GET", "torrents/info?"+url.Values{"hashes": {j.Hash}}.Encode(), "", nil, 65536)
	if err != nil {
		return "", err
	}
	var rows []struct {
		Hash     *string `json:"hash"`
		Category *string `json:"category"`
		Tags     *string `json:"tags"`
	}
	if json.Unmarshal(body, &rows) != nil || bytes.Equal(bytes.TrimSpace(body), []byte("null")) || len(rows) > 1 {
		return "", invalid()
	}
	if len(rows) == 0 {
		return "absent", nil
	}
	r := rows[0]
	if r.Hash == nil || r.Category == nil || r.Tags == nil || len(*r.Hash) != 40 || !strings.EqualFold(*r.Hash, j.Hash) {
		return "", invalid()
	}
	owned := false
	for _, v := range strings.Split(*r.Tags, ",") {
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "louro-acquisition-") && v != tag(j) {
			return "needs-review", nil
		}
		if v == tag(j) {
			owned = true
		}
	}
	if *r.Category != "louro" || !owned {
		return "needs-review", nil
	}
	return "succeeded", nil
}
func categories(ctx context.Context, s *downloadclient.Session) (bool, error) {
	data, err := s.Exchange(ctx, "GET", "torrents/categories", "", nil, 65536)
	if err != nil {
		return false, err
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(data, &values) != nil || values == nil {
		return false, invalid()
	}
	raw, ok := values["louro"]
	if !ok {
		return false, nil
	}
	var v struct {
		Name *string `json:"name"`
		Path *string `json:"savePath"`
	}
	if json.Unmarshal(raw, &v) != nil || v.Name == nil || *v.Name != "louro" || v.Path == nil {
		return false, invalid()
	}
	return true, nil
}
func ensureCategory(ctx context.Context, s *downloadclient.Session, path string) error {
	exists, err := categories(ctx, s)
	if err != nil || exists {
		return err
	}
	_, err = s.Exchange(ctx, "POST", "torrents/createCategory", "application/x-www-form-urlencoded", []byte(url.Values{"category": {"louro"}, "savePath": {path}}.Encode()), 4096)
	if err != nil {
		return err
	}
	exists, err = categories(ctx, s)
	if err != nil {
		return err
	}
	if !exists {
		return invalid()
	}
	return nil
}
func add(ctx context.Context, s *downloadclient.Session, j Job, r acquisitions.Record) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range map[string]string{"category": "louro", "tags": tag(j), "savepath": j.Directory, "autoTMM": "false", "skip_checking": "false"} {
		if err := writer.WriteField(key, value); err != nil {
			return invalid()
		}
	}
	if *r.Kind == "magnet" {
		if !strings.HasPrefix(r.Reference, "magnet:?") {
			return invalid()
		}
		if err := writer.WriteField("urls", r.Reference); err != nil {
			return invalid()
		}
	} else {
		part, err := writer.CreateFormFile("torrents", j.Hash+".torrent")
		if err != nil {
			return invalid()
		}
		if _, err = part.Write(r.Payload); err != nil {
			return invalid()
		}
	}
	if writer.Close() != nil {
		return invalid()
	}
	response, err := s.Exchange(ctx, "POST", "torrents/add", writer.FormDataContentType(), body.Bytes(), 4096)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(response)) != "Ok." {
		return invalid()
	}
	return nil
}

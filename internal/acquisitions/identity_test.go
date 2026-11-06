package acquisitions

import (
	"crypto/sha1"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"
)

func encode(value any) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%d:%s", len(v), v)
	case int:
		return fmt.Sprintf("i%de", v)
	case []any:
		result := "l"
		for _, item := range v {
			result += encode(item)
		}
		return result + "e"
	case map[string]any:
		keys := []string{}
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result := "d"
		for _, key := range keys {
			result += encode(key) + encode(v[key])
		}
		return result + "e"
	}
	panic("unsupported fixture value")
}
func validInfo() map[string]any {
	return map[string]any{"name": "movie.mkv", "length": 1, "piece length": 16384, "pieces": strings.Repeat("\xff", 20)}
}
func TestMagnetCanonicalIdentityAndUnsupportedHashes(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef01234567"
	bytes, _ := hex.DecodeString(hash)
	base32Hash := base32.StdEncoding.EncodeToString(bytes)
	for _, value := range []string{hash, strings.ToUpper(hash), base32Hash} {
		actual, code := magnetIdentity("magnet:?xt=urn:btih:" + value + "&tr=https%3A%2F%2Fprivate.invalid%2Fdummy-secret")
		if actual != hash || code != "" {
			t.Fatal(actual, code)
		}
	}
	for _, entry := range []struct{ uri, code string }{{"magnet:?xt=urn:btmh:1220abcdef", "unsupported-identity"}, {"magnet:?xt=urn:btih:" + hash + "&xt=urn:btmh:1220ab", "unsupported-identity"}, {"magnet:?xt=urn:btih:invalid", "invalid-magnet"}, {"magnet:?xt=urn:btih:" + hash + "&xt=urn:btih:" + strings.Repeat("0", 40), "identity-mismatch"}, {"magnet:?dn=Title", "unsupported-identity"}, {"magnet:?xt=%xx", "invalid-magnet"}} {
		_, code := magnetIdentity(entry.uri)
		if code != entry.code {
			t.Fatal(entry, code)
		}
	}
}
func TestTorrentUsesExactInfoSpanAndValidatesFileLayout(t *testing.T) {
	info := validInfo()
	raw := encode(info)
	expected := sha1.Sum([]byte(raw))
	data := encode(map[string]any{"announce": "http://private.invalid/dummy-passkey", "info": info})
	hash, code := torrentIdentity([]byte(data))
	if code != "" || hash != hex.EncodeToString(expected[:]) {
		t.Fatal(hash, code)
	}
	delete(info, "length")
	info["name"] = "Folder"
	info["files"] = []any{map[string]any{"length": 1, "path": []any{"folder", "movie.mkv"}}}
	if _, code = torrentIdentity([]byte(encode(map[string]any{"info": info}))); code != "" {
		t.Fatal("multi-file rejected", code)
	}
	for _, component := range []string{"..", ".", "/etc/passwd", "C:drive", "folder\\file", "", "bad\x00name"} {
		info["files"] = []any{map[string]any{"length": 1, "path": []any{component}}}
		if _, code = torrentIdentity([]byte(encode(map[string]any{"info": info}))); code != "torrent-invalid" {
			t.Fatal("unsafe path accepted", component, code)
		}
	}
}
func TestTorrentRejectsUnsafeAlternatePathsAndSymlinkMetadata(t *testing.T) {
	info := validInfo()
	info["name.utf-8"] = "../escape"
	if _, code := torrentIdentity([]byte(encode(map[string]any{"info": info}))); code != "torrent-invalid" {
		t.Fatal("unsafe alternate name accepted", code)
	}
	delete(info, "length")
	delete(info, "name.utf-8")
	for _, extra := range []map[string]any{{"symlink path": []any{"..", "outside"}}, {"attr": "l"}, {"path.utf-8": []any{"..", "outside"}}} {
		file := map[string]any{"length": 1, "path": []any{"movie.mkv"}}
		for key, value := range extra {
			file[key] = value
		}
		info["files"] = []any{file}
		if _, code := torrentIdentity([]byte(encode(map[string]any{"info": info}))); code != "torrent-invalid" {
			t.Fatal("unsafe alternate/link accepted", code)
		}
	}
}
func TestTorrentRejectsMalformedAmbiguousAndUnboundedMetadata(t *testing.T) {
	for _, data := range []string{"", "<html>private</html>", "deextra", "d4:infoi1ee", "d4:infoi01ee", "d4:infoi-0ee", "d4:info0:4:info0:e", "d4:zzzz0:4:aaaa0:e", "d04:info0:e", "d4:info" + strings.Repeat("l", 34) + "0:" + strings.Repeat("e", 34) + "e", "d4:infol" + strings.Repeat("0:", 100001) + "ee"} {
		if _, code := torrentIdentity([]byte(data)); code != "torrent-invalid" {
			t.Fatal("invalid bencoding accepted", code)
		}
	}
	for _, change := range []func(map[string]any){func(i map[string]any) { i["length"] = -1 }, func(i map[string]any) { i["length"] = 0 }, func(i map[string]any) { i["pieces"] = "bad" }, func(i map[string]any) { i["piece length"] = 0 }, func(i map[string]any) { i["length"] = 16385 }, func(i map[string]any) { i["files"] = []any{} }, func(i map[string]any) { i["name"] = ".." }} {
		info := validInfo()
		change(info)
		if _, code := torrentIdentity([]byte(encode(map[string]any{"info": info}))); code != "torrent-invalid" {
			t.Fatal("invalid info accepted", code)
		}
	}
	info := validInfo()
	info["meta version"] = 2
	if _, code := torrentIdentity([]byte(encode(map[string]any{"info": info}))); code != "unsupported-identity" {
		t.Fatal("hybrid accepted", code)
	}
	if _, code := torrentIdentity(make([]byte, (4<<20)+1)); code != "torrent-too-large" {
		t.Fatal(code)
	}
}

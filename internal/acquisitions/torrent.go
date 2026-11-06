package acquisitions

import (
	"bytes"
	"crypto/sha1" // The v1 BitTorrent protocol defines this identity digest.
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type bvalue struct {
	kind       byte
	text       []byte
	integer    int64
	list       []bvalue
	dict       map[string]bvalue
	start, end int
}
type bparser struct {
	data       []byte
	at, tokens int
}

var invalidTorrent = errors.New("invalid metainfo")

func (p *bparser) value(depth int) (bvalue, error) {
	value := bvalue{start: p.at}
	p.tokens++
	if depth > 32 || p.tokens > 100000 || p.at >= len(p.data) {
		return value, invalidTorrent
	}
	value.kind = p.data[p.at]
	p.at++
	switch value.kind {
	case 'i':
		end := bytes.IndexByte(p.data[p.at:], 'e')
		if end < 0 || end > 20 {
			return value, invalidTorrent
		}
		text := string(p.data[p.at : p.at+end])
		number, err := strconv.ParseInt(text, 10, 64)
		if err != nil || strconv.FormatInt(number, 10) != text {
			return value, invalidTorrent
		}
		value.integer = number
		p.at += end + 1
	case 'l', 'd':
		if value.kind == 'd' {
			value.dict = map[string]bvalue{}
		}
		var previous []byte
		for p.at < len(p.data) && p.data[p.at] != 'e' {
			if value.kind == 'l' {
				item, err := p.value(depth + 1)
				if err != nil {
					return value, err
				}
				value.list = append(value.list, item)
			} else {
				key, err := p.value(depth + 1)
				if err != nil || key.kind != 's' || previous != nil && bytes.Compare(previous, key.text) >= 0 {
					return value, invalidTorrent
				}
				previous = key.text
				item, err := p.value(depth + 1)
				if err != nil {
					return value, err
				}
				value.dict[string(key.text)] = item
			}
		}
		if p.at >= len(p.data) {
			return value, invalidTorrent
		}
		p.at++
	default:
		p.at = value.start
		colon := bytes.IndexByte(p.data[p.at:], ':')
		if colon < 1 || colon > 10 {
			return value, invalidTorrent
		}
		text := string(p.data[p.at : p.at+colon])
		length, err := strconv.ParseInt(text, 10, 64)
		if err != nil || length < 0 || strconv.FormatInt(length, 10) != text || length > int64(len(p.data)-(p.at+colon+1)) {
			return value, invalidTorrent
		}
		p.at += colon + 1
		value.kind = 's'
		value.text = p.data[p.at : p.at+int(length)]
		p.at += int(length)
	}
	value.end = p.at
	return value, nil
}
func safeComponent(value bvalue) bool {
	if value.kind != 's' || len(value.text) < 1 || len(value.text) > 255 || !utf8.Valid(value.text) {
		return false
	}
	text := string(value.text)
	if text == "." || text == ".." || strings.ContainsAny(text, "/\\\x00") {
		return false
	}
	// Drive-qualified paths are unsafe on the Windows development client as well.
	if len(text) >= 2 && text[1] == ':' {
		return false
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func torrentIdentity(data []byte) (string, string) {
	if len(data) > 4<<20 {
		return "", "torrent-too-large"
	}
	parser := bparser{data: data}
	root, err := parser.value(0)
	if err != nil || parser.at != len(data) || root.kind != 'd' {
		return "", "torrent-invalid"
	}
	info, ok := root.dict["info"]
	if !ok || info.kind != 'd' {
		return "", "torrent-invalid"
	}
	if _, ok := info.dict["meta version"]; ok {
		return "", "unsupported-identity"
	}
	if _, ok := info.dict["file tree"]; ok {
		return "", "unsupported-identity"
	}
	if !safeComponent(info.dict["name"]) {
		return "", "torrent-invalid"
	}
	if alternate, ok := info.dict["name.utf-8"]; ok && !safeComponent(alternate) {
		return "", "torrent-invalid"
	}
	if _, ok := info.dict["symlink path"]; ok {
		return "", "torrent-invalid"
	}
	piece := info.dict["piece length"]
	pieces := info.dict["pieces"]
	if piece.kind != 'i' || piece.integer < 1 || piece.integer > 64<<20 || pieces.kind != 's' || len(pieces.text)%20 != 0 {
		return "", "torrent-invalid"
	}
	length, single := info.dict["length"]
	files, multiple := info.dict["files"]
	if single == multiple {
		return "", "torrent-invalid"
	}
	var total int64
	add := func(length bvalue) bool {
		if length.kind != 'i' || length.integer < 0 || length.integer > 9007199254740991-total {
			return false
		}
		total += length.integer
		return true
	}
	if single {
		if !add(length) {
			return "", "torrent-invalid"
		}
	} else {
		if files.kind != 'l' || len(files.list) < 1 || len(files.list) > 10000 {
			return "", "torrent-invalid"
		}
		paths := map[string]bool{}
		for _, file := range files.list {
			if file.kind != 'd' || !add(file.dict["length"]) {
				return "", "torrent-invalid"
			}
			if _, ok := file.dict["symlink path"]; ok || bytes.Contains(file.dict["attr"].text, []byte("l")) {
				return "", "torrent-invalid"
			}
			if alternate, ok := file.dict["path.utf-8"]; ok {
				if alternate.kind != 'l' || len(alternate.list) < 1 || len(alternate.list) > 64 {
					return "", "torrent-invalid"
				}
				for _, part := range alternate.list {
					if !safeComponent(part) {
						return "", "torrent-invalid"
					}
				}
			}
			path := file.dict["path"]
			if path.kind != 'l' || len(path.list) < 1 || len(path.list) > 64 {
				return "", "torrent-invalid"
			}
			parts := []string{}
			for _, part := range path.list {
				if !safeComponent(part) {
					return "", "torrent-invalid"
				}
				parts = append(parts, string(part.text))
			}
			key := strings.Join(parts, "/")
			if paths[key] {
				return "", "torrent-invalid"
			}
			paths[key] = true
		}
	}
	if total < 1 || (total-1)/piece.integer+1 != int64(len(pieces.text)/20) {
		return "", "torrent-invalid"
	}
	hash := sha1.Sum(data[info.start:info.end])
	return hex.EncodeToString(hash[:]), ""
}

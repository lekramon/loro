package sources

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestCapabilityLimitsUnknownsAndAuthentication(t *testing.T) {
	for _, data := range []string{`<caps/>`, `<html/>`, `<caps><searching/></caps><caps/>`, `<!DOCTYPE caps><caps><searching/></caps>`, `<caps><limits max="0"/><searching/></caps>`, `<caps><limits max="10" default="20"/><searching/></caps>`, `<caps><searching><search available="maybe"/></searching></caps>`, `<caps>` + strings.Repeat(`<nested>`, 33) + strings.Repeat(`</nested>`, 33) + `</caps>`, `<caps><searching/>` + strings.Repeat(`<unknown/>`, 11000) + `</caps>`} {
		if _, err := parseCapabilities([]byte(data), ""); err == nil {
			t.Fatal("invalid caps accepted")
		}
	}
	caps, err := parseCapabilities([]byte(`<caps><searching><movie-search available="no" supportedParams="q,unknown,q"/></searching></caps>`), "")
	if err != nil || caps.MaxLimit != nil || caps.DefaultLimit != nil || caps.MovieSearch.Available || len(caps.MovieSearch.SupportedParams) != 1 || caps.Categories == nil {
		t.Fatal(caps, err)
	}
	categories := `<caps><searching/><categories>`
	for i := 1; i <= 257; i++ {
		categories += `<category id="` + strconv.Itoa(i) + `" name="Movie"/>`
	}
	categories += `</categories></caps>`
	if _, err = parseCapabilities([]byte(categories), ""); err == nil {
		t.Fatal("category bound ignored")
	}
	_, err = parseCapabilities([]byte(`<error code="101" description="private"/>`), "")
	var provider *ProviderError
	if !errors.As(err, &provider) || provider.Code != "source-authentication-failed" {
		t.Fatal(err)
	}
}

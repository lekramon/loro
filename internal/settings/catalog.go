package settings

import (
	_ "embed"
	"encoding/json"
)

type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// ISO 639-1 identifiers with English names generated using Node's bundled CLDR.
//
//go:embed languages.json
var languageData []byte

var languageCatalog = func() []Language {
	var values []Language
	if err := json.Unmarshal(languageData, &values); err != nil {
		panic(err)
	}
	return values
}()

var languageCodes = func() map[string]bool {
	result := map[string]bool{}
	for _, language := range languageCatalog {
		result[language.Code] = true
	}
	return result
}()

func Languages() []Language { return append([]Language(nil), languageCatalog...) }

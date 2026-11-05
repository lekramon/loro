package httpapi

import (
	_ "embed"
	"encoding/json"
)

type language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// ISO 639-1 identifiers with English names generated using Node's bundled CLDR.
//go:embed languages.json
var languageData []byte
var languages = func() []language {
	var result []language
	if err := json.Unmarshal(languageData, &result); err != nil {
		panic(err)
	}
	return result
}()

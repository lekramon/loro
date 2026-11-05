package httpapi

import "github.com/lekramon/piratarr/internal/settings"

type language = settings.Language

// ISO 639-1 identifiers with English names generated using Node's bundled CLDR.
var languages = settings.Languages()

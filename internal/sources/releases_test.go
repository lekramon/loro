package sources

import (
	"strings"
	"testing"

	"github.com/lekramon/piratarr/internal/settings"
)

func TestReleaseFeedRejectsUnsafeOrInconsistentDocuments(t *testing.T) {
	for _, data := range []string{
		`<!DOCTYPE rss SYSTEM "http://external.invalid"><rss><channel/></rss>`,
		`<rss><channel/></rss><rss/>`, `<caps/>`, `<rss/>`, `<rss><channel/><channel/></rss>`,
		`<rss><channel><item><title>Title</title></item></channel></rss>`,
		`<rss><channel><item><guid>id</guid></item></channel></rss>`,
		`<rss xmlns:t="http://torznab.com/schemas/2015/feed"><channel><t:response offset="1" total="2"/></channel></rss>`,
		`<rss xmlns:t="http://torznab.com/schemas/2015/feed"><channel><t:response offset="0" total="-1"/></channel></rss>`,
		`<rss xmlns:t="http://torznab.com/schemas/2015/feed"><channel><t:response offset="0" total="0"/><item><title>Title</title><guid>id</guid></item></channel></rss>`,
		`<rss><channel>` + strings.Repeat(`<item><title>Title</title><guid>id</guid></item>`, 3) + `</channel></rss>`,
	} {
		if _, err := parseReleases([]byte(data), Config{}, 2, 0); err == nil {
			t.Fatal("accepted invalid feed", data)
		}
	}
}

func TestReleaseEvidenceConflictsRemainUnknownAndDuplicatesRetainPagination(t *testing.T) {
	data := `<rss xmlns:t="http://torznab.com/schemas/2015/feed"><channel><item><title>Film 1080p 720p WEB-DL BluRay MKV MP4</title><guid>same</guid><enclosure length="2"/><t:attr name="size" value="1"/><t:attr name="seeders" value="-1"/><t:attr name="language" value="English,Unrecognized"/></item><item><title>Duplicate</title><guid>same</guid></item></channel></rss>`
	page, err := parseReleases([]byte(data), Config{Public: Public{Revision: 1}}, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || !page.HasMore || page.NextOffset == nil || *page.NextOffset != 2 {
		t.Fatal(page)
	}
	candidate := page.Items[0]
	if candidate.Resolution != nil || candidate.Format != nil || candidate.Container != nil || candidate.Seeders != nil || candidate.SizeBytes != nil || len(candidate.AudioLanguages) != 0 {
		t.Fatal(candidate)
	}
	profile := settings.Profile{AcceptedResolutions: []string{"1080p"}, AcceptedFormats: []string{"remux"}, AcceptedContainers: []string{"mkv"}, AudioPreferences: []string{"original"}}
	explain(&candidate, profile, "fr")
	if candidate.Status != "unknown" {
		t.Fatal(candidate)
	}
	page2, err := parseReleases([]byte(data), Config{Public: Public{Revision: 2}}, 2, 0)
	if err != nil || page2.Items[0].ID == candidate.ID {
		t.Fatal("revision did not isolate candidates")
	}
}

func TestShortProviderPageAdvancesWithoutSkippingAndAttributeGUIDIsPrivate(t *testing.T) {
	data := `<rss xmlns:t="http://torznab.com/schemas/2015/feed"><channel><t:response offset="0" total="40"/><item><title>Example</title><t:attr name="guid" value="private-attribute-id"/></item></channel></rss>`
	page, err := parseReleases([]byte(data), Config{}, 25, 0)
	if err != nil || page.NextOffset == nil || *page.NextOffset != 1 || len(page.Items) != 1 || len(page.Items[0].ID) != 64 {
		t.Fatal(page, err)
	}
}

func TestOriginalAudioNeedsObservedLanguageAndKnownMetadata(t *testing.T) {
	profile := settings.Profile{AudioPreferences: []string{"original"}}
	for _, entry := range []struct {
		languages        []string
		original, status string
	}{{[]string{}, "fr", "unknown"}, {[]string{"fr"}, "fr", "match"}, {[]string{"en"}, "fr", "mismatch"}, {[]string{"en"}, "", "unknown"}, {[]string{"en"}, "xx", "unknown"}} {
		candidate := Candidate{AudioLanguages: entry.languages}
		explain(&candidate, profile, entry.original)
		if candidate.Checks[3].Status != entry.status {
			t.Fatal(entry, candidate)
		}
	}
}

package storage

import (
	"context"
	"testing"

	"github.com/lekramon/piratarr/internal/policies"
	"github.com/lekramon/piratarr/internal/settings"
)

func TestAcquisitionPoliciesMigrateDefaultAndFenceEdits(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil { t.Fatal(err) }
	defer store.Close()
	input := settings.Input{Directories: settings.Directories{DownloadDirectory:t.TempDir(), MovieDirectory:t.TempDir()}, Profile:settings.Profile{AcceptedResolutions:[]string{"720p","1080p"},PreferredResolution:"1080p",AcceptedFormats:settings.Formats(),AcceptedContainers:settings.Containers(),AudioPreferences:[]string{"pt","original"}}}
	if _, err = store.CreateSettings(ctx, input); err != nil { t.Fatal(err) }
	profiles, err := store.ListAcquisitionProfiles(ctx)
	if err != nil || len(profiles) != 1 || profiles[0].Name != "Default" || profiles[0].PreferredResolution != "1080p" { t.Fatalf("default profile not preserved: %+v %v", profiles, err) }
	created, err := store.CreateAcquisitionProfile(ctx, policies.ProfileInput{Name:"4K",AcceptedResolutions:[]string{"1080p","2160p"},PreferredResolution:"2160p",AcceptedFormats:settings.Formats(),AcceptedContainers:settings.Containers(),AudioPreferences:[]string{"original"}})
	if err != nil { t.Fatal(err) }
	if _, err = store.UpdateAcquisitionProfile(ctx, created.ID, created.Revision-1, policies.ProfileInput{Name:"stale",AcceptedResolutions:[]string{"1080p"},PreferredResolution:"1080p",AcceptedFormats:settings.Formats(),AcceptedContainers:settings.Containers(),AudioPreferences:[]string{"original"}}); err != policies.ErrConflict { t.Fatalf("stale edit error = %v", err) }
	if _, err = store.CreateQualityDefinition(ctx, policies.QualityInput{Name:"1080p web",Resolution:"1080p",Source:"web-dl"}); err != nil { t.Fatal(err) }
	if _, err = store.CreateCustomFormat(ctx, policies.CustomFormatInput{Name:"HDR",Predicates:[]policies.Predicate{{Field:"hdr",Operator:"equals",Value:"true"}},Score:10}); err != nil { t.Fatal(err) }
	if _, err = store.write.ExecContext(ctx, "INSERT INTO movies(tmdb_id,title,metadata,added_at) VALUES(42,'Movie','{}','now')"); err != nil { t.Fatal(err) }
	if err = store.AssignAcquisitionProfile(ctx, 1, created.ID); err != nil { t.Fatal(err) }
}

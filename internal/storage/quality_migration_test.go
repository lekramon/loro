package storage

import (
	"context"
	"github.com/lekramon/piratarr/internal/settings"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestQualityMigrationPreservesExistingProfile(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	original, err := migrations.ReadFile("migrations/00001_foundation.sql")
	if err != nil {
		t.Fatal(err)
	}
	old, err := open(ctx, directory, fstest.MapFS{"migrations/00001_foundation.sql": {Data: original}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.write.Exec(`INSERT INTO settings VALUES(1,7,'downloads','movies','720p','created','saved'); INSERT INTO accepted_resolutions VALUES(1,'720p'); INSERT INTO audio_preferences VALUES(1,0,'pt');`)
	if err != nil {
		t.Fatal(err)
	}
	if err = old.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := Open(ctx, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	profile, err := upgraded.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Revision != 7 || profile.UpdatedAt != "saved" || !reflect.DeepEqual(profile.Profile.AcceptedResolutions, []string{"720p"}) || !reflect.DeepEqual(profile.Profile.AudioPreferences, []string{"pt"}) || !reflect.DeepEqual(profile.Profile.AcceptedFormats, settings.Formats()) || !reflect.DeepEqual(profile.Profile.AcceptedContainers, settings.Containers()) {
		t.Fatalf("migration changed existing choices: %+v", profile)
	}
}

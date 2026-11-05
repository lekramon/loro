package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/lekramon/piratarr/internal/storage"
	"golang.org/x/sys/windows"
)

func TestSetupRejectsReadOnlyDirectoryACL(t *testing.T) {
	store, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(store, fstest.MapFS{}, "127.0.0.1:8080")
	input := validSetup(t)
	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	apply := func(rights string) error {
		descriptor, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;" + rights + ";;;" + tokenUser.User.Sid.String() + ")")
		if err != nil {
			return err
		}
		acl, _, err := descriptor.DACL()
		if err != nil {
			return err
		}
		return windows.SetNamedSecurityInfo(input.Directories.MovieDirectory, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
	}
	if err := apply("FR"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := apply("FA"); err != nil {
			t.Error(err)
		}
	})
	response := setupRequest(t, h, "POST", "/api/v1/setup", encoded(t, input), false)
	if response.Code != 422 {
		t.Fatalf("read-only directory accepted: %d %s", response.Code, response.Body.String())
	}
	if matches, _ := filepath.Glob(filepath.Join(input.Directories.DownloadDirectory, ".piratarr-probe-*")); len(matches) > 0 {
		t.Fatal("probe was not removed")
	}
	if completed, _ := store.SetupCompleted(context.Background()); completed {
		t.Fatal("read-only directory persisted")
	}
	if _, err := os.Stat(input.Directories.MovieDirectory); err != nil {
		t.Fatal("validation removed directory")
	}
}

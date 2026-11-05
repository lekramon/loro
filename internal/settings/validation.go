package settings

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var resolutions = []string{"480p", "576p", "720p", "1080p", "2160p"}

func Resolutions() []string { return append([]string(nil), resolutions...) }

func Validate(ctx context.Context, input Input, dataDirectory string) (Input, error) {
	var fields []FieldError
	add := func(field, message string) { fields = append(fields, FieldError{field, message}) }
	accepted := map[string]bool{}
	for _, value := range input.Profile.AcceptedResolutions {
		supported := false
		for _, resolution := range resolutions {
			if resolution == value {
				supported = true
				break
			}
		}
		if !supported || accepted[value] {
			add("profile.acceptedResolutions", "Choose distinct supported resolutions.")
		}
		accepted[value] = true
	}
	if len(accepted) == 0 {
		add("profile.acceptedResolutions", "Choose at least one accepted resolution.")
	}
	if !accepted[input.Profile.PreferredResolution] {
		add("profile.preferredResolution", "Choose a preferred resolution from the accepted set.")
	}
	audio := map[string]bool{}
	if len(input.Profile.AudioPreferences) == 0 {
		add("profile.audioPreferences", "Choose at least one language or original audio.")
	}
	for _, choice := range input.Profile.AudioPreferences {
		if (choice != "original" && !languageCodes[choice]) || audio[choice] {
			add("profile.audioPreferences", "Choose distinct supported languages or original audio.")
		}
		audio[choice] = true
	}
	if len(fields) > 0 {
		return Input{}, &ValidationError{fields}
	}
	canonicalData, err := filepath.EvalSymlinks(dataDirectory)
	if err != nil {
		return Input{}, ErrUnavailable
	}
	downloads, err := canonicalDirectory(input.Directories.DownloadDirectory)
	if err != nil {
		add("directories.downloadDirectory", err.Error())
	}
	movies, err := canonicalDirectory(input.Directories.MovieDirectory)
	if err != nil {
		add("directories.movieDirectory", err.Error())
	}
	if downloads != "" && overlap(downloads, canonicalData) {
		add("directories.downloadDirectory", "Choose a directory separate from application data.")
	}
	if movies != "" && overlap(movies, canonicalData) {
		add("directories.movieDirectory", "Choose a directory separate from application data.")
	}
	if downloads != "" && movies != "" && overlap(downloads, movies) {
		add("directories.movieDirectory", "Movie and download directories must not overlap.")
	}
	if len(fields) > 0 {
		return Input{}, &ValidationError{fields}
	}
	for _, root := range []struct{ field, path string }{{"directories.downloadDirectory", downloads}, {"directories.movieDirectory", movies}} {
		if err := ctx.Err(); err != nil {
			return Input{}, ErrUnavailable
		}
		if err := probeDirectory(root.path); err != nil {
			add(root.field, "The directory must allow reading, writing, and removal of a temporary validation file.")
		}
	}
	if len(fields) > 0 {
		return Input{}, &ValidationError{fields}
	}
	input.Directories = Directories{downloads, movies}
	ordered := make([]string, 0, len(accepted))
	for _, resolution := range resolutions {
		if accepted[resolution] {
			ordered = append(ordered, resolution)
		}
	}
	input.Profile.AcceptedResolutions = ordered
	return input, nil
}

func canonicalDirectory(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("Enter an absolute existing directory path.")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", errors.New("Directory does not exist or is inaccessible.")
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", errors.New("Directory path cannot be resolved.")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", errors.New("Choose an accessible directory, not a file.")
	}
	return filepath.Clean(resolved), nil
}

func overlap(left, right string) bool {
	if runtime.GOOS == "windows" {
		left, right = strings.ToLower(left), strings.ToLower(right)
	}
	within := func(parent, child string) bool {
		relative, err := filepath.Rel(parent, child)
		return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
	}
	return within(left, right) || within(right, left)
}

func probeDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	_, readErr := directory.Readdirnames(1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	probe, err := os.CreateTemp(path, ".piratarr-probe-*")
	if err != nil {
		return err
	}
	_, writeErr := probe.WriteString("Piratarr directory validation\n")
	closeErr = probe.Close()
	removeErr := os.Remove(probe.Name())
	return errors.Join(writeErr, closeErr, removeErr)
}

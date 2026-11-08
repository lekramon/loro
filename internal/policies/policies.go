// Package policies owns named acquisition profiles and their bounded release rules.
package policies

import (
	"context"
	"errors"
	"strings"
)

type Profile struct {
	ID                 int64    `json:"id"`
	Name               string   `json:"name"`
	Revision           int      `json:"revision"`
	AcceptedResolutions []string `json:"acceptedResolutions"`
	PreferredResolution string   `json:"preferredResolution"`
	AcceptedFormats    []string `json:"acceptedFormats"`
	AcceptedContainers []string `json:"acceptedContainers"`
	AudioPreferences   []string `json:"audioPreferences"`
	CutoffScore        int      `json:"cutoffScore"`
	AllowUpgrades      bool     `json:"allowUpgrades"`
	UpdatedAt          string   `json:"updatedAt"`
}

type QualityDefinition struct {
	ID int64 `json:"id"`; Name string `json:"name"`; Revision int `json:"revision"`
	Resolution string `json:"resolution"`; Source string `json:"source"`
	MinSizePerMinute *float64 `json:"minSizePerMinute"`; PreferredSizePerMinute *float64 `json:"preferredSizePerMinute"`; MaxSizePerMinute *float64 `json:"maxSizePerMinute"`
}

type Predicate struct { Field string `json:"field"`; Operator string `json:"operator"`; Value string `json:"value"` }
type CustomFormat struct { ID int64 `json:"id"`; Name string `json:"name"`; Revision int `json:"revision"`; Predicates []Predicate `json:"predicates"`; Score int `json:"score"`; MinimumScore int `json:"minimumScore"` }

type ProfileInput struct { Name string `json:"name"`; AcceptedResolutions []string `json:"acceptedResolutions"`; PreferredResolution string `json:"preferredResolution"`; AcceptedFormats []string `json:"acceptedFormats"`; AcceptedContainers []string `json:"acceptedContainers"`; AudioPreferences []string `json:"audioPreferences"`; CutoffScore int `json:"cutoffScore"`; AllowUpgrades bool `json:"allowUpgrades"` }
type QualityInput struct { Name string `json:"name"`; Resolution string `json:"resolution"`; Source string `json:"source"`; MinSizePerMinute *float64 `json:"minSizePerMinute"`; PreferredSizePerMinute *float64 `json:"preferredSizePerMinute"`; MaxSizePerMinute *float64 `json:"maxSizePerMinute"` }
type CustomFormatInput struct { Name string `json:"name"`; Predicates []Predicate `json:"predicates"`; Score int `json:"score"`; MinimumScore int `json:"minimumScore"` }

var ErrNotFound = errors.New("policy not found")
var ErrConflict = errors.New("policy revision conflict")
var ErrInvalid = errors.New("policy validation failed")

type Repository interface {
	ListAcquisitionProfiles(context.Context) ([]Profile, error)
	CreateAcquisitionProfile(context.Context, ProfileInput) (Profile, error)
	UpdateAcquisitionProfile(context.Context, int64, int, ProfileInput) (Profile, error)
	AssignAcquisitionProfile(context.Context, int64, int64) error
	ListQualityDefinitions(context.Context) ([]QualityDefinition, error)
	CreateQualityDefinition(context.Context, QualityInput) (QualityDefinition, error)
	ListCustomFormats(context.Context) ([]CustomFormat, error)
	CreateCustomFormat(context.Context, CustomFormatInput) (CustomFormat, error)
}

func ValidateProfile(input ProfileInput) error {
	if n := strings.TrimSpace(input.Name); n == "" || len(n) > 100 { return ErrInvalid }
	if len(input.AcceptedResolutions) == 0 || input.PreferredResolution == "" { return ErrInvalid }
	seen := map[string]bool{}
	for _, v := range input.AcceptedResolutions { if seen[v] || (v != "480p" && v != "576p" && v != "720p" && v != "1080p" && v != "2160p") { return ErrInvalid }; seen[v] = true }
	if !seen[input.PreferredResolution] || input.CutoffScore < 0 || input.CutoffScore > 10000 { return ErrInvalid }
	return nil
}
func ValidateQuality(input QualityInput) error { if strings.TrimSpace(input.Name)=="" || input.Resolution=="" || input.Source=="" { return ErrInvalid }; for _, v := range []*float64{input.MinSizePerMinute,input.PreferredSizePerMinute,input.MaxSizePerMinute} { if v != nil && *v < 0 { return ErrInvalid } }; return nil }
func ValidateCustomFormat(input CustomFormatInput) error { if strings.TrimSpace(input.Name)=="" || len(input.Predicates)==0 || input.Score < -10000 || input.Score > 10000 { return ErrInvalid }; for _, p := range input.Predicates { if p.Field!="resolution" && p.Field!="source" && p.Field!="format" && p.Field!="container" && p.Field!="audio" && p.Field!="codec" && p.Field!="hdr" { return ErrInvalid }; if p.Operator!="equals" && p.Operator!="contains" { return ErrInvalid }; if strings.ContainsAny(p.Value, "\\()[]{}|+*?") { return ErrInvalid } }; return nil }

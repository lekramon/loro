package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/lekramon/piratarr/internal/policies"
)

func decodePolicy(w http.ResponseWriter, r *http.Request, dst any) bool { d:=json.NewDecoder(r.Body); d.DisallowUnknownFields(); if err:=d.Decode(dst);err!=nil{fail(w,400,"invalid-request","Request body is invalid.");return false};var extra any;if err:=d.Decode(&extra);err!=io.EOF{fail(w,400,"invalid-request","Request body must contain exactly one JSON object.");return false};return true }
func policyFailure(w http.ResponseWriter, err error) { switch { case errors.Is(err,policies.ErrInvalid):fail(w,422,"validation-failed","Policy values are invalid.");case errors.Is(err,policies.ErrConflict):fail(w,409,"revision-conflict","This policy changed in another tab. Reload before editing.");default:fail(w,503,"storage-unavailable","Policy storage is unavailable.")} }
func (h *Handler) policyAPI(w http.ResponseWriter,r *http.Request){ repo,ok:=h.storage.(policies.Repository);if !ok{fail(w,503,"not-ready","Acquisition policies are not available.");return}; route:=r.URL.Path
	if route=="/api/v1/acquisition-profiles" { if r.Method==http.MethodGet {v,e:=repo.ListAcquisitionProfiles(r.Context());if e!=nil{policyFailure(w,e);return};writeJSON(w,200,map[string]any{"items":v});return};if r.Method==http.MethodPost{var in policies.ProfileInput;if !decodePolicy(w,r,&in){return};v,e:=repo.CreateAcquisitionProfile(r.Context(),in);if e!=nil{policyFailure(w,e);return};writeJSON(w,201,v);return};methodNotAllowed(w,"GET, POST");return }
	if strings.HasPrefix(route,"/api/v1/acquisition-profiles/") { if r.Method!=http.MethodPut{methodNotAllowed(w,"PUT");return};id,e:=strconv.ParseInt(strings.TrimPrefix(route,"/api/v1/acquisition-profiles/"),10,64);if e!=nil||id<1{fail(w,404,"not-found","Acquisition profile does not exist.");return};var body struct{Revision int `json:"revision"`;policies.ProfileInput};if !decodePolicy(w,r,&body){return};if body.Revision<1{fail(w,422,"validation-failed","Revision is required.");return};v,e:=repo.UpdateAcquisitionProfile(r.Context(),id,body.Revision,body.ProfileInput);if e!=nil{policyFailure(w,e);return};writeJSON(w,200,v);return }
	if route=="/api/v1/quality-definitions" {if r.Method==http.MethodGet{v,e:=repo.ListQualityDefinitions(r.Context());if e!=nil{policyFailure(w,e);return};writeJSON(w,200,map[string]any{"items":v});return};if r.Method==http.MethodPost{var in policies.QualityInput;if !decodePolicy(w,r,&in){return};v,e:=repo.CreateQualityDefinition(r.Context(),in);if e!=nil{policyFailure(w,e);return};writeJSON(w,201,v);return};methodNotAllowed(w,"GET, POST");return}
	if route=="/api/v1/custom-formats" {if r.Method==http.MethodGet{v,e:=repo.ListCustomFormats(r.Context());if e!=nil{policyFailure(w,e);return};writeJSON(w,200,map[string]any{"items":v});return};if r.Method==http.MethodPost{var in policies.CustomFormatInput;if !decodePolicy(w,r,&in){return};v,e:=repo.CreateCustomFormat(r.Context(),in);if e!=nil{policyFailure(w,e);return};writeJSON(w,201,v);return};methodNotAllowed(w,"GET, POST");return}
	if strings.HasSuffix(route,"/acquisition-profile") {if r.Method!=http.MethodPut{methodNotAllowed(w,"PUT");return};raw:=strings.TrimSuffix(strings.TrimPrefix(route,"/api/v1/catalog/movies/"),"/acquisition-profile");movie,e1:=strconv.ParseInt(raw,10,64);var body struct{ProfileID int64 `json:"profileId"`};if e1!=nil||movie<1||!decodePolicy(w,r,&body){if e1!=nil{fail(w,404,"not-found","Movie does not exist.")};return};if body.ProfileID<1{fail(w,422,"validation-failed","Profile is required.");return};if e:=repo.AssignAcquisitionProfile(r.Context(),movie,body.ProfileID);e!=nil{policyFailure(w,e);return};writeJSON(w,200,map[string]any{"movieId":movie,"profileId":body.ProfileID});return}
	fail(w,404,"not-found","Policy endpoint does not exist.")
}

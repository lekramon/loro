package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lekramon/piratarr/internal/series"
)

func (h *Handler) seriesAPI(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path,"/api/v1/series") && !strings.HasPrefix(r.URL.Path,"/api/v1/episodes/") { return false }
	if h.stopping.Load(){fail(w,503,"not-ready","The service is stopping.");return true};if h.seriesRepo==nil{fail(w,503,"series-unavailable","Series catalog is unavailable.");return true}
	ctx,cancel:=context.WithTimeout(r.Context(),10*time.Second);defer cancel()
	if r.URL.Path=="/api/v1/series" {if r.Method==http.MethodPost{var input series.Input;if _,ok:=decodeConfiguration(w,r,&input,"provider","providerId","title","anime");!ok{return true};v,err:=h.seriesRepo.CreateSeries(ctx,input);if err!=nil{seriesFailure(w,err);return true};writeJSON(w,201,v);return true};if r.Method!=http.MethodGet{methodNotAllowed(w,"GET, POST");return true};q,ok:=catalogQuery(w,r,"limit","offset","anime");if !ok{return true};limit,a:=boundedInteger(q,"limit",25,1,100);offset,b:=boundedInteger(q,"offset",0,0,1000000);if !a||!b{fail(w,422,"validation-failed","Provide valid pagination.");return true};var anime *bool;if q.Has("anime"){v:=q.Get("anime");parsed,e:=strconv.ParseBool(v);if e!=nil{fail(w,422,"validation-failed","Anime must be true or false.");return true};anime=&parsed};v,err:=h.seriesRepo.ListSeries(ctx,limit,offset,anime);if err!=nil{seriesFailure(w,err);return true};writeJSON(w,200,v);return true}
	parts:=strings.Split(strings.TrimPrefix(r.URL.Path,"/api/v1/series/"),"/");if len(parts)>=1{seriesID,err:=strconv.ParseInt(parts[0],10,64);if err!=nil||seriesID<1{fail(w,422,"validation-failed","Series ID is invalid.");return true};if len(parts)==1{if r.Method!=http.MethodGet{methodNotAllowed(w,"GET");return true};v,e:=h.seriesRepo.GetSeries(ctx,seriesID);if e!=nil{seriesFailure(w,e);return true};writeJSON(w,200,v);return true};if parts[1]=="seasons"{if r.Method!=http.MethodGet{methodNotAllowed(w,"GET");return true};v,e:=h.seriesRepo.ListSeasons(ctx,seriesID);if e!=nil{seriesFailure(w,e);return true};writeJSON(w,200,map[string]any{"items":v});return true};if parts[1]=="episodes"{if r.Method!=http.MethodGet{methodNotAllowed(w,"GET");return true};q,ok:=catalogQuery(w,r,"limit","offset","season");if !ok{return true};limit,a:=boundedInteger(q,"limit",25,1,100);offset,b:=boundedInteger(q,"offset",0,0,1000000);if !a||!b{fail(w,422,"validation-failed","Provide valid pagination.");return true};var seasonNumber *int;if q.Has("season"){v,e:=strconv.Atoi(q.Get("season"));if e!=nil||v<0{fail(w,422,"validation-failed","Season is invalid.");return true};seasonNumber=&v};v,e:=h.seriesRepo.ListEpisodes(ctx,seriesID,limit,offset,seasonNumber);if e!=nil{seriesFailure(w,e);return true};writeJSON(w,200,v);return true}}
	if strings.HasPrefix(r.URL.Path,"/api/v1/episodes/"){fail(w,404,"not-found","Episode endpoint is not implemented in this slice.");return true};fail(w,404,"not-found","Series endpoint not found.");return true
}

func seriesFailure(w http.ResponseWriter,err error){switch{case errors.Is(err,series.ErrNotFound):fail(w,404,"not-found","Series was not found.");case errors.Is(err,series.ErrConflict):fail(w,409,"conflict","Series provider identity already exists.");default:fail(w,503,"series-unavailable","Series catalog storage is unavailable.")}}

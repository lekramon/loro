package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/lekramon/piratarr/internal/imports"
	"github.com/lekramon/piratarr/internal/library"
)

func (h *Handler) importAPI(w http.ResponseWriter, r *http.Request) {
	if h.stopping.Load() || h.importService == nil { fail(w,503,"not-ready","Import service is unavailable."); return }
	ctx,cancel:=context.WithTimeout(r.Context(),30*time.Second); defer cancel()
	if r.URL.Path=="/api/v1/imports" && r.Method==http.MethodGet { q,ok:=catalogQuery(w,r,"limit","offset");if !ok{return};limit,a:=boundedInteger(q,"limit",25,1,50);offset,b:=boundedInteger(q,"offset",0,0,10000);if !a||!b{fail(w,422,"validation-failed","Provide valid import pagination.");return};v,err:=h.importService.Repo.List(ctx,limit,offset);if err!=nil{importFailure(w,err);return};writeJSON(w,200,v);return }
	if r.Method!=http.MethodPost { methodNotAllowed(w,"GET, POST");return }
	if r.URL.Path=="/api/v1/imports/preview" {var in imports.PreviewInput;if _,ok:=decodeConfiguration(w,r,&in,"downloadId","revision");!ok{return};v,err:=h.importService.PreviewFile(ctx,in);if err!=nil{importFailure(w,err);return};writeJSON(w,200,v);return}
	var in imports.Input;if _,ok:=decodeConfiguration(w,r,&in,"downloadId","fileId","movieId","revision","conflictPolicy");!ok{return};v,err:=h.importService.Create(ctx,in);if err!=nil{importFailure(w,err);return};writeJSON(w,202,v)
}
func importFailure(w http.ResponseWriter,err error){switch{case errors.Is(err,imports.ErrNotFound):fail(w,404,"import-not-found","Download or import does not exist.");case errors.Is(err,imports.ErrChanged):fail(w,409,"import-changed","Download or import changed. Refresh before continuing.");case errors.Is(err,imports.ErrConflict):fail(w,409,"import-conflict","The destination already contains a conflicting file.");case errors.Is(err,imports.ErrReview):fail(w,422,"import-needs-review","The file needs explicit review before import.");case errors.Is(err,imports.ErrInvalid),errors.Is(err,library.ErrInvalidVideo):fail(w,422,"validation-failed","The file is not a valid import candidate.");case errors.Is(err,library.ErrProbeTimeout):fail(w,504,"video-probe-timeout","Video verification timed out.");default:fail(w,503,"import-unavailable","Import storage or verification is unavailable.")}}

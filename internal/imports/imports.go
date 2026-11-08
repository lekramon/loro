// Package imports verifies and publishes completed download files into the library.
package imports

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lekramon/piratarr/internal/library"
)

type Error string
func (e Error) Error() string { return string(e) }
const (
	ErrUnavailable Error = "import-unavailable"
	ErrNotFound Error = "import-not-found"
	ErrChanged Error = "import-changed"
	ErrInvalid Error = "import-invalid"
	ErrConflict Error = "import-conflict"
	ErrReview Error = "import-needs-review"
)

type Preview struct {
	DownloadID string `json:"downloadId"`; Revision int `json:"revision"`; MovieID int64 `json:"movieId"`; MovieTitle string `json:"movieTitle"`; MovieYear *int `json:"movieYear,omitempty"`; RelativePath string `json:"relativePath"`; SizeBytes int64 `json:"sizeBytes"`; Destination string `json:"destination"`; State string `json:"state"`
	SourcePath string `json:"-"`; DestinationRoot string `json:"-"`; ModifiedAt string `json:"-"`
}
type Record struct { ID string `json:"id"`; DownloadID string `json:"downloadId"`; MovieID int64 `json:"movieId"`; RelativePath string `json:"relativePath"`; State string `json:"state"`; Phase string `json:"phase"`; Revision int `json:"revision"`; CreatedAt string `json:"createdAt"`; UpdatedAt string `json:"updatedAt"` }
type List struct { Items []Record `json:"items"`; Total int `json:"total"`; Limit int `json:"limit"`; Offset int `json:"offset"` }
type PreviewInput struct { DownloadID string `json:"downloadId"`; Revision int `json:"revision"` }
type Input struct { DownloadID string `json:"downloadId"`; FileID string `json:"fileId"`; MovieID int64 `json:"movieId"`; Revision int `json:"revision"`; ConflictPolicy string `json:"conflictPolicy"` }
type Repository interface { Preview(context.Context,string,int) (Preview,error); List(context.Context,int,int) (List,error); Plan(context.Context,Input,Preview,string) (Record,error); Commit(context.Context,string,int,string,string,int64,string) (Record,error); MarkNeedsReview(context.Context,string,int,string) error }
type Service struct { Repo Repository; Prober library.Prober; Now func() time.Time }
func (s *Service) PreviewFile(ctx context.Context, in PreviewInput) (Preview,error) { if len(in.DownloadID)!=32 || in.Revision<1{return Preview{},ErrInvalid}; p,err:=s.Repo.Preview(ctx,in.DownloadID,in.Revision);if err!=nil{return Preview{},err}; f,err:=os.Open(p.SourcePath);if err!=nil{return Preview{},ErrReview};defer f.Close();info,err:=f.Stat();if err!=nil||!info.Mode().IsRegular()||info.Size()<=0||info.Size()!=p.SizeBytes{return Preview{},ErrChanged};if s.Prober!=nil {if err=s.Prober(ctx,f);err!=nil{return Preview{},err}};p.State="awaiting-review";return p,nil }
func (s *Service) Create(ctx context.Context,in Input) (Record,error) { if len(in.DownloadID)!=32||in.FileID==""||in.MovieID<1||in.Revision<1||(in.ConflictPolicy!="skip"&&in.ConflictPolicy!="keep-both"&&in.ConflictPolicy!="replace"){return Record{},ErrInvalid};p,err:=s.PreviewFile(ctx,PreviewInput{in.DownloadID,in.Revision});if err!=nil{return Record{},err};if in.FileID!=p.RelativePath||in.MovieID!=p.MovieID{return Record{},ErrChanged};dest:=filepath.Join(p.DestinationRoot, safeName(p.MovieTitle,p.MovieYear), filepath.Base(p.RelativePath));if in.ConflictPolicy=="keep-both"{dest=strings.TrimSuffix(dest,filepath.Ext(dest))+"-"+in.DownloadID+filepath.Ext(dest)};record,err:=s.Repo.Plan(ctx,in,p,dest);if err!=nil{return Record{},err};if err=publish(p.SourcePath,dest,in.ConflictPolicy);err!=nil{_ = s.Repo.MarkNeedsReview(ctx,record.ID,record.Revision,"publish-failed");return Record{},err};return s.Repo.Commit(ctx,record.ID,record.Revision,dest,filepath.ToSlash(strings.TrimPrefix(dest,p.DestinationRoot+string(filepath.Separator))),p.SizeBytes,time.Now().UTC().Format(time.RFC3339Nano))}
func safeName(title string, year *int) string { title=strings.TrimSpace(title);if title==""{title="Untitled"};title=strings.NewReplacer("/","-","\\","-",":","-","*","-","?","-","\"","'","<","(",">",")","|","-").Replace(title);title=strings.TrimRight(title," .");if year!=nil{return title+" ("+itoa(*year)+")"};return title }
func itoa(v int) string { if v==0{return "0"}; neg:=v<0;if neg{v=-v};b:=make([]byte,0,12);for v>0{b=append([]byte{byte('0'+v%10)},b...);v/=10};if neg{b=append([]byte{'-'},b...)};return string(b) }
func publish(source,destination,policy string) error { if err:=os.MkdirAll(filepath.Dir(destination),0750);err!=nil{return ErrUnavailable};if _,err:=os.Stat(destination);err==nil {if policy=="skip"{return ErrConflict};if policy=="replace"{backup:=destination+".louro-backup";if err=os.Rename(destination,backup);err!=nil{return ErrConflict};defer func(){if _,e:=os.Stat(backup);e==nil{_ = os.Remove(backup)}}()}};if err:=os.Link(source,destination);err==nil{return nil};in,err:=os.Open(source);if err!=nil{return ErrReview};defer in.Close();out,err:=os.OpenFile(destination+".louro-staging",os.O_CREATE|os.O_EXCL|os.O_WRONLY,0640);if err!=nil{return ErrConflict};staging:=out.Name();defer func(){_ = os.Remove(staging)}();if _,err=ioCopy(out,in);err!=nil{_ = out.Close();return ErrUnavailable};if err=out.Close();err!=nil{return ErrUnavailable};if err=os.Rename(staging,destination);err!=nil{return ErrUnavailable};return nil }
func ioCopy(dst *os.File,src *os.File)(int64,error){return io.Copy(dst,src)}

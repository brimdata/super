package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/brimdata/zed/pkg/iosrc"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zio/zjsonio"
)

const RequestIDHeader = "X-Request-ID"

func RequestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(RequestIDHeader); v != nil {
		return v.(string)
	}
	return ""
}

type Error struct {
	Type    string      `json:"type"`
	Kind    string      `json:"kind"`
	Message string      `json:"error"`
	Info    interface{} `json:"info,omitempty"`
}

func (e Error) Error() string {
	return e.Message
}

type ASTRequest struct {
	ZQL string `json:"zql"`
}

type TaskStart struct {
	Type   string `json:"type"`
	TaskID int64  `json:"task_id"`
}

type TaskEnd struct {
	Type   string `json:"type"`
	TaskID int64  `json:"task_id"`
	Error  *Error `json:"error,omitempty"`
}

type SearchRequest struct {
	Space SpaceID         `json:"space" validate:"required"`
	Proc  json.RawMessage `json:"proc,omitempty"`
	Span  nano.Span       `json:"span"`
	Dir   int             `json:"dir" validate:"required"`
}

type SearchRecords struct {
	Type      string           `json:"type"`
	ChannelID int              `json:"channel_id"`
	Records   []zjsonio.Record `json:"records"`
}

type SearchWarning struct {
	Type    string `json:"type"`
	Warning string `json:"warning"`
}

type SearchEnd struct {
	Type      string `json:"type"`
	ChannelID int    `json:"channel_id"`
	Reason    string `json:"reason"`
}

type SearchStats struct {
	Type       string  `json:"type"`
	StartTime  nano.Ts `json:"start_time"`
	UpdateTime nano.Ts `json:"update_time"`
	ScannerStats
}

type ScannerStats struct {
	BytesRead      int64 `json:"bytes_read"`
	BytesMatched   int64 `json:"bytes_matched"`
	RecordsRead    int64 `json:"records_read"`
	RecordsMatched int64 `json:"records_matched"`
}

var spaceIDRegexp = regexp.MustCompile("^[a-zA-Z0-9_]+$")

type SpaceID string

// String is part of the flag.Value interface allowing a SpaceID value to be
// used as a command line flag.
func (s SpaceID) String() string {
	return string(s)
}

// Set is part of the flag.Value interface allowing a SpaceID value to be
// used as a command line flag.
func (s *SpaceID) Set(str string) error {
	if !spaceIDRegexp.MatchString(str) {
		return errors.New("all characters in a SpaceID must be [a-zA-Z0-9_]")
	}
	*s = SpaceID(str)
	return nil
}

type Space struct {
	ID          SpaceID     `json:"id" zng:"id"`
	Name        string      `json:"name" zng:"name"`
	DataPath    iosrc.URI   `json:"data_path" zng:"data_path"`
	StorageKind StorageKind `json:"storage_kind" zng:"storage_kind"`
}

type SpaceInfo struct {
	Space
	Span *nano.Span `json:"span,omitempty"`
	Size int64      `json:"size" unit:"bytes"`
}

type VersionResponse struct {
	Version string `json:"version"`
}

type SpacePostRequest struct {
	Name     string         `json:"name"`
	DataPath string         `json:"data_path"`
	Storage  *StorageConfig `json:"storage,omitempty"`
}

type SpacePutRequest struct {
	Name string `json:"name"`
}

type LogPostRequest struct {
	Paths   []string        `json:"paths"`
	StopErr bool            `json:"stop_err"`
	Shaper  json.RawMessage `json:"shaper,omitempty"`
}

type LogPostWarning struct {
	Type    string `json:"type"`
	Warning string `json:"warning"`
}

type LogPostStatus struct {
	Type         string `json:"type"`
	LogTotalSize int64  `json:"log_total_size" unit:"bytes"`
	LogReadSize  int64  `json:"log_read_size" unit:"bytes"`
}

type LogPostResponse struct {
	Type      string   `json:"type"`
	BytesRead int64    `json:"bytes_read" unit:"bytes"`
	Warnings  []string `json:"warnings"`
}

type IndexSearchRequest struct {
	IndexName string   `json:"index_name"`
	Patterns  []string `json:"patterns"`
}

type IndexPostRequest struct {
	Patterns   []string `json:"patterns"`
	ZQL        string   `json:"zql,omitempty"`
	Keys       []string `json:"keys"`
	InputFile  string   `json:"input_file"`
	OutputFile string   `json:"output_file"`
}

type StorageKind string

const (
	UnknownStore StorageKind = ""
	ArchiveStore StorageKind = "archivestore"
	FileStore    StorageKind = "filestore"
)

func (k StorageKind) String() string {
	return string(k)
}

func (k *StorageKind) Set(s string) error {
	switch s := StorageKind(s); s {
	case ArchiveStore, FileStore:
		*k = s
		return nil
	}
	return fmt.Errorf("unknown storage kind: %s", s)
}

type StorageConfig struct {
	Kind    StorageKind    `json:"kind"`
	Archive *ArchiveConfig `json:"archive,omitempty"`
}

type ArchiveConfig struct {
	CreateOptions *ArchiveCreateOptions `json:"create_options,omitempty"`
}

type ArchiveCreateOptions struct {
	LogSizeThreshold *int64 `json:"log_size_threshold,omitempty"`
}

// FileStoreReadOnly controls if new spaces may be created using the
// FileStore storage kind, and if existing FileStore spaces may have new
// data added to them.
// This intended to be temporary until we transition to only allowing archive
// stores for new spaces; see issue 1085.
var FileStoreReadOnly bool

func DefaultStorageKind() StorageKind {
	if FileStoreReadOnly {
		return ArchiveStore
	}
	return FileStore
}

package api

import (
	"context"

	"github.com/brimdata/super/compiler/srcfiles"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/sbuf"
	"github.com/segmentio/ksuid"
)

const RequestIDHeader = "X-Request-ID"

func RequestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(RequestIDHeader); v != nil {
		return v.(string)
	}
	return ""
}

type Error struct {
	Type              string             `json:"type"`
	Kind              string             `json:"kind"`
	Message           string             `json:"error"`
	CompilationErrors srcfiles.ErrorList `json:"compilation_errors,omitempty"`
}

func (e Error) Error() string {
	return e.Message
}

type VersionResponse struct {
	Version string `json:"version"`
}

type PoolPostRequest struct {
	Name       string   `json:"name"`
	SortKeys   SortKeys `json:"layout"`
	SeekStride int      `json:"seek_stride"`
	Thresh     int64    `json:"thresh"`
}

type SortKeys struct {
	Order order.Which `json:"order" super:"order"`
	Keys  field.List  `json:"keys" super:"keys"`
}

type PoolPutRequest struct {
	Name string `json:"name"`
}

type BranchPostRequest struct {
	Name   string `json:"name"`
	Commit string `json:"commit"`
}

type BranchMergeRequest struct {
	At string `json:"at"`
}

type CompactRequest struct {
	ObjectIDs []ksuid.KSUID `super:"object_ids"`
}

type DeleteRequest struct {
	ObjectIDs []string `super:"object_ids"`
	Where     string   `super:"where"`
}

type CommitMessage struct {
	Author string `super:"author"`
	Body   string `super:"body"`
	Meta   string `super:"meta"`
}

type CommitResponse struct {
	Commit   ksuid.KSUID `super:"commit"`
	Warnings []string    `super:"warnings"`
}

type EventBranchCommit struct {
	CommitID ksuid.KSUID `super:"commit_id"`
	PoolID   ksuid.KSUID `super:"pool_id"`
	Branch   string      `super:"branch"`
	Parent   string      `super:"parent"`
}

type EventPool struct {
	PoolID ksuid.KSUID `super:"pool_id"`
}

type EventBranch struct {
	PoolID ksuid.KSUID `super:"pool_id"`
	Branch string      `super:"branch"`
}

type QueryRequest struct {
	Query string `json:"query"`
}

type QueryChannelSet struct {
	Channel string `json:"channel" super:"channel"`
}

type QueryChannelEnd struct {
	Channel string `json:"channel" super:"channel"`
}

type QueryError struct {
	Error string `json:"error" super:"error"`
}

type QueryStats struct {
	StartTime  nano.Ts `json:"start_time" super:"start_time"`
	UpdateTime nano.Ts `json:"update_time" super:"update_time"`
	sbuf.Progress
}

type QueryWarning struct {
	Warning string `json:"warning" super:"warning"`
}

type VacuumResponse struct {
	ObjectIDs []ksuid.KSUID `super:"object_ids"`
}

type VectorRequest struct {
	ObjectIDs []ksuid.KSUID `super:"object_ids"`
}

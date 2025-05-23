package commits

import (
	"errors"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/bsupbytes"
	"github.com/brimdata/super/lake/data"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/sup"
	"github.com/segmentio/ksuid"
)

var ErrEmptyTransaction = errors.New("empty transaction")

type Object struct {
	Commit  ksuid.KSUID `super:"commit"`
	Parent  ksuid.KSUID `super:"parent"`
	Actions []Action    `super:"actions"`
}

func NewObject(parent ksuid.KSUID, author, message string, meta super.Value, retries int) *Object {
	commit := ksuid.New()
	o := &Object{
		Commit: commit,
		Parent: parent,
	}
	o.append(&Commit{
		ID:      commit,
		Parent:  parent,
		Retries: uint8(retries),
		Date:    nano.Now(),
		Author:  author,
		Message: message,
		Meta:    meta,
	})
	return o
}

func NewAddsObject(parent ksuid.KSUID, retries int, author, message string, meta super.Value, objects []data.Object) *Object {
	o := NewObject(parent, author, message, meta, retries)
	for _, dataObject := range objects {
		o.append(&Add{Commit: o.Commit, Object: dataObject})
	}
	return o
}

func NewDeletesObject(parent ksuid.KSUID, retries int, author, message string, ids []ksuid.KSUID) *Object {
	o := NewObject(parent, author, message, super.Null, retries)
	for _, id := range ids {
		o.appendDelete(id)
	}
	return o
}

func NewAddVectorsObject(parent ksuid.KSUID, author, message string, ids []ksuid.KSUID, retries int) *Object {
	o := NewObject(parent, author, message, super.Null, retries)
	for _, id := range ids {
		o.appendAddVector(id)
	}
	return o
}

func NewDeleteVectorsObject(parent ksuid.KSUID, author, message string, ids []ksuid.KSUID, retries int) *Object {
	o := NewObject(parent, author, message, super.Null, retries)
	for _, id := range ids {
		o.appendDeleteVector(id)
	}
	return o
}

func (o *Object) append(action Action) {
	o.Actions = append(o.Actions, action)
}

func (o *Object) appendAdd(dataObject *data.Object) {
	o.append(&Add{Commit: o.Commit, Object: *dataObject})
}

func (o *Object) appendDelete(id ksuid.KSUID) {
	o.append(&Delete{Commit: o.Commit, ID: id})
}

func (o *Object) appendAddVector(id ksuid.KSUID) {
	o.append(&AddVector{Commit: o.Commit, ID: id})
}

func (o *Object) appendDeleteVector(id ksuid.KSUID) {
	o.append(&DeleteVector{Commit: o.Commit, ID: id})
}

func (o Object) Serialize() ([]byte, error) {
	writer := bsupbytes.NewSerializer()
	writer.Decorate(sup.StylePackage)
	for _, action := range o.Actions {
		if err := writer.Write(action); err != nil {
			writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	b := writer.Bytes()
	if len(b) == 0 {
		return nil, ErrEmptyTransaction
	}
	return b, nil
}

func DecodeObject(r io.Reader) (*Object, error) {
	o := &Object{}
	reader := bsupbytes.NewDeserializer(r, ActionTypes)
	defer reader.Close()
	for {
		entry, err := reader.Read()
		if err != nil {
			return nil, err
		}
		if entry == nil {
			break
		}
		action, ok := entry.(Action)
		if !ok {
			return nil, badObject(entry)
		}
		o.append(action)
	}
	// Fill in the commit and parent IDs from the first record,
	// which must always be a Commit action.
	if len(o.Actions) > 0 {
		first, ok := o.Actions[0].(*Commit)
		if !ok {
			return nil, ErrBadCommitObject
		}
		o.Commit = first.ID
		o.Parent = first.Parent
	}
	return o, nil
}

func badObject(entry any) error {
	return fmt.Errorf("internal error: corrupt commit object has unknown entry type %T", entry)
}

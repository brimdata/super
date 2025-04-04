package commits

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/lake/data"
	"github.com/brimdata/super/pkg/nano"
	"github.com/segmentio/ksuid"
)

type Action interface {
	CommitID() ksuid.KSUID
	fmt.Stringer
}

var ActionTypes = []interface{}{
	Add{},
	AddVector{},
	Delete{},
	DeleteVector{},
	Commit{},
}

type Add struct {
	Commit ksuid.KSUID `super:"commit"`
	Object data.Object `super:"object"`
}

var _ Action = (*Add)(nil)

func (a *Add) CommitID() ksuid.KSUID {
	return a.Commit
}

func (a *Add) String() string {
	return fmt.Sprintf("ADD %s", a.Object)
}

// Note that we store the number of retries in the final commit
// object.  This will allow easily introspection of optimistic
// locking problems under high commit load by simply issuing
// a meta-query and looking at the retry count in the persisted
// commit objects.  If/when this is a problem, we could add
// pessimistic locking mechanisms alongside the optimistic approach.

type Commit struct {
	ID      ksuid.KSUID `super:"id"`
	Parent  ksuid.KSUID `super:"parent"`
	Retries uint8       `super:"retries"`
	Author  string      `super:"author"`
	Date    nano.Ts     `super:"date"`
	Message string      `super:"message"`
	Meta    super.Value `super:"meta"`
}

func (c *Commit) CommitID() ksuid.KSUID {
	return c.ID
}

func (c *Commit) String() string {
	//XXX need to format Message field for single line
	return fmt.Sprintf("COMMIT %s -> %s %s %s %s", c.ID, c.Parent, c.Date, c.Author, c.Message)
}

type Delete struct {
	Commit ksuid.KSUID `super:"commit"`
	ID     ksuid.KSUID `super:"id"`
}

func (d *Delete) CommitID() ksuid.KSUID {
	return d.Commit
}

func (d *Delete) String() string {
	return "DEL " + d.ID.String()
}

type AddVector struct {
	Commit ksuid.KSUID `super:"commit"`
	ID     ksuid.KSUID `super:"id"`
}

func (a *AddVector) String() string {
	return fmt.Sprintf("ADD_VECTOR %s", a.ID)
}

func (a *AddVector) CommitID() ksuid.KSUID {
	return a.Commit
}

type DeleteVector struct {
	Commit ksuid.KSUID `super:"commit"`
	ID     ksuid.KSUID `super:"id"`
}

func (d *DeleteVector) String() string {
	return fmt.Sprintf("DEL_VECTOR %s", d.ID)
}

func (d *DeleteVector) CommitID() ksuid.KSUID {
	return d.Commit
}

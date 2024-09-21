package compiler

import (
	"errors"

	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/ast/dag"
	"github.com/brimdata/super/compiler/data"
	"github.com/brimdata/super/compiler/kernel"
	"github.com/brimdata/super/compiler/optimizer"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/compiler/semantic"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/runtime/vam"
	vamop "github.com/brimdata/super/runtime/vam/op"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
)

type Job struct {
	rctx      *runtime.Context
	builder   *kernel.Builder
	optimizer *optimizer.Optimizer
	outputs   map[string]zbuf.Puller
	puller    zbuf.Puller
	entry     dag.Seq
}

func NewJob(rctx *runtime.Context, in ast.Seq, src *data.Source, head *lakeparse.Commitish) (*Job, error) {
	seq := ast.CopySeq(in)
	// An AST always begins with a Sequential op with at least one
	// operator.  If the first op is a From or a Parallel whose branches
	// are Sequentials with a leading From, then we presume there is
	// no externally defined input.  Otherwise, we expect two readers
	// to be defined for a Join and one reader for anything else.  When input
	// is expected like this, we set up one or two readers inside of an
	// automatically inserted From.  These readers can be accessed by the
	// caller via runtime.readers.  In most cases, the AST is left
	// with an ast.From at the entry point, and hence a dag.From for the
	// DAG's entry point.
	if len(seq) == 0 {
		return nil, errors.New("internal error: AST seq cannot be empty")
	}
	entry, err := semantic.AnalyzeAddSource(rctx.Context, seq, src, head)
	if err != nil {
		return nil, err
	}
	return &Job{
		rctx:      rctx,
		builder:   kernel.NewBuilder(rctx, src),
		optimizer: optimizer.New(rctx.Context, src),
		entry:     entry,
	}, nil
}

func (j *Job) DefaultScan() (*dag.DefaultScan, bool) {
	scan, ok := j.entry[0].(*dag.DefaultScan)
	return scan, ok
}

func (j *Job) Entry() dag.Seq {
	//XXX need to prepend consts depending on context
	return j.entry
}

// This must be called before the zbuf.Filter interface will work.
func (j *Job) Optimize() error {
	var err error
	j.entry, err = j.optimizer.Optimize(j.entry)
	return err
}

func (j *Job) OptimizeDeleter(replicas int) error {
	var err error
	j.entry, err = j.optimizer.OptimizeDeleter(j.entry, replicas)
	return err
}

func (j *Job) Parallelize(n int) error {
	var err error
	j.entry, err = j.optimizer.Parallelize(j.entry, n)
	if err != nil {
		return err
	}
	j.entry, err = j.optimizer.Vectorize(j.entry)
	return err
}

func Parse(sql bool, src string, filenames ...string) (ast.Seq, *parser.SourceSet, error) {
	return parser.ParseSuperPipe(sql, filenames, src)
}

// MustParse is like Parse but panics if an error is encountered.
func MustParse(sql bool, query string) ast.Seq {
	seq, _, err := (*anyCompiler)(nil).Parse(sql, query)
	if err != nil {
		panic(err)
	}
	return seq
}

func (j *Job) Builder() *kernel.Builder {
	return j.builder
}

// Build builds a flowgraph for j.  If the flowgraph expects an input stream
// from the runtime (because its DAG contains a dag.DefaultSource), Build
// constructs it from readers.
func (j *Job) Build(readers ...zio.Reader) error {
	outputs, err := j.builder.Build(j.entry, readers...)
	if err != nil {
		return err
	}
	j.outputs = outputs
	return nil
}

func (j *Job) Puller() zbuf.Puller {
	if j.puller == nil {
		switch outputs := j.outputs; len(outputs) {
		case 0:
			return nil
		case 1:
			for k, p := range outputs {
				j.puller = op.NewCatcher(op.NewSingle(k, p))
			}
		default:
			j.puller = op.NewMux(j.rctx, outputs)
		}
	}
	return j.puller
}

type anyCompiler struct{}

// Parse concatenates the source files in filenames followed by src and parses
// the resulting program.
func (*anyCompiler) Parse(sql bool, src string, filenames ...string) (ast.Seq, *parser.SourceSet, error) {
	return Parse(sql, src, filenames...)
}

// VectorCompile is used for testing queries over single VNG object scans
// where the entire query is vectorizable.  It does not call optimize
// nor does it compute the demand of the query to prune the projection
// from the vcache.
func VectorCompile(rctx *runtime.Context, sql bool, query string, object *vcache.Object) (zbuf.Puller, error) {
	seq, sset, err := Parse(sql, query)
	if err != nil {
		return nil, err
	}
	src := &data.Source{}
	entry, err := semantic.Analyze(rctx.Context, seq, src, nil)
	if err != nil {
		if list := (parser.ErrorList)(nil); errors.As(err, &list) {
			list.SetSourceSet(sset)
		}
		return nil, err
	}
	puller := vam.NewVectorProjection(rctx.Zctx, object, nil) //XXX project all
	builder := kernel.NewBuilder(rctx, src)
	outputs, err := builder.BuildWithPuller(entry, puller)
	if err != nil {
		return nil, err
	}
	if len(outputs) == 1 {
		puller = outputs[0]
	} else {
		puller = vamop.NewCombine(rctx, outputs)
	}
	return vam.NewMaterializer(puller), nil
}

func VectorFilterCompile(rctx *runtime.Context, sql bool, query string, src *data.Source, head *lakeparse.Commitish) (zbuf.Puller, error) {
	// Eventually the semantic analyzer + kernel will resolve the pool but
	// for now just do this manually.
	if !src.IsLake() {
		return nil, errors.New("non-lake searches not supported")
	}
	poolID, err := src.PoolID(rctx.Context, head.Pool)
	if err != nil {
		return nil, err
	}
	commitID, err := src.CommitObject(rctx.Context, poolID, head.Branch)
	if err != nil {
		return nil, err
	}
	seq, sset, err := Parse(sql, query)
	if err != nil {
		return nil, err
	}
	entry, err := semantic.Analyze(rctx.Context, seq, src, nil)
	if err != nil {
		if list := (parser.ErrorList)(nil); errors.As(err, &list) {
			list.SetSourceSet(sset)
		}
		return nil, err
	}
	if len(entry) != 2 {
		return nil, errors.New("filter query must have a single op")
	}
	f, ok := entry[0].(*dag.Filter)
	if !ok {
		return nil, errors.New("filter query must be a single filter op")
	}
	return kernel.NewBuilder(rctx, src).BuildVamToSeqFilter(f.Expr, poolID, commitID)
}

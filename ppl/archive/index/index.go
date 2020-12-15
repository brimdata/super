package index

import (
	"context"
	"fmt"
	"regexp"

	"github.com/brimsec/zq/microindex"
	"github.com/brimsec/zq/pkg/iosrc"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/brimsec/zq/zqe"
	"github.com/segmentio/ksuid"
)

type Index struct {
	Definition *Definition
	Dir        iosrc.URI
}

func (i Index) Path() iosrc.URI {
	return IndexPath(iosrc.URI(i.Dir), i.Definition.ID)
}

func IndexPath(dir iosrc.URI, id ksuid.KSUID) iosrc.URI {
	return dir.AppendPath(indexFilename(id))
}

func (i Index) Filename() string {
	return indexFilename(i.Definition.ID)
}

func ListDefinitionIDs(ctx context.Context, d iosrc.URI) ([]ksuid.KSUID, error) {
	infos, err := infos(ctx, d)
	if err != nil {
		return nil, err
	}
	var indices []ksuid.KSUID
	for _, info := range infos {
		if uuid, err := parseIndexFile(info.Name()); err == nil {
			indices = append(indices, uuid)
		}
	}
	return indices, nil
}

func ListFilenames(ctx context.Context, d iosrc.URI) ([]string, error) {
	infos, err := infos(ctx, d)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, info := range infos {
		if _, err := parseIndexFile(info.Name()); err != nil {
			files = append(files, info.Name())
		}
	}
	return files, nil
}

func Indices(ctx context.Context, d iosrc.URI, m DefinitionMap) ([]Index, error) {
	ids, err := ListDefinitionIDs(ctx, d)
	if err != nil {
		return nil, err
	}
	var indices []Index
	for _, id := range ids {
		if def, ok := m[id]; ok {
			indices = append(indices, Index{def, d})
		}
	}
	return indices, nil
}

func ApplyDefinitions(ctx context.Context, d iosrc.URI, r zbuf.Reader, defs ...*Definition) ([]Index, error) {
	writers, err := NewMultiWriter(ctx, d, defs)
	if err != nil {
		return nil, err
	}
	if len(writers) == 0 {
		return nil, nil
	}
	if err := zbuf.CopyWithContext(ctx, writers, r); err != nil {
		writers.Abort()
		return nil, err
	}
	if err := writers.Close(); err != nil {
		return nil, err
	}
	return writers.Indices(), nil
}

func Find(ctx context.Context, d iosrc.URI, id ksuid.KSUID, patterns ...string) (*zng.Record, error) {
	return FindFromPath(ctx, IndexPath(d, id), patterns...)
}

func FindFromPath(ctx context.Context, idxfile iosrc.URI, patterns ...string) (*zng.Record, error) {
	finder, err := microindex.NewFinder(ctx, resolver.NewContext(), idxfile)
	if err != nil {
		return nil, fmt.Errorf("index find %s: %w", idxfile, err)
	}
	defer finder.Close()
	keys, err := finder.ParseKeys(patterns...)
	if err != nil {
		return nil, fmt.Errorf("index find %s: %w", finder.Path(), err)
	}
	rec, err := finder.Lookup(keys)
	if err != nil {
		return nil, fmt.Errorf("index find %s: %w", finder.Path(), err)
	}
	return rec, nil
}

func indexFilename(id ksuid.KSUID) string {
	return fmt.Sprintf("idx-%s.zng", id)
}

var indexFileRegex = regexp.MustCompile(`idx-([0-9A-Za-z]{27}).zng$`)

func parseIndexFile(name string) (ksuid.KSUID, error) {
	match := indexFileRegex.FindStringSubmatch(name)
	if match == nil {
		return ksuid.Nil, fmt.Errorf("invalid index file: %s", name)
	}
	return ksuid.Parse(match[1])
}

func mkdir(d iosrc.URI) error {
	return iosrc.MkdirAll(d, 0700)
}

func infos(ctx context.Context, d iosrc.URI) ([]iosrc.Info, error) {
	infos, err := iosrc.ReadDir(ctx, d)
	if zqe.IsNotFound(err) {
		return nil, mkdir(d)
	}
	return infos, err
}

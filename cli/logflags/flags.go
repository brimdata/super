package logflags

import (
	"flag"

	"github.com/superdb/super/service/logger"
	"go.uber.org/zap"
)

type Flags struct {
	Config logger.Config
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&f.Config.DevMode, "log.devmode", false, "development mode (if enabled dpanic level logs will cause a panic)")
	f.Config.Level = zap.InfoLevel
	fs.Var(&f.Config.Level, "log.level", "logging level")
	fs.StringVar(&f.Config.Path, "log.path", "stderr", "path to send logs (values: stderr, stdout, path in file system)")
	f.Config.Mode = logger.FileModeTruncate
	fs.Var(&f.Config.Mode, "log.filemode", "logger file write mode (values: append, truncate, rotate)")
}

func (f *Flags) Open() (*zap.Logger, error) {
	return logger.New(f.Config)
}

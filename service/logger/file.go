package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/brimdata/super/pkg/fs"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type FileMode string

func (m *FileMode) Set(s string) error {
	switch FileMode(s) {
	case FileModeAppend, "":
		*m = FileModeAppend
	case FileModeTruncate:
		*m = FileModeTruncate
	case FileModeRotate:
		*m = FileModeRotate
	default:
		if s == "" {
			*m = FileModeTruncate
			break
		}
		return fmt.Errorf("invalid FileMode type: %s", s)
	}
	return nil
}

func (m FileMode) String() string {
	return string(m)
}

const (
	// FileModeAppend will append to existing log files between restarts.
	// This is the default option.
	FileModeAppend FileMode = "append"
	// FileModeTruncate will truncate onto existing log files in between
	// restarts.
	FileModeTruncate FileMode = "truncate"
	// FileModeRotate will enable log rotation for log files.
	FileModeRotate FileMode = "rotate"
)

func OpenFile(path string, mode FileMode) (zapcore.WriteSyncer, error) {
	switch path {
	case "stdout":
		return zapcore.Lock(os.Stdout), nil
	case "stderr":
		return zapcore.Lock(os.Stderr), nil
	case "/dev/null":
		return zapcore.AddSync(io.Discard), nil
	}
	switch mode {
	case FileModeRotate:
		return logrotate(path)
	case FileModeTruncate:
		return fs.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	default: // FileModeAppend
		return fs.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	}
}

func logrotate(path string) (zapcore.WriteSyncer, error) {
	// Make sure directory exists
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		return nil, err
	}
	// lumberjack.Logger is already safe for concurrent use, so we don't need to
	// lock it.
	return zapcore.AddSync(&lumberjack.Logger{
		Filename:   path,
		MaxSize:    5, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	}), nil
}

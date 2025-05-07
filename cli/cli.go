package cli

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"syscall"
)

type Flags struct {
	showVersion    bool
	cpuprofile     string
	memprofile     string
	trace          string
	cpuProfileFile *os.File
	traceFile      *os.File
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&f.showVersion, "version", false, "print version and exit")
	fs.StringVar(&f.cpuprofile, "cpuprofile", "", "write cpu profile to given file name")
	fs.StringVar(&f.memprofile, "memprofile", "", "write memory profile to given file name")
	fs.StringVar(&f.trace, "trace", "", "write trace to given file name")
}

type Initializer interface {
	Init() error
}

// Init is equivalent to InitWithSignals with SIGINT, SIGPIPE, and SIGTERM.
func (f *Flags) Init(all ...Initializer) (context.Context, context.CancelFunc, error) {
	return f.InitWithSignals(all, syscall.SIGINT, syscall.SIGPIPE, syscall.SIGTERM)
}

// InitWithSignals handles the flags defined in SetFlags, calls the Init method
// for each element of all, and returns a context canceled when any signal in
// signals is raised.
func (f *Flags) InitWithSignals(all []Initializer, signals ...os.Signal) (context.Context, context.CancelFunc, error) {
	if f.showVersion {
		fmt.Printf("Version: %s\n", Version())
		os.Exit(0)
	}
	var err error
	for _, flags := range all {
		if initErr := flags.Init(); err == nil {
			err = initErr
		}
	}
	if err != nil {
		return nil, nil, err
	}
	if f.cpuprofile != "" {
		f.runCPUProfile(f.cpuprofile)
	}
	if f.trace != "" {
		f.traceFile, err = os.Create(f.trace)
		if err != nil {
			log.Fatal(err)
		}
		trace.Start(f.traceFile)
	}
	ctx, cancel := signalContext(context.Background(), signals...)
	cleanup := func() {
		cancel()
		f.cleanup()
	}
	return ctx, cleanup, nil
}

func signalContext(ctx context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(ctx)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)
	go func() {
		select {
		case <-ctx.Done():
		case s := <-ch:
			cancel(fmt.Errorf("received %s signal", s))
		}
	}()
	return ctx, func() {
		cancel(nil)
		signal.Stop(ch)
	}
}

func (f *Flags) cleanup() {
	if f.cpuProfileFile != nil {
		pprof.StopCPUProfile()
		f.cpuProfileFile.Close()
	}
	if f.memprofile != "" {
		runMemProfile(f.memprofile)
	}
	if f.traceFile != nil {
		trace.Stop()
		f.traceFile.Close()
	}
}

func (f *Flags) runCPUProfile(path string) {
	file, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	f.cpuProfileFile = file
	pprof.StartCPUProfile(file)
}

func runMemProfile(path string) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	runtime.GC()
	pprof.Lookup("allocs").WriteTo(f, 0)
	f.Close()
}

func FileExists(path string) bool {
	if path == "-" {
		return true
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

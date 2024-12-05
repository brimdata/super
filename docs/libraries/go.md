---
weight: 1
title: Go
---

The Zed system was developed in Go so support for Go clients is
fairly comprehensive.  That said, the code-embedded documentation of exported
package functions is scant and we are actively working to document
the functions of the key Go packages.

Also, our focus for the Go client packages has been on supporting
the core Zed implementation.  We intend to develop a Go package that
is easier to use for external clients.  In the meantime, clients
may use the internal Go packages though the APIs are subject to change.

## Installation

The Zed system is structured as a standard Go module so it's easy to import into
other Go projects straight from the GitHub repo.

Some of the key packages are:

* [zed](https://pkg.go.dev/github.com/brimdata/super) - core Zed values and types
* [zson](https://pkg.go.dev/github.com/brimdata/super/zson) - ZSON support
* [zio](https://pkg.go.dev/github.com/brimdata/super/zio) - I/O interfaces for Zed following the Reader/Writer patterns
* [zio/zsonio](https://pkg.go.dev/github.com/brimdata/super/zio/zsonio) - ZSON reader/writer
* [zio/zngio](https://pkg.go.dev/github.com/brimdata/super/zio/zngio) - ZNG reader/writer
* [lake/api](https://pkg.go.dev/github.com/brimdata/super/lake/api) - interact with a Zed lake

To install in your local Go project, simply run:
```
go get github.com/brimdata/super
```

## Examples

### ZSON Reader

Read ZSON from stdin, dereference field `s`, and print results:
```mdtest-go-example
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio/zsonio"
	"github.com/brimdata/super/zson"
)

func main() {
	zctx := super.NewContext()
	reader := zsonio.NewReader(zctx, os.Stdin)
	for {
		val, err := reader.Read()
		if err != nil {
			log.Fatalln(err)
		}
		if val == nil {
			return
		}
		s := val.Deref("s")
		if s == nil {
			s = zctx.Missing().Ptr()
		}
		fmt.Println(zson.String(s))
	}
}
```
To build, create a directory for the main package, initialize it,
copy the above code into `main.go`, and fetch the required Zed packages.
```
mkdir example
cd example
go mod init example
cat > main.go
# [paste from above]
go mod tidy
```
To run type:
```
echo '{s:"hello"}{x:123}{s:"world"}' | go run .
```
which produces
```
"hello"
error("missing")
"world"
```

### Local Lake Reader

This example interacts with a Zed lake.  Note that it is straightforward
to support both direct access to a lake via the file system (or S3 URL) as well
as access via a service endpoint.

First, we'll use `zed` to create a lake and load the example data:
```
zed init -lake scratch
zed create -lake scratch Demo
echo '{s:"hello, world"}{x:1}{s:"good bye"}' | zed load -lake scratch -use Demo -
```
Now replace `main.go` with this code:
```mdtest-go-example
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/brimdata/super"
	"github.com/brimdata/super/lake/api"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zson"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("URI of Zed lake not provided")
	}
	uri, err := storage.ParseURI(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}
	ctx := context.TODO()
	lake, err := api.OpenLake(ctx, nil, uri.String())
	if err != nil {
		log.Fatalln(err)
	}
	q, err := lake.Query(ctx, "from Demo")
	if err != nil {
		log.Fatalln(err)
	}
	defer q.Pull(true)
	reader := zbuf.PullerReader(q)
	zctx := super.NewContext()
	for {
		val, err := reader.Read()
		if err != nil {
			log.Fatalln(err)
		}
		if val == nil {
			return
		}
		s := val.Deref("s")
		if s == nil {
			s = zctx.Missing().Ptr()
		}
		fmt.Println(zson.String(s))
	}
}
```
After a re-run of `go mod tidy`, run this command to interact with the lake via
the local file system:
```
go run . ./scratch
```
which should output
```
"hello, world"
"good bye"
error("missing")
```
Note that the order of data has changed because the Zed lake stores data
in a sorted order.  Since we did not specify a "pool key" when we created
the lake, it ends up sorting the data by `this`.

### Lake Service Reader

We can use the same code above to talk to a Zed lake server.  All we do is
give it the URI of the service, which by default is on port 9867.

To try this out, first run a Zed service on the scratch lake we created
above:
```
zed serve -lake ./scratch
```
Finally, in another local shell, run the Go program and specify the service
endpoint we just created:
```
go run . http://localhost:9867
```
and you should again get this result:
```
"hello, world"
"good bye"
error("missing")
```

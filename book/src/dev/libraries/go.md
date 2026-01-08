## Go

SuperDB is developed in Go so support for Go clients is
fairly comprehensive.  Documentation of exported
package functions is fairly scant though we plan to improve it soon.

Also, our focus for the Go client packages has been on supporting
the interal APIs in the SuperDB implementation.  We intend to develop
a Go package that is easier to use for external clients.  In the meantime,
clients may use the internal Go packages though the APIs are subject to change.

## Installation

SuperDB is structured as a standard Go module so it's easy to import into
other Go projects straight from the GitHub repo.

Some of the key packages are:

* [super](https://pkg.go.dev/github.com/brimdata/super) - Super values and types
* [sup](https://pkg.go.dev/github.com/brimdata/super/sup) - SUP support
* [sio](https://pkg.go.dev/github.com/brimdata/super/sio) - I/O interfaces for Super data following the Reader/Writer patterns
* [sio/bsupio](https://pkg.go.dev/github.com/brimdata/super/sio/bsupio) - BSUP reader/writer
* [sio/supio](https://pkg.go.dev/github.com/brimdata/super/sio/supio) - SUP reader/writer
* [db/api](https://pkg.go.dev/github.com/brimdata/super/db/api) - interact with a SuperDB database

To install in your local Go project, simply run:
```
go get github.com/brimdata/super
```

## Examples

### SUP Reader

Read SUP from stdin, dereference field `s`, and print results:
```mdtest-go-example
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sio/supio"
	"github.com/brimdata/super/sup"
)

func main() {
	sctx := super.NewContext()
	reader := supio.NewReader(sctx, os.Stdin)
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
			s = sctx.Missing().Ptr()
		}
		fmt.Println(sup.String(s))
	}
}
```
To build, create a directory for the main package, initialize it,
copy the above code into `main.go`, and fetch the required Super packages.
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

### Local Database Reader

This example interacts with a SuperDB database.  Note that it is straightforward
to support both direct access to a lake via the file system (or S3 URL) as well
as access via a service endpoint.

First, we'll use `super` to create a lake and load the example data:
```
super db init -db scratch
super db create -db scratch Demo
echo '{s:"hello, world"}{x:1}{s:"good bye"}' | super db load -db scratch -use Demo -
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
	"github.com/brimdata/super/compiler/srcfiles"
	"github.com/brimdata/super/db/api"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/sbuf"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("URI of database not provided")
	}
	uri, err := storage.ParseURI(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}
	ctx := context.TODO()
	db, err := api.Connect(ctx, nil, uri.String())
	if err != nil {
		log.Fatalln(err)
	}
	q, err := db.Query(ctx, srcfiles.Plain("from Demo"))
	if err != nil {
		log.Fatalln(err)
	}
	defer q.Pull(true)
	reader := sbuf.PullerReader(q)
	sctx := super.NewContext()
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
			s = sctx.Missing().Ptr()
		}
		fmt.Println(sup.String(s))
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
Note that the order of data has changed because the database stores data
in a sorted order.  Since we did not specify a "pool key" when we created
the lake, it ends up sorting the data by `this`.

### Database Service Reader

We can use the same code above to talk to a SuperDB servuce.  All we do is
give it the URI of the service, which by default is on port 9867.

To try this out, first run a SuperDB service on the scratch lake we created
above:
```
super db serve -db ./scratch
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

### Command

&emsp; **serve** &mdash; run a SuperDB service endpoint

### Synopsis

```
super db serve [options]
```

### Options

* `-auth.audience` [Auth0](https://auth0.com/) audience for API clients (will be publicly accessible)
* `-auth.clientid` [Auth0](https://auth0.com/) client ID for API clients (will be publicly accessible)
* `-auth.domain` [Auth0](https://auth0.com/) domain (as a URL) for API clients (will be publicly accessible)
* `-auth.enabled` enable authentication checks
* `-auth.jwkspath` path to JSON Web Key Set file
* `-cors.origin` CORS allowed origin (may be repeated)
* `-defaultfmt` default response format (default "sup")
* `-l [addr]:port` to listen on (default ":9867")
* `-log.devmode` development mode (if enabled dpanic level logs will cause a panic)
* `-log.filemod` logger file write mode (values: append, truncate, rotate)
* `-log.level` logging level
* `-log.path` path to send logs (values: stderr, stdout, path in file system)
* `-manage` when positive, run lake maintenance tasks at this interval
* `-rootcontentfile` file to serve for GET /

Additional options of the [db sub-command](db.md#options)

### Description

TODO: get rid of personality metaphor?

The `serve` command implements the
[server personality](../database/intro.md#command-personalities) to service requests
from instances of the client personality.
It listens for [API](../database/api.md) requests on the interface and port
specified by the `-l` option, executes the requests, and returns results.

The `-log.level` option controls log verbosity.  Available levels, ordered
from most to least verbose, are `debug`, `info` (the default), `warn`,
`error`, `dpanic`, `panic`, and `fatal`.  If the volume of logging output at
the default `info` level seems too excessive for production use, `warn` level
is recommended.

The `-manage` option enables the running of the same maintenance tasks
normally performed via the separate [`manage`](db-manage.md) command.

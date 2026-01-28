# Hello World

To test out the [installed](install.md) `super` binary, try running these
"Hello World" examples.

## Stdin to Stdout

First, here is a Unix-y version that simply reads from standard input.
Copy this one liner to your shell and run it:
```
echo '"hello, world"' | super -
```
You should get:
```
"hello, world"
```
In this simple case,
there is no query argument specified for `super` (i.e., no `-c` argument), which causes
`super` to presume an implied [from](../super-sql/operators/from.md) operator.
This `from` operator scans each of the command-line arguments
interpreted as file paths or URLs (or `-` for standard input).

In this case, the input is read from the implied operator, no further query
is applied, and the results are emitted to standard output.
This results in the string value `"hello, world"`,
serialized in the default [SUP format](../formats/sup.md),
which is simply the string literal itself.

## SQL Version

A SQL version of Hello World is:
```
super -c "SELECT 'hello, world' as Message"
```
which outputs
```
{Message:"hello, world"}
```
This is a single row in a table with one column called `Message` of type `string`.

## SuperDB Database

The top-level `super` command runs without any underlying persistent database,
but you can also run Hello World with a database.

To create a database and populate it with data, run the following commands:
```
export SUPER_DB=./scratch
super db init
super db create Demo
echo '{Message:"hello, world"}' | super db load -use Demo -
```
Now you have a database with a data pool called "Demo" and some data in it.
Query this data as follow:
```
super db -c "from Demo"
```
and you should see
```
{Message:"hello, world"}
```

## SuperDB Service

With your database in the `./scratch` directory, you can also
run Hello World as a client talking to a SuperDB server instance.
Continuing the example above (with the `SUPER_DB` environment pointing to `./scratch`),
run a service as follows:
```
super db serve
```
This command will block and output logging information to standard output.

In another window (without `SUPER_DB` defined), run `super` as a client talking
to the service input as follows:
```
super db -c "from Demo"
```

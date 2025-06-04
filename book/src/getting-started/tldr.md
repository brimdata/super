## TL;DR

Don't have time to dive into the documentation?

Just skim these one liners to get the gist of what SuperDB can do!

Note that JSON files can include any sequence of JSON values like 
[newline-deliminted JSON](https://github.com/ndjson/ndjson-spec)
though the values need not be newline deliminated.

### Query a CSV, JSON, or Parquet file using SuperSQL
```
super -c "SELECT * FROM file.[csv|csv.gz|json|json.gz|parquet]"
```
### Run a SuperSQL query sourced from an input file
```
super -I path/to/query.sql
```
### Compute a histogram of the "data shapes" in a JSON file
```
super -c "count() by typeof(this)" file.json
```
### Display a sample value of each "shape" of JSON data
```
super -c "any(this) by typeof(this) | yield any" file.json
```
### Read a CSV from stdin, process with a query, and write to stdout
```
cat input.csv | super -f csv -c <query> -
```
### Fuse JSON data into a unified schema and output as Parquet
```
super -f parquet -o out.parquet -c fuse file.json 
```
### Run as a calculator
```
super -c "1.+(1/2.)+(1/3.)+(1/4.)"
```
### Handle and wrap errors in a SuperSQL pipeline
```
... | super -c """
switch is_error(this) (
    case true ( yield error({message:"error into stage N", on:this}) )
    default (
        <non-error processing here>
        ...
    )
)
"""
| ...
```

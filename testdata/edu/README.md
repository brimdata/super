# Educational Sample Data

This directory contains a small sample data set regarding California
schools and their average SAT scores.  It is used in query examples in
the [SuperPipe language documentation](../../docs/language/README.md).


# Acknowledgement

This data is dervied from an SQLite database from the
[Public Affairs Data Journalism](http://2016.padjo.org/tutorials/sqlite-data-starterpacks/)
website at Stanford. We express our thanks to them for publishing
this data.

# Creation

[`schools.jsup`](schools.jsup) and [`testscores.jsup`](testscores.jsup)
are created by downloading an SQLite database, extracting two tables as
JSON, and shaping and sorting the resulting records.

```sh
curl -O http://2016.padjo.org/files/data/starterpack/cde-schools/cdeschools.sqlite

sqlite3 -json cdeschools.sqlite "select * from schools;" | super -z -c '
  type school = {
    School:string,
    District:string,
    City:string,
    County:string,
    Zip:string,
    Latitude:float64,
    Longitude:float64,
    Magnet:bool,
    OpenDate:time,
    ClosedDate:time,
    Phone:string,
    StatusType:string,
    Website:string
  };
  this := crop(shape(school), school) |> sort School
' - > schools.jsup

sqlite3 -json cdeschools.sqlite "select * from satscores;" | super -z -c '
  type testscore = {
    AvgScrMath: uint16,
    AvgScrRead: uint16,
    AvgScrWrite: uint16,
    cname: string,
    dname: string,
    sname: string
  };
  this := crop(shape(testscore), testscore) |> sort sname
' - > testscores.jsup
```

Some SuperPipe language examples require IP address data, so the data set is
augmented with [`webaddrs.jsup`](webaddrs.jsup), which captures an IP
address at which each school website was once hosted.

```sh
for host in $(zq -f text 'Website != null |> by Website' schools.jsup | sed -e 's|http://||' -e 's|/.*||' | sort -u); do
  addr=$(dig +short $host |> egrep '\d{1,3}(.\d{1,3}){3}' | tail -1)
  [ "$addr" ] &&
    echo "{Website:\"$host\",addr:$addr}"
done > webaddrs.jsup
```

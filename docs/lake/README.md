# Zed lake overview

> DISCLAIMER: "ZED LAKE" IS A CURRENTLY A PROTOTYPE UNDER DEVELOPMENT AND IS
> CHANGING QUICKLY.  PLEASE EXPERIMENT WITH IT AND GIVE US FEEDBACK BUT
> IT'S NOT QUITE READY FOR PRODUCTION USE. THE SYNTAX/OPTIONS/OUTPUT ETC
> ARE ALL SUBJECT TO CHANGE.

---

> A video walk-through of some of what's shown below is captured in
> [this excerpt from a recent Zeek From Home event](https://www.youtube.com/watch?v=ldrEadAQYTM&t=46m00s).

---

This documents describes the `zed lake` command, a work-in-progress for
indexing and searching Zed data lakes.

# Contents

  * [Test data](#test-data)
  * [Ingesting the data](#ingesting-the-data)
  * [Initializing the archive](#initializing-the-archive)
  * [Counting as "hello world"](#counting-as-hello-world)
  * [Search for an IP](#search-for-an-ip)
  * [Indexes](#indexes)
  * [Creating more indexes](#creating-more-indexes)
  * [Operating directly on indexes](#operating-directly-on-indexes)
  * [Custom indexes: Storing aggregations in an index](#custom-indexes-storing-aggregations-in-an-index)
  * [`zed lake find` with custom index](#zed-lake-find-with-custom-index)
  * [Multi-key custom indexes](#multi-key-custom-indexes)
  * [Map-reduce](#map-reduce)
  * [Simple graph queries](#simple-graph-queries)
  * [A final word about pipes...](#a-final-word-about-pipes)
  * [Cleanup](#cleanup)

## Test data

We'll use the [ZNG](../formats/zng.md)-format test data from here:
```
https://github.com/brimdata/zed-sample-data/tree/main/zng
```
You can copy just the zng data directory needed for this demo
into your current directory using subversion:
```
svn checkout https://github.com/brimdata/zed-sample-data/trunk/zng
```
Or, you can clone the whole data repo using git and symlink the zng dir:
```
git clone --depth=1 https://github.com/brimdata/zed-sample-data.git
ln -s zed-sample-data/zng
```

## Ingesting the data

Let's take those logs and ingest them into a directory.  Often we'd keep our
archive somewhere like [Amazon S3](https://aws.amazon.com/s3/), but since we're
just doing a quick test we'll use local temp space.  We'll make it easier to
run all the commands by setting an environment variable pointing to the root of
the logs tree.
```
export ZED_LAKE_ROOT=/tmp/logs
mkdir $ZED_LAKE_ROOT
```

Now, let's ingest the data using `zed lake import`.  We are working on more
sophisticated ways to ingest data (e.g., arbitrary partition keys and
auto-sizing of partitions) but for now `zed lake import` just chops its input into
LZ4-compressed chunk files of approximately equal size, each sorted by
timestamp in descending order.  We'll chop into chunks of approximately 25MB
each, which is very small, but in this example the data set is fairly small
(71 MB of LZ4-compressed ZNG) and you can always try it out on larger data
sets:
```
zed lake import -s 25MB zng/*.gz
```

## Initializing the archive

Try `zed lake ls` now and you can see the lake directories.  This is where
`zed lake` puts lots of interesting data associated with each chunk file of the ingested logs.
```
zed lake ls
```

## Counting as "hello world"

Now that it's set up, you can do stuff with the archive.  Maybe the simplest thing
is to count up all the events across the archive.  Since the chunk files
are in different directories in the archive, we need a way to run `zed query` over
all of them and aggregate the result.

The `zq` subcommand of `zed lake` lets you do this.  Here's how you run `zq`
on the data of every log in the archive:
```
zed lake query "count()" > counts.zng
```
This invocation of `zed lake` traverses the lake, applies the Zed "count()" operator
on the data from all the chunks, and writes the output as a stream of zng data.
By default, the output is sent to stdout, which means you can simply pipe the
resulting stream to a vanilla `zq` command that specifies `-` to expect the
stream on stdin, then show the output as a table:
```
zed lake query "count()" | zq -f table -
```
which, for example, results in:
```
count
1462078
```

The `zq` common options are also available when invoking `zed lake query`, so instead of a
pipeline we can get the same result in one shot via:

```
zed lake query -f table "count()"
```

`zed lake query` treats the lake as if it were one large set of data, regardless
of how many chunk files are in it. It's also possible to perform queries for
each chunk; that's done with a command called `zed lake map`.
This invocation of `zed lake`
traverses the archive, applies a query to each log file, and writes the output
to either stdout, or to a new file in the chunk's directory.

Here's an example using the same "count()" query as before:

```
zed lake map -f text "count()"
```
which results in:
```
275745
276326
290789
619218
```

You could take the stream of event counts and sum them to get a total:
```
zed lake map "count()" | zq -f text "sum(count)" -
```
which should have the same result as
```
zq -f text "count()" zng/*.gz
```
or...
```
1462078
```

## Search for an IP

Now let's say you want to search for a particular IP across the Zed lake.
This is easy. You just say:
```
zed lake query -Z "id.orig_h=10.10.23.2"
```
which gives this result in the [ZSON](../formats/zson.md) format.  ZSON
describes the complete detail from the ZNG stream as human-readable text.
```
{
    _path: "conn",
    ts: 2018-03-24T17:15:21.307472Z,
    uid: "C4NuQHXpLAuXjndmi" (bstring),
    id: {
        orig_h: 10.10.23.2,
        orig_p: 11 (port=(uint16)),
        resp_h: 10.0.0.111,
        resp_p: 0 (port)
    } (=0),
    proto: "icmp" (=zenum),
    service: null (bstring),
    duration: 21m0.819589s,
    orig_bytes: 23184 (uint64),
    resp_bytes: 0 (uint64),
    conn_state: "OTH" (bstring),
    local_orig: null (bool),
    local_resp: null (bool),
    missed_bytes: 0 (uint64),
    history: null (bstring),
    orig_pkts: 828 (uint64),
    orig_ip_bytes: 46368 (uint64),
    resp_pkts: 0 (uint64),
    resp_ip_bytes: 0 (uint64),
    tunnel_parents: null (1=(|[bstring]|))
} (=2)
```
(If you want to learn more about this format, check out the
[ZSON spec](../formats/zson.md).)

You might have noticed that this is kind of slow --- like all the counting above ---
because every record is read to search for that IP.

We can speed this up by building an index.  
`zed lake` lets you pretty much build any
sort of index you'd like and you can even embed whatever custom Zed analytics
you would like in a search index.  But for now, let's look at just IP addresses.

> NOTE: we are in the process of changing `zed lake index` to operate over
> specific key ranges instead of the arbitrary boundaries formed with lake chunks.
> For now, this describes the old behavior.

The `zed lake index` command makes it easy to index any field or any Zed type.
e.g., to index every value that is of type IP, we simply say
```
zed lake index create :ip
```
For each Zed lake chunk, this command will find every field of type IP in every
record and add a key for that field's value to chunk's index file.

Hmm that was interesting.  If you type
```
zed lake ls -l
```
You will see all the indexes left behind. They are just zng files.
If you want to see one, just look at it with zq, e.g.
```
find $ZED_LAKE_ROOT -name idx-* | head -n 1 | xargs zq -z -
```
Now if you run `zed lake find`, it will efficiently look through all the index files
instead of the raw lake data and run much faster...
```
zed lake find -f table :ip=10.10.23.2
```
In the output here, you'll see this IP exists in exactly one lake chunk:
```
/tmp/logs/zd/20180324/d-1jQ2dLYVOL4NSSYCYC4P400ZfPN.zng
```

(In this and later outputs in this README that show pathnames in the archive,
the portion of the paths following `d-` will be unique and hence differ in your
output if you repeat the commands.)

## Indexes

A Zed index is a zng index file that pertains to just one
chunk of lake data and represents just one indexing rule.  If you're curious about
what's in the index, it's just a sorted list of keyed records along with some
additional zng streams that comprise a constant b-tree index into the sorted list.
But the cool thing here is that everything is just a zng stream.

Instead of building a massive, inverted index with glorious roaring
bitmaps that tell you exactly where each event is in the event store, our model
is to instead build lots of small indexes for each log chunk and index different
things in the different indexes.  This approach dovetails with the modern
cloud warehouse approach of scanning native-cloud storage objets and efficiently
pruning objects that are not needed from the scan.

## Creating more indexes

The beauty of this approach is that you can add and delete indexes
whenever you want.  No need to suffer the fate of a massive reindexing
job when you have a new idea about what to index.

So, let's say you later decide you want searches over the "uri" field to run fast.
You just run `zed lake index` again but with different parameters:
```
zed lake index create uri
```
And now you can run field matches on `uri`:
```
zed lake find -f table uri=/file
```
and you'll find "hits" in multiple lake chunks:
```
/tmp/logs/zd/20180324/d-1jQ2d5DwDHJULCRN6gq84IwArbb.zng
/tmp/logs/zd/20180324/d-1jQ2co6Ttjk9wEUdzI2yW7koYtB.zng
```

## Operating directly on indexes

Let's say instead of searching for what lake chunk a value is in, we want to
actually pull out the zng records that comprise the index.  This turns out
to be really powerful in general, but to give you a taste here, you can say...
```
zed lake find -z :ip=10.47.21.138
```
where `-z` says to produce compact ZSON output instead of a table,
and you'll get this...
```
{key:10.47.21.138,count:1 (uint64),_log:"/tmp/logs/zd/20180324/d-1qCzy6mfDLtsDXeEU1EJxSn1DTi.zng" (=zfile),first:2018-03-24T17:36:30.01359Z,last:2018-03-24T17:15:20.600725Z} (=0)
{key:10.47.21.138,count:13,_log:"/tmp/logs/zd/20180324/d-1qCzyJtVcZh8fLyFgTJl0fcoyrp.zng",first:2018-03-24T17:29:56.0241Z,last:2018-03-24T17:15:20.601374Z} (0)
```
The find command adds a column called "_log" (which can be disabled
or customized to a different field name) so you can see where the
search hits came from even when they are combined into a zng stream.
The type of the path field is a "zng alias" --- a sort of logical type ---
where a client can infer the type "zfile" refers to a zng data file.

But, what if we wanted to put even more information in the index
alongside each key?  If we could, it seems we could do arbitrarily
interesting things with this...

## Custom indexes: Storing aggregations in an index

Since everything is a zng file, you can create whatever values you want to
go along with your index keys using Zed queries.  Why don't we go back to counting?

Let's create an index keyed on the field id.orig_h and for each unique value of
this key, we'll compute the number of times that value appeared for each zeek
log type.  To do this, we'll run `zed lake index` in a way that leaves
these results behind in each lake directory:
```
zed lake index create -q -o custom.zng -k id.orig_h -z "count() by _path, id.orig_h | sort id.orig_h"
```
Unlike for the field and type indexes we created previously, for
custom indexes the index file name must be specified via the `-o`
flag.  You can run ls to see the custom index files are indeed there:
```
zed lake ls custom.zng
```
To see what's in it:
```
find $ZED_LAKE_ROOT -name idx-$(zed lake index ls -f zng | zq -f text 'desc="zql-custom.zng" | cut id' -).zng | head -n 1 | xargs zq -f table 'head 10' -
```
You can see the IPs, counts, and _path strings.

At the bottom you'll also find a record describing the index layout. To
see it:

```
find $ZED_LAKE_ROOT -name idx-$(zed lake index ls -f zng | zq -f text 'desc="zql-custom.zng" | cut id' -).zng | head -n 1 | xargs zq -f table 'tail 1' -
```

## `zed lake find` with custom index

And now I can go back to my example from before and use `zed lake find` on the custom
index:
```
zed lake find -z -x custom.zng 10.164.94.120
```
Now we're talking!  And if you take the results and do a little more math to
aggregate the aggregations, like this:
```
zed lake find -x custom.zng 10.164.94.120 | zq -f table "count=sum(count) by _path | sort -r" -
```
You'll get
```
_path       count
conn        26726
http        13485
ssl         9538
rdp         4116
smtp        1178
weird       316
ftp         93
ntlm        80
smb_mapping 65
notice      35
dpd         24
dns         8
rfb         3
dce_rpc     2
ssh         1
smb_files   1
```
We can compute this aggregation now for any IP in the index
without reading any of the original data files!  You'll get the same
output from this...
```
zq "id.orig_h=10.164.94.120" zng/*.gz | zq -f table "count() by _path | sort -r" -
```
But using `zed lake` with the custom indexes is MUCH faster.  Pretty cool.

## Multi-key custom indexes

In addition to a single-key search, you can build indexes with multiple keys
in each row.  To do this, you list the keys in order
of precedence, e.g., primary, secondary, etc.  Then, you can perform searches
using one or more keys in that order where any missing keys are "don't cares"
and will match all search rows with any value.

For example, let's say we want to build an index that has primary key
`id.resp_h` and secondary key `id.orig_h` from all the conn logs where we
cache the sum of response bytes to each originator.
```
zed lake index create -o custom2.zng -k id.resp_h,id.orig_h -z "_path=conn | resp_bytes=sum(resp_bytes) by id.resp_h,id.orig_h | sort id.resp_h,id.orig_h"
```
And now we can search with a primary key and a secondary key, e.g.,
```
zed lake find -Z -x custom2.zng 216.58.193.206 10.47.6.173
```
which produces just one record as this pair appears in only one log file.
```
{
    id: {
        resp_h: 216.58.193.206,
        orig_h: 10.47.6.173
    },
    resp_bytes: 5112 (uint64),
    _log: "/tmp/logs/zd/20180324/d-1q85q79hAjNCHg5Wx0EQlkulmKU.zng" (=zfile),
    first: 2018-03-24T17:29:56.0241Z,
    last: 2018-03-24T17:15:20.601374Z
} (=0)
```
The nice thing here is that you can also just specify a primary key, which will
issue a search that returns all the index hits that have the primary key with
any value for the secondary key, e.g.,
```
zed lake find -z -x custom2.zng 216.58.193.206
```
and of course you can sum up all the response bytes to get a table and output
it as text...
```
zed lake find -x custom2.zng 216.58.193.206 | zq -f text "sum(resp_bytes)" -
```
Note that you can't "wild card" the primary key when doing a search via
`zed lake find` because the index is sorted by primary key first, then secondary
key, and so forth, and efficient lookups are carried out by traversing the
b-tree index structure of these sorted keys.  But remember,
everything is a zng file, so you can do a brute-force search on the base-layer
of the index, e.g., to look for all the instances of a value in the secondary
key position (ignoring the primary key) by using
`zed lake map` instead of `zed lake find`.

So, let's say we wanted
a count of all bytes received by 10.47.6.173 as the originator, which is the
secondary key.  While we could build a different custom index where `id.orig_h`
is the primary key, we could also just scan the custom2 index using brute force:
```
zed lake map id.orig_h=10.47.6.173 idx-$(zed lake index ls -f zng | zq -f text 'desc="zql-custom2.zng" | cut id' -).zng | zq -f text "sum(resp_bytes)" -
```
Even though this is a "brute force scan", it's a brute force scan of only this
one index so it runs much faster than scanning all of the original
log data to perform the same query.

But to double check here, you can run
```
zed lake map id.orig_h=10.47.6.173 | zq -f text "sum(resp_bytes)" -
```
and you will get the same answer.


## Map-reduce

What's really going on here is map-reduce style computation on your log archives
without having to set up a spark cluster and write java map-reduce classes.

The classic map-reduce example is word count.  Let's do this example with
the uri field in http logs.  First, we map each record that has a uri
to a new record with that uri and a count of 1:
```
zed lake map -q -o words.zng "uri != null | cut uri | put count=1"
```
again you can look at one of the files...
```
find $ZED_LAKE_ROOT -name words.zng ! -size 0 | head -n 1 | xargs zq -z -
```
Now we reduce by aggregating the uri and summing the counts:
```
zed lake map -q -o wordcounts.zng "sum(count) by uri | cut uri,count=sum" words.zng
```
If we were dealing with a huge archive, we could do an approximation by taking
the top 1000 in each lake directory then we could aggregate with another zq
command at the top-level:
```
zed lake map "sort -r count | head 1000" wordcounts.zng | zq -f table "sum(count) by uri | sort -r sum | head 10" -
```
and you get the top-ten URIs...
```
uri                     sum
/wordpress/wp-login.php 6516
/                       5848
/api/get/3/6            4677
/api/get/1/2            4645
/api/get/4/7            4639
/api/get/2/3            4638
/api/get/4/8            4638
/api/get/1/1            4636
/api/get/6/12           4634
/api/get/9/18           4627
```
Pretty cool!

## Simple graph queries

Here is another example to illustrate the power of `zed lake`.  Just like you can
build search indexes with arbitrary Zed queries, you can also build graph indexes
to hold edge lists and node attributes, providing an efficient means to do
topological queries of graph data structures.  While this doesn't provide a
full-featured graph database like [neo4j](https://github.com/neo4j/neo4j),
it does provide a nice way to do many types of graph queries that
could prove useful for your archive analytics.

For example, to build a searchable edge list comprising communicating IP addresses,
run the following commands to a create a multi-key search index
keyed by all unique instances of IP address pairs (in both directions)
where each index includes a count of the occurrences.
```
zed lake map -o forward.zng "id.orig_h != null | put from_addr=id.orig_h,to_addr=id.resp_h | count() by from_addr,to_addr"
zed lake map -o reverse.zng "id.orig_h != null | put from_addr=id.resp_h,to_addr=id.orig_h | count() by from_addr,to_addr"
zed lake map -o directed-pairs.zng "count=sum(count) by from_addr,to_addr | sort from_addr,to_addr" forward.zng reverse.zng
zed lake index create -i directed-pairs.zng -o graph.zng -k from_addr,to_addr -z "*"
```
> (Note: there is a small change we can make to the Zed language to do this with one
> command... coming soon.)

This creates an index called "graph" that you can use to search for IP address
pair relationships, e.g., you can say
```
zed lake find -x graph.zng 216.58.193.195 | zq -f table "count=sum(count) by from_addr,to_addr | sort -r count" -
```
to get a listing of all of the edges from IP 216.58.193.195 to any other IP,
which looks like this:
```
from_addr      to_addr      count
216.58.193.195 10.47.2.155  55
216.58.193.195 10.47.2.100  47
216.58.193.195 10.47.6.162  31
216.58.193.195 10.47.7.150  30
216.58.193.195 10.47.5.153  26
216.58.193.195 10.47.3.154  25
216.58.193.195 10.47.5.152  24
216.58.193.195 10.47.8.19   23
216.58.193.195 10.47.7.154  19
...
```
To view this with d3, we can collect up the edges emanating from a few IP addresses
and format the output as ndjson in the format expected by bostock's
force-directed graph.
This command sequence will collect up the edges into `edges.njdson`:
```
zed lake find -z -x graph.zng 216.58.193.195 | zq "count() by from_addr,to_addr" - > edges.zng
zed lake find -z -x graph.zng 10.47.6.162 | zq "count() by from_addr,to_addr" - >> edges.zng
zed lake find -z -x graph.zng 10.47.5.153 | zq "count() by from_addr,to_addr" - >> edges.zng
zq -f ndjson "value=sum(count) by from_addr,to_addr | cut source=from_addr,target=to_addr,value" edges.zng >> edges.ndjson
```
> (Note: with a few additions to Zed, we can make this much simpler and
> more efficient.  Coming soon.  Also, we should be able to say `group by node`,
> which implies no reducer and emits columns with just the group-by keys.)

Now that we have the edges in `edges.ndjson`, let's grab all the nodes
in the graph and put them in the form expected by bostock using this command sequence:
```
zq "count() by from_addr | put id=from_addr" edges.zng > nodes.zng
zq "count() by to_addr | put id=to_addr" edges.zng >> nodes.zng
zq -f ndjson "count() by id | cut id | put addr_group=1" nodes.zng > nodes.ndjson
```
<!-- markdown-link-check-disable -->
To make a simple demo of this concept here, I cut and paste the nodes and edges
data into a gist and added some commas with `awk`.  Check out this
[d3 "block"](https://bl.ocks.org/mccanne/ff6f703cf202aee59197fff1f63d04fe).
<!-- markdown-link-check-enable -->

## A final word about pipes...

As you've likely noticed, we love pipes in the zq project. Make a test file:

```
zq "head 10000" zng/* > pipes.zng
```

You can use pipes in Zed expressions like you've seen above:

```
zq -f table "orig_bytes > 100 | count() by id.resp_p | sort -r" pipes.zng
```

Or you can pipe the output of one zq to another...
```
zq "orig_bytes > 100 | count() by id.resp_p" pipes.zng | zq -f table "sort -r" -
```
We were careful to make the output of zq just a stream of zng records.
So whether you are piping within a Zed query, or between zq commands, or
between zed and zq, or over the network (ssh zq...), it's all the same.
```
zq "orig_bytes > 100" pipes.zng | zq "count() by id.resp_p" - | zq -f table "sort -r" -
```
In fact, files are self-contained zng streams, so you can just cat them together
and you still end up with a valid zng stream
```
cat pipes.zng pipes.zng > pipes2.zng
zq -f text "count()" pipes.zng
zq -f text "count()" pipes2.zng
```

## Cleanup

To clean out all the files you've created in the lake directories and
start over, just run
```
zed lake rmdirs $ZED_LAKE_ROOT
```

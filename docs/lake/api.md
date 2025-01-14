---
sidebar_position: 1
sidebar_label: API
---

# Zed lake API

## _Status_

> This is a brief sketch of the functionality exposed in the
> Zed API. More detailed documentation of the API will be forthcoming.

## Endpoints

### Pools

#### Create pool

Create a new lake pool.

```
POST /pool
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| name | string | body | **Required.** Name of the pool. Must be unique to lake. |
| layout.order | string | body | Order of storage by primary key(s) in pool. Possible values: desc, asc. Default: asc. |
| layout.keys | [[string]] | body | Primary key(s) of pool. The element of each inner string array should reflect the hierarchical ordering of named fields within indexed records. Default: [[ts]]. |
| thresh | int | body | The size in bytes of each seek index. |
| Content-Type | string | header | [MIME type](#mime-types) of the request payload. |
| Accept | string | header | Preferred [MIME type](#mime-types) of the response. |

**Example Request**

```
curl -X POST \
     -H 'Accept: application/json' \
     -H 'Content-Type: application/json' \
     -d '{"name": "inventory", "layout": {"keys": [["product","serial_number"]]}}' \
     http://localhost:9867/pool
```

**Example Response**

```
{
  "pool": {
    "ts": "2022-07-13T21:23:05.323016Z",
    "name": "inventory",
    "id": "0x0f5ce9b9b6202f3883c9db8ff58d8721a075d1e4",
    "layout": {
      "order": "asc",
      "keys": [
        [
          "product",
          "serial_number"
        ]
      ]
    },
    "seek_stride": 65536,
    "threshold": 524288000
  },
  "branch": {
    "ts": "2022-07-13T21:23:05.367365Z",
    "name": "main",
    "commit": "0x0000000000000000000000000000000000000000"
  }
}
```

---

#### Rename pool

Change a pool's name.

```
PUT /pool/{pool}
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| pool | string | path | **Required.** ID or name of the requested pool. |
| name | string | body | **Required.** The desired new name of the pool. Must be unique to lake. |
| Content-Type | string | header | [MIME type](#mime-types) of the request payload. |

**Example Request**

```
curl -X PUT \
     -H 'Content-Type: application/json' \
     -d '{"name": "catalog"}' \
     http://localhost:9867/pool/inventory
```

On success, HTTP 204 is returned with no response payload.

---

#### Delete pool

Permanently delete a pool.

```
DELETE /pool/{pool}
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| pool | string | path | **Required.** ID or name of the requested pool. |

**Example Request**

```
curl -X DELETE \
     http://localhost:9867/pool/inventory
```

On success, HTTP 204 is returned with no response payload.

---

#### Vacuum pool

Free storage space by permanently removing underlying data objects that have
previously been subject to a [delete](#delete-data) operation.

```
POST /pool/{pool}/revision/{revision}/vacuum
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| pool | string | path | **Required.** ID or name of the requested pool. |
| revision | string | path | **Required.** The starting point for locating objects that can be vacuumed. Can be the name of a branch (whose tip would be used) or a commit ID. |
| dryrun | string | query | Set to "T" to return the list of objects that could be vacuumed, but don't actually remove them. Defaults to "F". |

**Example Request**

```
curl -X POST \
     -H 'Accept: application/json' \
     http://localhost:9867/pool/inventory/revision/main/vacuum
```

**Example Response**

```
{"object_ids":["0x10f5a24253887eaf179ee385532ee411c2ed8050","0x10f5a2410ccd08f72e5d98f6d054477173b4f13f"]}
```

---

### Branches

#### Load Data

Add data to a pool and return a reference commit ID.

```
POST /pool/{pool}/branch/{branch}
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| pool | string | path | **Required.** ID or name of the pool. |
| branch | string | path | **Required.** Name of branch to which data will be loaded. |
|   | various | body | **Required.** Contents of the posted data. |
| csv.delim | string | query | Exactly one character specifying the field delimiter for CSV data. Defaults to ",". |
| Content-Type | string | header | [MIME type](#mime-types) of the posted content. If undefined, the service will attempt to introspect the data and determine type automatically. |
| Accept | string | header | Preferred [MIME type](#mime-types) of the response. |

**Example Request**

```
curl -X POST \
     -H 'Accept: application/json' \
     -H 'Content-Type: application/json' \
     -d '{"product": {"serial_number": 12345, "name": "widget"}, "warehouse": "chicago"}
         {"product": {"serial_number": 12345, "name": "widget"}, "warehouse": "miami"}
         {"product": {"serial_number": 12346, "name": "gadget"}, "warehouse": "chicago"}' \
     http://localhost:9867/pool/inventory/branch/main
```

**Example Response**

```
{"commit":"0x0ed4f42da5763a9500ee71bc3fa5c69f306872de","warnings":[]}
```

---

#### Get Branch

Get information about a branch.

```
GET /pool/{pool}/branch/{branch}
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| pool | string | path | **Required.** ID or name of the pool. |
| branch | string | path | **Required.** Name of branch. |
| Accept | string | header | Preferred [MIME type](#mime-types) of the response. |

**Example Request**

```
curl -X GET \
     -H 'Accept: application/json' \
     http://localhost:9867/pool/inventory/branch/main
```

**Example Response**

```
{"commit":"0x0ed4fa21616ecd8fec9d6fd395ad876db98a5dae","warnings":null}
```

---

#### Delete Branch

Delete a branch.

```
DELETE /pool/{pool}/branch/{branch}
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| pool | string | path | **Required.** ID or name of the pool. |
| branch | string | path | **Required.** Name of branch. |

**Example Request**

```
curl -X DELETE \
     http://localhost:9867/pool/inventory/branch/staging
```

On success, HTTP 204 is returned with no response payload.

---

#### Delete Data

Create a commit that reflects the deletion of some data in the branch. The data
to delete can be specified via a list of object IDs or
as a filter expression (see [limitations](../commands/super-db.md#delete)).

This simply removes the data from the branch without actually removing the
underlying data objects thereby allowing [time travel](../commands/super-db.md#time-travel) to work in the face
of deletes. Permanent removal of underlying data objects is handled by a
separate [vacuum](#vacuum-pool) operation.

```
POST /pool/{pool}/branch/{branch}/delete
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| pool | string | path | **Required.** ID of the pool. |
| branch | string | path | **Required.** Name of branch. |
| object_ids | [string] | body | Object IDs to be deleted. |
| where | string | body | Filter expression (see [limitations](../commands/super-db.md#delete)). |
| Content-Type | string | header | [MIME type](#mime-types) of the request payload. |
| Accept | string | header | Preferred [MIME type](#mime-types) of the response. |

**Example Request**

```
curl -X POST \
     -H 'Accept: application/json' \
     -H 'Content-Type: application/json' \
     -d '{"object_ids": ["274Eb1Kn8MTM6qxPyBpVTvYhLLa", "274EavbXt546VNelRLNXrzWShNh"]}' \
     http://localhost:9867/pool/inventory/branch/main/delete
```

**Example Response**

```
{"commit":"0x0ed4fee861e8fb61568783205a46a218182eba6c","warnings":null}
```

**Example Request**

```
curl -X POST \
     -H 'Accept: application/json' \
     -H 'Content-Type: application/json' \
     -d '{"where": "product.serial_number > 12345"}' \
     http://localhost:9867/pool/inventory/branch/main/delete
```

**Example Response**

```
{"commit":"0x0f5ceaeaaec7b4c33cfdece9f2e8577ad89d21e2","warnings":null}
```

---

#### Merge Branches

Create a commit with the difference of the child branch added to the selected
branch.

```
POST /pool/{pool}/branch/{branch}/merge/{child}
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| pool | string | path | **Required.** ID of the pool. |
| branch | string | path | **Required.** Name of branch selected as merge destination. |
| child | string | path | **Required.** Name of child branch selected as source of merge. |
| Accept | string | header | Preferred [MIME type](#mime-types) of the response. |

**Example Request**

```
curl -X POST \
     -H 'Accept: application/json' \
     http://localhost:9867/pool/inventory/branch/main/merge/staging
```

**Example Response**

```
{"commit":"0x0ed4ffc2566b423ee444c1c8e6bf964515290f4c","warnings":null}
```

---

#### Revert

Create a revert commit of the specified commit.

```
POST /pool/{pool}/branch/{branch}/revert/{commit}
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| pool | string | path | **Required.** ID of the pool. |
| branch | string | path | **Required.** Name of branch on which to revert commit. |
| commit | string | path | **Required.** ID of commit to be reverted. |
| Accept | string | header | Preferred [MIME type](#mime-types) of the response. |

**Example Request**

```
curl -X POST \
     -H 'Accept: application/json' \
     http://localhost:9867/pool/inventory/branch/main/revert/27D22ifDw3Ms2NMzo8jXpDfpgjc
```

**Example Response**

```
{"commit":"0x0ed500ab6f80e5ac8a1b871bddd88c57fe963ab1","warnings":null}
```

---

### Query

Execute a Zed query against data in a data lake.

```
POST /query
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| query | string | body | Zed query to execute. All data is returned if not specified. ||
| head.pool | string | body | Pool to query against Not required if pool is specified in query. |
| head.branch | string | body | Branch to query against. Defaults to "main". |
| ctrl | string | query | Set to "T" to include control messages in ZNG or ZJSON responses. Defaults to "F". |
| Content-Type | string | header | [MIME type](#mime-types) of the request payload. |
| Accept | string | header | Preferred [MIME type](#mime-types) of the response. |

**Example Request**

```
curl -X POST \
     -H 'Accept: application/x-zson' \
     -H 'Content-Type: application/json' \
     http://localhost:9867/query -d '{"query":"from inventory@main |> count() by warehouse"}'
```

**Example Response**

```
{warehouse:"chicago",count:2(uint64)}
{warehouse:"miami",count:1(uint64)}
```

**Example Request**

```
curl -X POST \
     -H 'Accept: application/x-zjson' \
     -H 'Content-Type: application/json' \
     http://localhost:9867/query?ctrl=T -d '{"query":"from inventory@main |> count() by warehouse"}'
```

**Example Response**

```
{"type":"QueryChannelSet","value":{"channel":"main"}}
{"type":{"kind":"record","id":30,"fields":[{"name":"warehouse","type":{"kind":"primitive","name":"string"}},{"name":"count","type":{"kind":"primitive","name":"uint64"}}]},"value":["miami","1"]}
{"type":{"kind":"ref","id":30},"value":["chicago","2"]}
{"type":"QueryChannelEnd","value":{"channel":"main"}}
{"type":"QueryStats","value":{"start_time":{"sec":1658193276,"ns":964207000},"update_time":{"sec":1658193276,"ns":964592000},"bytes_read":55,"bytes_matched":55,"records_read":3,"records_matched":3}}
```

#### Query Status

Retrieve any runtime errors from a specific query. This endpoint only responds
after the query has exited and is only available for a limited time afterwards.

```
GET /query/status/{request_id}
```

**Params**

| Name | Type | In | Description |
| ---- | ---- | -- | ----------- |
| request_id | string | path | **Required.** The value of the response header `X-Request-Id` of the target query. |

**Example Request**

```
curl -X GET \
     -H 'Accept: application/json' \
     http://localhost:9867/query/status/2U1oso7btnCXfDenqFOSExOBEIv
```

**Example Response**

```
{"error":"parquetio: unsupported type: empty record"}
```

---

### Events

Subscribe to an events feed, which returns an event stream in the format of
[server-sent events](https://html.spec.whatwg.org/multipage/server-sent-events.html).
The [MIME type](#mime-types) specified in the request's Accept HTTP header determines the format
of `data` field values in the event stream.

```
GET /events
```

**Params**

None

**Example Request**

```
curl -X GET \
     -H 'Accept: application/json' \
     http://localhost:9867/events
```

**Example Response**

```
event: pool-new
data: {"pool_id": "1sMDXpVwqxm36Rc2vfrmgizc3jz"}

event: pool-update
data: {"pool_id": "1sMDXpVwqxm36Rc2vfrmgizc3jz"}

event: pool-commit
data: {"pool_id": "1sMDXpVwqxm36Rc2vfrmgizc3jz", "commit_id": "1tisISpHoWI7MAZdFBiMERXeA2X"}

event: pool-delete
data: {"pool_id": "1sMDXpVwqxm36Rc2vfrmgizc3jz"}
```

---

## Media Types

For both request and response payloads, the service supports a variety of
formats.

### Request Payloads

When sending request payloads, include the MIME type of the format in the
request's Content-Type header. If the Content-Type header is not specified, the
service will expect ZSON as the payload format.

An exception to this is when [loading data](#load-data) and Content-Type is not
specified. In this case the service will attempt to introspect the data and may
determine the type automatically. The
[input formats](../commands/super.md#input-formats) table describes which
formats may be successfully auto-detected.

### Response Payloads

To receive successful (2xx) responses in a preferred format, include the MIME
type of the format in the request's Accept HTTP header. If the Accept header is
not specified, the service will return ZSON as the default response format. A
different default response format can be specified by invoking the
`-defaultfmt` option when running [`super db serve`](../commands/super-db.md#serve).

For non-2xx responses, the content type of the response will be
`application/json` or `text/plain`.

### MIME Types

The following table shows the supported MIME types and where they can be used.

| Format           | Request   | Response | MIME Type                             |
| ---------------- | --------- | -------- | ------------------------------------- |
| Arrow IPC Stream | yes       | yes      | `application/vnd.apache.arrow.stream` |
| CSV              | yes       | yes      | `text/csv`                            |
| JSON             | yes       | yes      | `application/json`                    |
| Line             | yes       | yes      | `application/x-line`                  |
| NDJSON           | no        | yes      | `application/x-ndjson`                |
| Parquet          | yes       | yes      | `application/x-parquet`               |
| TSV              | yes       | yes      | `text/tab-separated-values`           |
| VNG              | yes       | yes      | `application/x-vng`                   |
| Zeek             | yes       | yes      | `application/x-zeek`                  |
| ZJSON            | yes       | yes      | `application/x-zjson`                 |
| ZSON             | yes       | yes      | `application/x-zson`                  |
| ZNG              | yes       | yes      | `application/x-zng`                   |

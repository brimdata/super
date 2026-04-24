"""
Integration tests for the superdb Python client.

These tests require a running SuperDB service. By default they connect to
http://localhost:9867 (the default SuperDB port). Set the SUPER_DB environment
variable to point at a different instance. Tests are automatically skipped if
the service is not reachable.

Start a local instance with:
    super db serve
"""

import datetime
import os
import uuid

import pytest
import requests

from superdb import Client, MixedTypesError, NonRecordError, RequestError

_BASE_URL = os.environ.get('SUPER_DB', 'http://localhost:9867').rstrip('/')
try:
    requests.get(_BASE_URL + '/pool', timeout=2)
except requests.exceptions.ConnectionError:
    pytest.skip(
        f'SuperDB service not reachable at {_BASE_URL}',
        allow_module_level=True,
    )


# One SUP record containing a field for each type under test.
_SUP_ALL_TYPES = """\
{
    time_val: 2024-01-02T03:04:05Z,
    int8_val: -1::int8,
    int16_val: -1000::int16,
    int32_val: -1000000::int32,
    int64_val: -1000000000,
    uint8_val: 255::uint8,
    uint16_val: 65535::uint16,
    uint32_val: 4294967295::uint32,
    uint64_val: 9007199254740992::uint64,
    float32_val: 1.5::float32,
    float64_val: 2.718281828,
    bool_val: true,
    string_val: "hello",
    bytes_val: 0x68656c6c6f,
    duration_val: 1h30m,
    null_val: null,
    ip_val: 192.0.2.1,
    net_val: 10.0.0.0/8,
    record_val: {x: 1, y: 2},
    nested_record_val: {x: {y: 1}},
    array_val: [1, 2, 3],
    set_val: set[4,5,6],
    map_val: map{"a":1,"b":2},
    enum_val: "HEADS"::enum(HEADS,TAILS),
    error_val: error("something went wrong"),
    type_val: <int64>,
    union_int_val: 42::(int64|string),
    union_str_val: "hello"::(int64|string)
}
"""


# ---------------------------------------------------------------------------
# Session-scoped fixtures
# ---------------------------------------------------------------------------

@pytest.fixture(scope='session')
def client():
    return Client()


@pytest.fixture(scope='session')
def pool(client):
    name = 'test_types_' + uuid.uuid4().hex[:8]
    client.create_pool(name)
    client.load(name, _SUP_ALL_TYPES.encode(), mime_type='application/x-sup')
    yield name
    client.delete_pool(name)


@pytest.fixture(scope='session')
def record(client, pool):
    results = list(client.query(f'from {pool}'))
    assert len(results) == 1
    return results[0]


# ---------------------------------------------------------------------------
# Positive tests: types that convert cleanly to native Python types
# ---------------------------------------------------------------------------

def test_time(record):
    assert record['time_val'] == datetime.datetime(2024, 1, 2, 3, 4, 5,
                                                   tzinfo=datetime.timezone.utc)


def test_int8(record):
    assert record['int8_val'] == -1


def test_int16(record):
    assert record['int16_val'] == -1000


def test_int32(record):
    assert record['int32_val'] == -1000000


def test_int64(record):
    assert record['int64_val'] == -1000000000


def test_uint8(record):
    assert record['uint8_val'] == 255


def test_uint16(record):
    assert record['uint16_val'] == 65535


def test_uint32(record):
    assert record['uint32_val'] == 4294967295


def test_uint64(record):
    assert record['uint64_val'] == 9007199254740992


def test_float32(record):
    assert record['float32_val'] == pytest.approx(1.5)


def test_float64(record):
    assert record['float64_val'] == pytest.approx(2.718281828)


def test_bool(record):
    assert record['bool_val'] is True


def test_string(record):
    assert record['string_val'] == 'hello'


def test_bytes(record):
    assert record['bytes_val'] == b'hello'


def test_duration(record):
    assert record['duration_val'] == datetime.timedelta(hours=1, minutes=30)


def test_null(record):
    assert record['null_val'] is None


def test_record(record):
    assert record['record_val'] == {'x': 1, 'y': 2}


def test_nested_record(record):
    assert record['nested_record_val'] == {'x': {'y': 1}}


def test_array(record):
    assert record['array_val'] == [1, 2, 3]


def test_set(record):
    # Sets are encoded as Arrow lists. SuperDB enforces no duplicates before
    # encoding, so the list will not contain duplicates, but Python does not
    # enforce this going forward.
    assert isinstance(record['set_val'], list)
    assert sorted(record['set_val']) == [4, 5, 6]


def test_map(record):
    assert record['map_val'] == {'a': 1, 'b': 2}


def test_union(record):
    # Union values arrive as the Python type of the active branch.
    assert record['union_int_val'] == 42
    assert isinstance(record['union_int_val'], int)
    assert record['union_str_val'] == 'hello'
    assert isinstance(record['union_str_val'], str)


# ---------------------------------------------------------------------------
# Limited-type tests: SuperDB types that arrive as plain Python strings
# ---------------------------------------------------------------------------

def test_ip_arrives_as_string(record):
    # ip values are encoded as strings by the Arrow writer; they are not
    # ipaddress.IPv4Address or IPv6Address objects.
    assert record['ip_val'] == '192.0.2.1'
    assert isinstance(record['ip_val'], str)


def test_net_arrives_as_string(record):
    # net values are encoded as strings; not ipaddress.IPv4Network etc.
    assert record['net_val'] == '10.0.0.0/8'
    assert isinstance(record['net_val'], str)


def test_enum_arrives_as_string(record):
    assert record['enum_val'] == 'HEADS'
    assert isinstance(record['enum_val'], str)


def test_error_arrives_as_string(record):
    assert isinstance(record['error_val'], str)
    assert 'something went wrong' in record['error_val']


def test_type_value_arrives_as_string(record):
    assert isinstance(record['type_val'], str)
    assert 'int64' in record['type_val']


# ---------------------------------------------------------------------------
# Negative tests: structural situations the Arrow encoder rejects
# ---------------------------------------------------------------------------

def test_multiple_record_types_raises_mixed_types_error(client):
    # With safe=True (default), query() runs a pre-flight check and raises
    # MixedTypesError before requesting Arrow data, preventing silent truncation.
    name = 'test_multi_' + uuid.uuid4().hex[:8]
    client.create_pool(name)
    try:
        client.load(name, b'{a: 1}\n{b: "hello"}',
                    mime_type='application/x-sup')
        with pytest.raises(MixedTypesError) as exc_info:
            list(client.query(f'from {name}'))
        assert exc_info.value.type_count == 2
    finally:
        client.delete_pool(name)


def test_multiple_record_types_safe_false_returns_partial(client):
    # With safe=False the pre-flight check is skipped.  The Arrow encoder
    # silently truncates at the first schema change, so callers that opt out
    # must know their data is homogeneous.
    name = 'test_multi_unsafe_' + uuid.uuid4().hex[:8]
    client.create_pool(name)
    try:
        client.load(name, b'{a: 1}\n{b: "hello"}',
                    mime_type='application/x-sup')
        results = list(client.query(f'from {name}', safe=False))
        assert len(results) < 2
    finally:
        client.delete_pool(name)


def test_non_record_values_raises_non_record_error(client):
    # Data loaded in line format is stored as top-level strings, not records.
    # With safe=True (default), query() detects this and raises NonRecordError.
    name = 'test_nonrec_' + uuid.uuid4().hex[:8]
    client.create_pool(name)
    try:
        client.load(name, b'hello\nworld', mime_type='application/x-line')
        with pytest.raises(NonRecordError) as exc_info:
            list(client.query(f'from {name}'))
        assert exc_info.value.kinds == ['primitive']
    finally:
        client.delete_pool(name)


def test_non_record_values_safe_false_returns_empty(client):
    # With safe=False, non-record values are not detected.  Arrow cannot encode
    # non-record top-level values, so the server returns an empty stream.
    name = 'test_nonrec_unsafe_' + uuid.uuid4().hex[:8]
    client.create_pool(name)
    try:
        client.load(name, b'hello\nworld', mime_type='application/x-line')
        assert list(client.query(f'from {name}', safe=False)) == []
    finally:
        client.delete_pool(name)


def test_empty_record_returns_no_results(client):
    # Arrow does not support records with no fields.  The server returns an
    # empty stream rather than an error, so the client silently returns no
    # results.  See the README for details.
    name = 'test_empty_' + uuid.uuid4().hex[:8]
    client.create_pool(name)
    try:
        client.load(name, b'{}', mime_type='application/x-sup')
        assert list(client.query(f'from {name}')) == []
    finally:
        client.delete_pool(name)

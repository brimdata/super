"""
Auth integration tests for the SuperDB Python client.

These tests require a SuperDB service running with authentication enabled
and valid credentials stored in ~/.super/credentials.json.  They are
intended to be run via the pytest-auth ztest, which handles that setup
automatically.  When run manually with plain pytest they are skipped unless
the SUPER_DB_AUTH environment variable is set.
"""

import os
import uuid

import pytest
import requests

from superdb import Client, RequestError

if not os.environ.get('SUPER_DB_AUTH'):
    pytest.skip('auth not configured (SUPER_DB_AUTH not set)', allow_module_level=True)

_BASE_URL = os.environ.get('SUPER_DB', 'http://localhost:9867').rstrip('/')
try:
    requests.get(_BASE_URL + '/status', timeout=2)
except requests.exceptions.ConnectionError:
    pytest.skip(
        f'SuperDB service not reachable at {_BASE_URL}',
        allow_module_level=True,
    )


def test_authenticated_client_can_query():
    client = Client()
    name = 'test_auth_' + uuid.uuid4().hex[:8]
    client.create_pool(name)
    try:
        client.load(name, b'{a: 1}', mime_type='application/x-sup')
        assert list(client.query(f'from {name}')) == [{'a': 1}]
    finally:
        client.delete_pool(name)


def test_unauthenticated_client_raises_request_error():
    # A client with no credentials (config_dir='') should be rejected by an
    # auth-enabled service on any request.
    with pytest.raises(RequestError):
        Client(config_dir='').create_pool('x')

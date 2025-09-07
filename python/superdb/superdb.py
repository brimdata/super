import binascii
import decimal
import getpass
import ipaddress
import json
import os
import os.path
import urllib.parse

import dateutil.parser
import durationpy
import requests


class Client():
    def __init__(self,
                 base_url=os.environ.get('SUPER_DB', 'http://localhost:9867'),
                 config_dir=os.path.expanduser('~/.super')):
        self.base_url = base_url.rstrip('/')
        self.session = requests.Session()
        self.session.headers.update({'Accept': 'application/x-jsup'})
        token = self.__get_auth_token(config_dir)
        if token is not None:
            self.session.headers.update({'Authorization': 'Bearer ' + token})

    def __get_auth_token(self, config_dir):
        creds_path = os.path.join(config_dir, 'credentials.json')
        try:
            with open(creds_path) as f:
                data = f.read()
        except FileNotFoundError:
            return None
        creds = json.loads(data)
        if self.base_url in creds['services']:
            return creds['services'][self.base_url]['access']
        return None

    def create_pool(self, name, layout={'order': 'desc', 'keys': [['ts']]},
                    thresh=0):
        r = self.session.post(self.base_url + '/pool', json={
            'name': name,
            'layout': layout,
            'thresh': thresh,
        })
        self.__raise_for_status(r)

    def load(self, pool_name_or_id, data, branch_name='main',
             commit_author=getpass.getuser(), commit_body=''):
        pool = urllib.parse.quote(pool_name_or_id, safe='')
        branch = urllib.parse.quote(branch_name, safe='')
        url = self.base_url + '/pool/' + pool + '/branch/' + branch
        commit_message = {'author': commit_author, 'body': commit_body}
        headers = {'SuperDB-Commit': json.dumps(commit_message)}
        r = self.session.post(url, headers=headers, data=data)
        self.__raise_for_status(r)

    def query(self, query):
        r = self.query_raw(query)
        jsup = (json.loads(line) for line in r.iter_lines() if line)
        return decode_jsup(jsup)

    def query_raw(self, query, headers=None):
        r = self.session.post(self.base_url + '/query', headers=headers,
                              json={'query': query}, stream=True)
        self.__raise_for_status(r)
        return r

    @staticmethod
    def __raise_for_status(response):
        if response.status_code >= 400:
            try:
                error = response.json()['error']
            except Exception:
                response.raise_for_status()
            else:
                raise RequestError(error, response)


class RequestError(Exception):
    """Raised by Client methods when an HTTP request fails."""
    def __init__(self, message, response):
        super(RequestError, self).__init__(message)
        self.response = response


class QueryError(Exception):
    """Raised by Client.query() when a query fails."""
    pass


def decode_jsup(jsup):
    types = {}
    for msg in jsup:
        typ, value = msg['type'], msg['value']
        if isinstance(typ, dict):
            yield _decode_value(_decode_type(types, typ), value)
        elif typ == 'QueryError':
            raise QueryError(value['error'])


def _decode_type(types, typ):
    kind = typ['kind']
    if kind == 'ref':
        return types[typ['id']]
    if kind == 'primitive':
        return typ
    elif kind == 'record':
        for f in typ['fields']:
            f['type'] = _decode_type(types, f['type'])
    elif kind in ['array', 'set']:
        typ['type'] = _decode_type(types, typ['type'])
    elif kind == 'map':
        typ['key_type'] = _decode_type(types, typ['key_type'])
        typ['val_type'] = _decode_type(types, typ['val_type'])
    elif kind == 'union':
        typ['types'] = [_decode_type(types, t) for t in typ['types']]
    elif kind == 'enum':
        pass
    elif kind in ['error', 'named']:
        typ['type'] = _decode_type(types, typ['type'])
    else:
        raise Exception(f'unknown type kind {kind}')
    types[typ['id']] = typ
    return typ


def _decode_value(typ, value):
    if value is None:
        return None
    kind = typ['kind']
    if kind == 'primitive':
        name = typ['name']
        if name in ['uint8', 'uint16', 'uint32', 'uint64',
                    'int8', 'int16', 'int32', 'int64']:
            return int(value)
        if name == 'duration':
            return durationpy.from_str(value)
        if name == 'time':
            return dateutil.parser.isoparse(value)
        if name in ['float16', 'float32', 'float64']:
            return float(value)
        if name == 'decimal':
            return decimal.Decimal(value)
        if name == 'bool':
            return value == 'true'
        if name == 'bytes':
            return binascii.a2b_hex(value[2:])
        if name == 'string':
            return value
        if name == 'ip':
            return ipaddress.ip_address(value)
        if name == 'net':
            return ipaddress.ip_network(value)
        if name in 'type':
            return value
        if name == 'null':
            return None
        raise Exception(f'unknown primitive name {name}')
    if kind == 'record':
        return {f['name']: _decode_value(f['type'], v)
                for f, v in zip(typ['fields'], value)}
    if kind == 'array':
        return [_decode_value(typ['type'], v) for v in value]
    if kind == 'set':
        return {_decode_value(typ['type'], v) for v in value}
    if kind == 'map':
        key_type, val_type = typ['key_type'], typ['val_type']
        return {_decode_value(key_type, v[0]): _decode_value(val_type, v[1])
                for v in value}
    if kind == 'union':
        type_index, val = value
        return _decode_value(typ['types'][int(type_index)], val)
    if kind == 'enum':
        return typ['symbols'][int(value)]
    if kind in ['error', 'named']:
        return _decode_value(typ['type'], value)
    raise Exception(f'unknown type kind {kind}')


if __name__ == '__main__':
    import argparse
    import pprint

    parser = argparse.ArgumentParser(
        description='Query default SuperDB service and print results.',
        formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument('query')
    args = parser.parse_args()

    c = Client()
    for record in c.search(args.query):
        pprint.pprint(record)

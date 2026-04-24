import getpass
import json
import os
import os.path
import urllib.parse

import pyarrow as pa
import pyarrow.ipc
import requests


class Client():
    def __init__(self,
                 base_url=os.environ.get('SUPER_DB', 'http://localhost:9867'),
                 config_dir=os.path.expanduser('~/.super')):
        self.base_url = base_url.rstrip('/')
        self.session = requests.Session()
        self.session.headers.update({'Accept': 'application/vnd.apache.arrow.stream'})
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
             commit_author=getpass.getuser(), commit_body='',
             mime_type=None):
        pool = urllib.parse.quote(pool_name_or_id, safe='')
        branch = urllib.parse.quote(branch_name, safe='')
        url = self.base_url + '/pool/' + pool + '/branch/' + branch
        commit_message = {'author': commit_author, 'body': commit_body}
        headers = {'SuperDB-Commit': json.dumps(commit_message)}
        if mime_type is not None:
            headers['Content-Type'] = mime_type
        r = self.session.post(url, headers=headers, data=data)
        self.__raise_for_status(r)

    def delete_pool(self, pool_name_or_id):
        pool = urllib.parse.quote(pool_name_or_id, safe='')
        r = self.session.delete(self.base_url + '/pool/' + pool)
        self.__raise_for_status(r)

    def query(self, query, safe=True):
        if safe:
            # Pre-flight: verify all top-level values are records of a single
            # type.  Arrow requires top-level records and silently truncates on
            # type changes, so we detect both problems before issuing the real
            # query.
            safety_r = self.query_raw(
                query + ' | union(typeof(this)) by kind(this)',
                headers={'Accept': 'application/x-ndjson'},
            )
            rows = [
                json.loads(line)
                for line in safety_r.iter_lines(decode_unicode=True)
                if line
            ]
            if rows:
                if any(row['kind'] != 'record' for row in rows):
                    kinds = sorted({row['kind'] for row in rows})
                    raise NonRecordError(
                        f"Query result contains non-record values "
                        f"(kind: {', '.join(repr(k) for k in kinds)}). "
                        f"Arrow requires top-level records.",
                        kinds,
                    )
                type_count = len(rows[0]['union'])
                if type_count > 1:
                    raise MixedTypesError(
                        f'Query result contains {type_count} distinct types; results '
                        f'would be silently truncated. Use \'| blend\' to merge types '
                        f'into one, or pass safe=False to skip this check and accept '
                        f'partial results.',
                        type_count,
                    )
        r = self.query_raw(query)
        try:
            reader = pa.ipc.open_stream(r.raw)
        except pa.lib.ArrowInvalid as e:
            # An empty response body (no schema) means either the pool has no
            # data or the data contains a type the Arrow encoder can't handle
            # (e.g. an empty record).  Both cases are indistinguishable at the
            # HTTP level when streaming, so both are silently treated as an
            # empty result.  Any other ArrowInvalid (wrong format, mid-stream
            # corruption, etc.) is re-raised.
            if 'null or length 0' in str(e):
                return
            raise
        for batch in reader:
            yield from batch.to_pylist(maps_as_pydicts='strict')

    def query_raw(self, query, headers=None):
        r = self.session.post(self.base_url + '/query', headers=headers,
                              json={'query': query}, stream=True)
        self.__raise_for_status(r)
        r.raw.decode_content = True
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


class MixedTypesError(Exception):
    """Raised by query() when the result contains more than one distinct type."""
    def __init__(self, message, type_count):
        super().__init__(message)
        self.type_count = type_count


class NonRecordError(Exception):
    """Raised by query() when the result contains non-record top-level values."""
    def __init__(self, message, kinds):
        super().__init__(message)
        self.kinds = kinds


if __name__ == '__main__':
    import argparse
    import pprint

    parser = argparse.ArgumentParser(
        description='Query default SuperDB service and print results.',
        formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument('query')
    args = parser.parse_args()

    c = Client()
    for record in c.query(args.query):
        pprint.pprint(record)

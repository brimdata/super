# `superdb` Python Package

Visit <https://superdb.org/dev/libraries/python.html> for installation
instructions and example usage.

## Running the tests

Create and activate a virtual environment, install the package with its test
dependencies, and start a local SuperDB service:

```
python3 -m venv .venv
source .venv/bin/activate
pip3 install -e '.[test]'
super db serve
```

Then in another shell (with the virtual environment activated):

```
source .venv/bin/activate
pytest
```

Tests are skipped automatically if the SuperDB service is not reachable.

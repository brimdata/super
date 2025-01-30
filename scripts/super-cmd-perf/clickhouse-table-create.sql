SET enable_json_type = 1;
CREATE TABLE gha (v JSON) ENGINE MergeTree() ORDER BY tuple();
INSERT INTO gha SELECT * FROM file('gharchive_gz/*.json.gz', JSONAsObject);

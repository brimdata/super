SELECT count(),type
FROM '__SOURCE__'
WHERE repo.name='duckdb/duckdb'
GROUP BY type

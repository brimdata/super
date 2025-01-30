SET allow_suspicious_types_in_group_by = 1;
SELECT count(),v.type
FROM '__SOURCE__'
WHERE v.repo.name='duckdb/duckdb'
GROUP BY v.type

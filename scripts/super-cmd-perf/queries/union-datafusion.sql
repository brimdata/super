WITH assignees AS (
  SELECT payload.pull_request.assignee.login assignee
  FROM '__SOURCE__'
  UNION ALL
  SELECT object.login as assignee FROM (
    SELECT unnest(payload.pull_request.assignees) object
    FROM '__SOURCE__'
  )
)
SELECT assignee, count() count
FROM assignees
WHERE assignee IS NOT NULL
GROUP BY assignee
ORDER BY count DESC
LIMIT 5

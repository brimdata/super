WITH assignees AS (
  SELECT payload.pull_request.assignee.login assignee
  FROM '__SOURCE__'
  UNION ALL
  SELECT arrayJoin(payload.pull_request.assignees).login assignee
  FROM '__SOURCE__'
)
SELECT assignee, count(*) count
FROM assignees
WHERE assignee IS NOT NULL
GROUP BY assignee
ORDER BY count DESC
LIMIT 5

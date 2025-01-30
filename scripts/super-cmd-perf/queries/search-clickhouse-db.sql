SELECT count()
FROM '__SOURCE__'
WHERE v.payload.pull_request.body LIKE '%in case you have any feedback 😊%'

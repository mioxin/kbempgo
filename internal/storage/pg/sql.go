package pg

// const updateQueryTemplate = `

const deleteQueryTemplate = `
-- set sotr_id = NULL in deleted phones
WITH cte AS (
	SELECT {{.TableName}}.id, {{.TableName}}.sotr_deleted_id
	FROM {{.TableName}}, (VALUES {{.Values}}) AS v(sotr_id, {{.KeyField}})
	WHERE {{.TableName}}.sotr_id = v.sotr_id AND {{.TableName}}.{{.KeyField}} = v.{{.KeyField}}{{if .KeyType}}::{{.KeyType}}{{end}}
)

UPDATE {{.TableName}}
SET sotr_id = NULL
WHERE {{.TableName}}.id IN (SELECT id FROM cte WHERE sotr_deleted_id IS NOT NULL);

-- delete row if sotr_id = NULL and sotr_deleted_id IS NULL
WITH cte AS (
	SELECT {{.TableName}}.id, {{.TableName}}.sotr_deleted_id
	FROM {{.TableName}}, (VALUES {{.Values}}) AS v(sotr_id, {{.KeyField}})
	WHERE {{.TableName}}.sotr_id = v.sotr_id AND {{.TableName}}.{{.KeyField}} = v.{{.KeyField}}{{if .KeyType}}::{{.KeyType}}{{end}}
)
DELETE FROM {{.TableName}}
WHERE id IN (SELECT id FROM cte WHERE sotr_deleted_id IS NULL);
`

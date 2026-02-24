-- name: UpsertNamespacePolicy :exec
INSERT INTO namespace_policies (namespace, version, policy)
VALUES ($1, $2, $3)
ON CONFLICT (namespace, version) DO UPDATE SET policy = EXCLUDED.policy;

-- name: GetNamespacePolicies :many
SELECT id, namespace, version, policy
FROM namespace_policies
WHERE namespace = $1
ORDER BY version DESC;

package db

const (
	getObjectByIDSQL = `
SELECT * FROM object WHERE id=$1
`

	getObjectByNameSQL = `
SELECT * FROM object WHERE parent_id=$1 AND name=$2
`
	insertObjectSQL = `
INSERT INTO object (
	id, name, aliases, parent_id, ref_id, kind, hash, size, inode, namespace,
	created_at, changed_at, modified_at, access_at, data
) VALUES (
	:id, :name, :aliases, :parent_id, :ref_id, :kind, :hash, :size, :inode,
	:namespace, :created_at, :changed_at, :modified_at, :access_at, :data
);
`
	updateObjectSQL = `
UPDATE object
SET
	name=:name,
	aliases=:aliases,
	parent_id=:parent_id,
	ref_id=:ref_id,
	kind=:kind,
	hash=:hash,
	size=:size,
	inode=:inode,
	namespace=:namespace,
	created_at=:created_at,
	changed_at=:changed_at,
	modified_at=:modified_at,
	access_at=:access_at,
	data=:data
WHERE
	id=:id
`
	deleteObjectSQL = `
DELETE FROM object WHERE id=:id;
`
	insertObjectLabelSQL = `
INSERT INTO object_label (
	id, key, value
) VALUES (
	:id, :key, :value
);
`
	deleteObjectLabelSQL = `
DELETE FROM object_label WHERE id=:id;
`
	insertObjectContentSQL = `
INSERT INTO object_content (
	id, kind, version, data
) VALUES (
	:id, :kind, :version, :data
);
`
	updateObjectContentSQL = `
UPDATE object_content
SET
	data=:data
WHERE
	id=:id AND kind=:kind AND version=:version
`
	deleteObjectContentSQL = `
DELETE FROM object_custom WHERE id=:id;
`
)

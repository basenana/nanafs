package migrate

const (
	reversion20220520 = "20220520.1"
)

const reversion20220520Upgrade = `
CREATE TABLE object (
    id VARCHAR(32),
	name VARCHAR(512),
	aliases VARCHAR(512),
	parent_id VARCHAR(32),
	ref_id VARCHAR(32),
	kind VARCHAR(128),
	hash VARCHAR(512),
	size INTEGER(64),
	inode INTEGER(64),
	namespace VARCHAR(512),
	created_at DATETIME,
	changed_at DATETIME,
	modified_at DATETIME,
	access_at DATETIME,
	data BLOB,
	PRIMARY KEY (id)
);
CREATE TABLE object_label (
    id VARCHAR(32),
    key VARCHAR(128),
    value VARCHAR(512),
    CONSTRAINT pk_lid PRIMARY KEY (id,key)
);
CREATE TABLE object_content (
    id VARCHAR(32),
    kind VARCHAR(128),
    version VARCHAR(512),
    data BLOB,
    CONSTRAINT pk_cid PRIMARY KEY (id,kind,version)
);
`
const reversion20220520Downgrade = `
DROP TABLE object;
DROP TABLE object_label;
DROP TABLE object_content;
`

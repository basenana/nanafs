package migrate

const (
	reversion20220619 = "20220619.1"
)

const reversion20220619Upgrade = `
CREATE TABLE object_workflow (
    id VARCHAR(32),
	synced boolean,
	created_at DATETIME,
	updated_at DATETIME,
	PRIMARY KEY (id)
);
`
const reversion20220619Downgrade = `
DROP TABLE object_workflow;
`

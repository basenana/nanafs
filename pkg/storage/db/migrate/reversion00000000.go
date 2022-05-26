package migrate

const (
	reversion00000000 = "00000000.1"
)

const reversion00000000Upgrade = `
CREATE TABLE db_reversion (
    id VARCHAR(32),
    current VARCHAR(32),
	PRIMARY KEY (id)
);
`

const pingDbReversion = `
SELECT 1 FROM db_reversion;
`

const queryDbReversion = `
SELECT * FROM db_reversion WHERE id=$1;
`

const insertDbReversion = `
INSERT INTO db_reversion (
	id, current
) VALUES (
	:id, :current
);
`

const updateDbReversion = `
UPDATE db_reversion
SET
	current=:current
WHERE
	id=:id;
`

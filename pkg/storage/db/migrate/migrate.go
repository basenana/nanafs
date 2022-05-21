package migrate

import (
	"database/sql"
	"fmt"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

const (
	dbReversionKey = "schema_version"
)

type Migrate struct {
	db     *sqlx.DB
	logger *zap.SugaredLogger
}

func (m *Migrate) Current() (string, error) {
	_, tableCheck := m.db.Query(pingDbReversion)
	if tableCheck != nil {
		m.logger.Warnf("check db reversion table not pass: %s", tableCheck.Error())
		return "", nil
	}

	version := DbReversionModel{}
	if err := m.db.Get(&version, queryDbReversion, dbReversionKey); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return version.Current, nil
}

func (m *Migrate) HEAD() string {
	revList := m.Reversions()
	if len(revList) == 0 {
		return ""
	}
	return revList[len(revList)-1]
}

func (m *Migrate) Reversions() []string {
	return reversions
}

func (m *Migrate) UpgradeHead() error {
	return m.Upgrade(m.HEAD())
}

func (m *Migrate) Upgrade(reversion string) error {
	return m.exec(m.Reversions(), reversion, true)
}

func (m *Migrate) Downgrade(reversion string) error {
	var revList = m.Reversions()
	for i, j := 0, len(revList)-1; i < j; i, j = i+1, j-1 {
		revList[i], revList[j] = revList[j], revList[i]
	}
	return m.exec(revList, reversion, false)
}

func (m *Migrate) exec(revList []string, reversion string, upgrade bool) error {
	if reversion == "" || len(revList) == 0 {
		return fmt.Errorf("reversion was empty")
	}

	crt, err := m.Current()
	if err != nil {
		return err
	}

	var (
		crtIdx = -1
		tgtIdx = -1
	)

	if crt != "" {
		for i := range revList {
			if revList[i] == crt {
				crtIdx = i
				break
			}
		}
		if crtIdx == -1 {
			return fmt.Errorf("current reversion %s not in plan", crt)
		}
	}

	for i := range revList {
		if revList[i] == reversion {
			tgtIdx = i
			break
		}
	}
	if tgtIdx == -1 {
		return fmt.Errorf("taget reversion %s not in plan", reversion)
	}

	if crtIdx != -1 && tgtIdx == crtIdx {
		return nil
	}

	if crtIdx > tgtIdx {
		return fmt.Errorf("can not do a downgrade %s=>%s", crt, reversion)
	}

	var tx *sqlx.Tx
	for i := crtIdx + 1; i <= tgtIdx; i++ {
		m.logger.Infow("run migrate to %s", revList[i])
		act, ok := reversionRegistry[revList[i]]
		if !ok {
			return fmt.Errorf("reversion %s not found", revList[i])
		}

		tx, err = m.db.Beginx()
		if err != nil {
			return err
		}

		if upgrade && act.before != "" {
			_, err = tx.Exec(act.before)
			if err != nil {
				return fmt.Errorf("run reversion %s/before failed: %s", revList[i], err.Error())
			}
		}
		if upgrade && act.upgrade != "" {
			_, err = tx.Exec(act.upgrade)
			if err != nil {
				return fmt.Errorf("run reversion %s/upgrade failed: %s", revList[i], err.Error())
			}
		}
		if !upgrade && act.downgrade != "" {
			_, err = tx.Exec(act.downgrade)
			if err != nil {
				return fmt.Errorf("run reversion %s/downgrade failed: %s", revList[i], err.Error())
			}
		}
		if upgrade && act.after != "" {
			_, err = tx.Exec(act.before)
			if err != nil {
				return fmt.Errorf("run reversion %s/after failed: %s", revList[i], err.Error())
			}
		}

		if err = m.updateReversion(tx, revList[i]); err != nil {
			return fmt.Errorf("run reversion %s/commit failed: %s", revList[i], err.Error())
		}

		if err = tx.Commit(); err != nil {
			return fmt.Errorf("tx commit reversion %s failed: %s", revList[i], err.Error())
		}
		m.logger.Infow("migrate to %s succeed", revList[i])
	}

	return nil
}

func (m *Migrate) updateReversion(tx *sqlx.Tx, reversion string) error {
	_, err := tx.Exec(updateDbReversion, DbReversionModel{
		ID:      dbReversionKey,
		Current: reversion,
	})
	return err
}

func NewMigrateManager(db *sqlx.DB) *Migrate {
	return &Migrate{db: db, logger: logger.NewLogger("migrate")}
}

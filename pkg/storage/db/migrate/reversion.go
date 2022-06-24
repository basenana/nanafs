package migrate

var (
	reversionRegistry = make(map[string]reversion)
	reversions        = make([]string, 0)
)

type DbReversionModel struct {
	ID      string `json:"id"`
	Current string `json:"current"`
}

func register(uid string, r reversion) {
	reversionRegistry[uid] = r
	reversions = append(reversions, uid)
}

type reversion struct {
	before    string
	after     string
	upgrade   string
	downgrade string
}

func init() {
	register(reversion00000000, reversion{
		upgrade: reversion00000000Upgrade,
	})
	register(reversion20220520, reversion{
		upgrade:   reversion20220520Upgrade,
		downgrade: reversion20220520Downgrade,
	})
	register(reversion20220619, reversion{
		upgrade:   reversion20220619Upgrade,
		downgrade: reversion20220619Downgrade,
	})
}

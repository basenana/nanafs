package config

type verifier func(config *Config) error

var verifiers = []verifier{
	setDefaultValue,
}

func setDefaultValue(config *Config) error {
	if config.Owner == nil {
		config.Owner = defaultOwner()
	}
	return nil
}

func Verify(cfg *Config) error {
	for _, f := range verifiers {
		if err := f(cfg); err != nil {
			return err
		}
	}
	return nil
}

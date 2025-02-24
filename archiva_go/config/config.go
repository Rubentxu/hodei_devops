package config

type Config struct {
	RepoDir string
}

func LoadConfig() *Config {
	return &Config{
		RepoDir: "./repository",
	}
}

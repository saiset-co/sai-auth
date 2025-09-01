package types

import "time"

type SaiAuthConfig struct {
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
	BcryptCost      int           `yaml:"bcrypt_cost"`
	SuperUser       struct {
		AllowedIPs []string `yaml:"allowed_ips"`
	} `yaml:"super_user"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

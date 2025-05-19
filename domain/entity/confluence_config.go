package entity

type ConfluenceConfig struct {
	AncestorID string `mapstructure:"ancestor_id"`
	Space      string `mapstructure:"space"`
	Domain     string `mapstructure:"domain"`
}

package entity

type IncidentLevel struct {
	Description string `mapstructure:"description" validate:"required"`
	Level       int    `mapstructure:"level" validate:"required,gte=0"`
	Disabled    bool   `mapstructure:"disabled"`
}

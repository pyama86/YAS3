package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/pyama86/YAS3/domain/entity"
	"github.com/spf13/viper"
)

func NewConfigRepository(path string) (*Config, error) {
	viper.SetConfigFile(path)

	viper.AutomaticEnv()

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("read config error: %w", err)
	}

	var c Config
	err = viper.Unmarshal(&c)
	if err != nil {
		return nil, fmt.Errorf("unmarshal config error: %w", err)
	}
	valid := validator.New()
	if err = valid.Struct(c); err != nil {
		return nil, fmt.Errorf("validate config error: %w", err)
	}

	return &c, nil
}

type Config struct {
	ServiceList             []entity.Service       `mapstructure:"services" validate:"required"`
	AnnouncementChannelList []string               `mapstructure:"announcement_channels"`
	IncidentLevelList       []entity.IncidentLevel `mapstructure:"incident_levels" validate:"required"`
	Confluence              ConfluenceConfig       `mapstructure:"confluence"`
}

type ConfluenceConfig struct {
	AncestorID string `mapstructure:"ancestor_id"`
	Space      string `mapstructure:"space"`
	Domain     string `mapstructure:"domain"`
}

func (c *Config) Services(_ context.Context) ([]entity.Service, error) {
	var services []entity.Service
	for _, service := range c.ServiceList {
		if service.Disabled {
			continue
		}
		services = append(services, service)
	}
	return services, nil
}

func (c *Config) ServiceByID(_ context.Context, id int) (*entity.Service, error) {
	for _, service := range c.ServiceList {
		if service.ID == id {
			return &service, nil
		}
	}
	return nil, fmt.Errorf("service not found")
}

func (c *Config) AnnouncementChannels(_ context.Context) []string {
	return c.AnnouncementChannelList
}

func (c *Config) IncidentLevels(_ context.Context) []entity.IncidentLevel {
	var levels []entity.IncidentLevel
	for _, level := range c.IncidentLevelList {
		if level.Disabled {
			continue
		}
		levels = append(levels, level)
	}
	return levels
}

// インシデントレベルをIDで取得
func (c *Config) IncidentLevelByLevel(_ context.Context, id int) (*entity.IncidentLevel, error) {
	if id == 0 {
		return &entity.IncidentLevel{
			Level:       0,
			Description: "サービス影響なし",
		}, nil
	}
	for _, level := range c.IncidentLevelList {
		if level.Level == id {
			return &level, nil
		}
	}
	return nil, fmt.Errorf("incident level not found")
}

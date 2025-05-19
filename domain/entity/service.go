package entity

type Service struct {
	ID                   int      `mapstructure:"id" validate:"required"`
	Name                 string   `mapstructure:"name" validate:"required"`
	Disabled             bool     `mapstructure:"disabled"`
	IncidentTeamMembers  []string `mapstructure:"incident_team_members"`
	AnnouncementChannels []string `mapstructure:"announcement_channels"`
}

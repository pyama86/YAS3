package model

type IncidentTeam struct {
	Base
	Name                string   `json:"name"`
	ServiceID           int      `json:"service_id" index:"service_id-index"`
	IncidentTeamMembers []string `json:"incident_team_members"`
}

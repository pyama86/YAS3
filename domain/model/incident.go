package model

type IncidentTypeEnum int

const (
	IncidentTypeEnumSecurity IncidentTypeEnum = iota
	IncidentTypeEnumService
	IncidentTypeEnumOther
)

type Incident struct {
	Base
	IncidentType  IncidentTypeEnum `json:"incident_type"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	ServiceID     int              `json:"service_id" index:"service_id-index"`
	HandlerID     string           `json:"handler_id" index:"handler_id-index"`
	PostMortemURL string           `json:"post_mortem_url"`
}

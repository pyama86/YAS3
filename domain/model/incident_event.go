package model

type IncidentEventTypeEnum int

const (
	// 発生
	IncidentEventTypeEnumOccurrence IncidentEventTypeEnum = iota
	// 解決
	IncidentEventTypeEnumResolution
	// クローズ
	IncidentEventTypeEnumClose
)

type IncidentEvent struct {
	Base
	IncidentID        int                   `json:"incident_id" index:"incident_id-index"`
	IncidentEventType IncidentEventTypeEnum `json:"incident_type"`
}

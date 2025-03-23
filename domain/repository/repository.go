package repository

import (
	"context"

	"github.com/pyama86/YAS3/domain/entity"
)

type IncidentRepositoryer interface {
	FindIncidentByChannel(context.Context, string) (*entity.Incident, error)
	SaveIncident(context.Context, *entity.Incident) error
	ActiveIncidents(context.Context) ([]entity.Incident, error)
}

type ServiceRepositoryer interface {
	Services(context.Context) ([]entity.Service, error)
	ServiceByID(context.Context, int) (*entity.Service, error)
	AnnouncementChannels(context.Context) []string
}

type IncidentLevelRepositoryer interface {
	IncidentLevels(context.Context) []entity.IncidentLevel
	IncidentLevelByLevel(context.Context, int) (*entity.IncidentLevel, error)
}

type Repository interface {
	IncidentRepositoryer
	ServiceRepositoryer
	IncidentLevelRepositoryer
	SlackRepositoryer
}

type RepositoryFacade struct {
	IncidentRepositoryer
	ServiceRepositoryer
	IncidentLevelRepositoryer
	SlackRepositoryer
}

type PostMortemRepositoryer interface {
	ExportPostMortem(context.Context, string, string) (string, error)
}

func NewRepository(
	incidentRepository IncidentRepositoryer,
	serviceRepository ServiceRepositoryer,
	incidentLevelRepository IncidentLevelRepositoryer,
	slackRepository SlackRepositoryer,
) Repository {
	return RepositoryFacade{
		IncidentRepositoryer:      incidentRepository,
		ServiceRepositoryer:       serviceRepository,
		IncidentLevelRepositoryer: incidentLevelRepository,
		SlackRepositoryer:         slackRepository,
	}
}

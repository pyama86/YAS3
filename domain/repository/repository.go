package repository

import (
	"context"

	"github.com/pyama86/YAS3/domain/entity"
)

type IncidentRepository interface {
	FindIncidentByChannel(context.Context, string) (*entity.Incident, error)
	SaveIncident(context.Context, *entity.Incident) error
	ActiveIncidents(context.Context) ([]entity.Incident, error)
}

type ServiceRepository interface {
	Services(context.Context) ([]entity.Service, error)
	ServiceByID(context.Context, int) (*entity.Service, error)
	AnnouncementChannels(context.Context) []string
}

type IncidentLevelRepository interface {
	IncidentLevels(context.Context) []entity.IncidentLevel
	IncidentLevelByLevel(context.Context, int) (*entity.IncidentLevel, error)
}

type Repository interface {
	IncidentRepository
	ServiceRepository
	IncidentLevelRepository
}

type RepositoryFacade struct {
	IncidentRepository
	ServiceRepository
	IncidentLevelRepository
}

type PostMortemExporter interface {
	ExportPostMortem(context.Context, string, string) (string, error)
}

func NewRepository(incidentRepository IncidentRepository, serviceRepository ServiceRepository, incidentLevelRepository IncidentLevelRepository) Repository {
	return RepositoryFacade{
		IncidentRepository:      incidentRepository,
		ServiceRepository:       serviceRepository,
		IncidentLevelRepository: incidentLevelRepository,
	}
}

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

func NewRepository(configPath string) (Repository, error) {
	cfgRepository, err := NewConfigRepository(configPath)
	if err != nil {
		return nil, err
	}
	dynamoRepository, err := NewDynamoDBRepository()
	if err != nil {
		return nil, err
	}

	return RepositoryFacade{
		IncidentRepository:      dynamoRepository,
		ServiceRepository:       cfgRepository,
		IncidentLevelRepository: cfgRepository,
	}, nil
}

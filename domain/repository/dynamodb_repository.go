package repository

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/guregu/dynamo/v2"
	"github.com/pyama86/YAS3/domain/entity"
)

var incidentsTable = "incidents"

func init() {
	if os.Getenv("DYNAMO_INCIDENTS_TABLE") != "" {
		incidentsTable = os.Getenv("DYNAMO_INCIDENTS_TABLE")
	}
}

func NewDynamoDBRepository() (*DynamoDBRepository, error) {
	var db *dynamo.DB
	if os.Getenv("DYNAMO_LOCAL") != "" {
		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithRegion("dummy"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummy", "dummy")),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load configuration: %v", err)
		}
		db = dynamo.New(cfg, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String("http://localhost:8000")
		},
		)

		err = setupDdbSchema(db)
		if err != nil {
			return nil, fmt.Errorf("failed to setup schema: %v", err)
		}
	} else {
		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("failed to load configuration: %v", err)
		}
		db = dynamo.New(cfg)
	}

	return &DynamoDBRepository{db: db}, nil
}

func setupDdbSchema(db *dynamo.DB) error {
	t := db.Table(incidentsTable)
	_, err := t.Describe().Run(context.TODO())
	if err != nil {

		input := db.CreateTable(incidentsTable, entity.Incident{}).
			Provision(10, 10)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		return input.Run(ctx)
	}
	return nil
}

type DynamoDBRepository struct {
	db *dynamo.DB
}

func (r *DynamoDBRepository) FindIncidentByChannel(ctx context.Context, channel string) (*entity.Incident, error) {
	incident := &entity.Incident{}
	err := r.db.Table(incidentsTable).Get("channel_id", channel).One(ctx, incident)
	if err != nil {
		if err == dynamo.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return incident, nil
}

func (r *DynamoDBRepository) SaveIncident(ctx context.Context, incident *entity.Incident) error {
	return r.db.Table(incidentsTable).Put(incident).Run(ctx)
}

// closed_atが0のものを取得
func (r *DynamoDBRepository) ActiveIncidents(ctx context.Context) ([]entity.Incident, error) {
	var incidents []entity.Incident
	t := time.Time{}
	err := r.db.Table(incidentsTable).Scan().Filter("'closed_at' = ?", t).All(ctx, &incidents)
	if err != nil {
		return nil, err
	}
	return incidents, nil
}

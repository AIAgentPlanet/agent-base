package dao

import (
	"context"

	"gorm.io/gorm"

	"agent-base/services/user-service/internal/model"
	"github.com/go-dev-frame/sponge/pkg/sgorm/query"
)

// ATHAgentDao ath agent DAO interface
type ATHAgentDao interface {
	Create(ctx context.Context, table *model.ATHAgent) error
	DeleteByID(ctx context.Context, id uint64) error
	UpdateByID(ctx context.Context, table *model.ATHAgent) error
	GetByID(ctx context.Context, id uint64) (*model.ATHAgent, error)
	GetByClientID(ctx context.Context, clientID string) (*model.ATHAgent, error)
	GetByAgentID(ctx context.Context, agentID string) (*model.ATHAgent, error)
	GetByCondition(ctx context.Context, condition *query.Conditions) (*model.ATHAgent, error)
	GetByColumns(ctx context.Context, params *query.Params) ([]*model.ATHAgent, int64, error)
}

// athAgentDao implements ATHAgentDao
type athAgentDao struct {
	db *gorm.DB
}

// NewATHAgentDao create a dao
func NewATHAgentDao(db *gorm.DB) ATHAgentDao {
	return &athAgentDao{db: db}
}

func (d *athAgentDao) Create(ctx context.Context, table *model.ATHAgent) error {
	return d.db.WithContext(ctx).Create(table).Error
}

func (d *athAgentDao) DeleteByID(ctx context.Context, id uint64) error {
	return d.db.WithContext(ctx).Where("id = ?", id).Delete(&model.ATHAgent{}).Error
}

func (d *athAgentDao) UpdateByID(ctx context.Context, table *model.ATHAgent) error {
	return d.db.WithContext(ctx).Model(table).Updates(table).Error
}

func (d *athAgentDao) GetByID(ctx context.Context, id uint64) (*model.ATHAgent, error) {
	var record model.ATHAgent
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (d *athAgentDao) GetByClientID(ctx context.Context, clientID string) (*model.ATHAgent, error) {
	var record model.ATHAgent
	err := d.db.WithContext(ctx).Where("client_id = ?", clientID).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (d *athAgentDao) GetByAgentID(ctx context.Context, agentID string) (*model.ATHAgent, error) {
	var record model.ATHAgent
	err := d.db.WithContext(ctx).Where("agent_id = ?", agentID).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (d *athAgentDao) GetByCondition(ctx context.Context, condition *query.Conditions) (*model.ATHAgent, error) {
	queryStr, args, err := condition.ConvertToGorm(query.WithWhitelistNames(model.ATHAgentColumnNames))
	if err != nil {
		return nil, err
	}
	var record model.ATHAgent
	err = d.db.WithContext(ctx).Where(queryStr, args...).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (d *athAgentDao) GetByColumns(ctx context.Context, params *query.Params) ([]*model.ATHAgent, int64, error) {
	var total int64
	tables := make([]*model.ATHAgent, 0)

	queryStr, args, err := params.ConvertToGormConditions(query.WithWhitelistNames(model.ATHAgentColumnNames))
	if err != nil {
		return nil, 0, err
	}

	err = d.db.WithContext(ctx).Model(&model.ATHAgent{}).Where(queryStr, args...).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return tables, 0, nil
	}

	queryStr, args, err = params.ConvertToGormConditions(query.WithWhitelistNames(model.ATHAgentColumnNames))
	if err != nil {
		return nil, 0, err
	}

	err = d.db.WithContext(ctx).Where(queryStr, args...).Order(params.Sort).Limit(params.Limit).Offset((params.Page - 1) * params.Limit).Find(&tables).Error
	if err != nil {
		return nil, 0, err
	}

	return tables, total, nil
}

package dao

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/go-dev-frame/sponge/pkg/sgorm/query"

	"agent-base/services/user-service/internal/model"
)

var _ OAuthClientDao = (*oauthClientDao)(nil)

// OAuthClientDao defining the dao interface
type OAuthClientDao interface {
	Create(ctx context.Context, table *model.OAuthClient) error
	DeleteByID(ctx context.Context, id uint64) error
	UpdateByID(ctx context.Context, table *model.OAuthClient) error
	GetByID(ctx context.Context, id uint64) (*model.OAuthClient, error)
	GetByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error)
	GetByColumns(ctx context.Context, params *query.Params) ([]*model.OAuthClient, int64, error)
	GetByCondition(ctx context.Context, condition *query.Conditions) (*model.OAuthClient, error)
}

type oauthClientDao struct {
	db *gorm.DB
}

// NewOAuthClientDao creating the dao interface
func NewOAuthClientDao(db *gorm.DB) OAuthClientDao {
	return &oauthClientDao{db: db}
}

// Create a new oauth client
func (d *oauthClientDao) Create(ctx context.Context, table *model.OAuthClient) error {
	return d.db.WithContext(ctx).Create(table).Error
}

// DeleteByID delete a oauth client by id
func (d *oauthClientDao) DeleteByID(ctx context.Context, id uint64) error {
	return d.db.WithContext(ctx).Where("id = ?", id).Delete(&model.OAuthClient{}).Error
}

// UpdateByID update a oauth client by id
func (d *oauthClientDao) UpdateByID(ctx context.Context, table *model.OAuthClient) error {
	if table.ID < 1 {
		return errors.New("id cannot be 0")
	}

	update := map[string]interface{}{}
	if table.Name != "" {
		update["name"] = table.Name
	}
	if table.RedirectURIs != "" {
		update["redirect_uris"] = table.RedirectURIs
	}
	if table.AllowedGrants != "" {
		update["allowed_grants"] = table.AllowedGrants
	}
	if table.AllowedScopes != "" {
		update["allowed_scopes"] = table.AllowedScopes
	}
	if table.Status != 0 {
		update["status"] = table.Status
	}

	return d.db.WithContext(ctx).Model(table).Updates(update).Error
}

// GetByID get a oauth client by id
func (d *oauthClientDao) GetByID(ctx context.Context, id uint64) (*model.OAuthClient, error) {
	record := &model.OAuthClient{}
	err := d.db.WithContext(ctx).Where("id = ?", id).First(record).Error
	return record, err
}

// GetByClientID get a oauth client by client_id
func (d *oauthClientDao) GetByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	record := &model.OAuthClient{}
	err := d.db.WithContext(ctx).Where("client_id = ?", clientID).First(record).Error
	return record, err
}

// GetByColumns get a paginated list of oauth clients by custom conditions
func (d *oauthClientDao) GetByColumns(ctx context.Context, params *query.Params) ([]*model.OAuthClient, int64, error) {
	queryStr, args, err := params.ConvertToGormConditions(query.WithWhitelistNames(model.OAuthClientColumnNames))
	if err != nil {
		return nil, 0, errors.New("query params error: " + err.Error())
	}

	var total int64
	if params.Sort != "ignore count" {
		err = d.db.WithContext(ctx).Model(&model.OAuthClient{}).Where(queryStr, args...).Count(&total).Error
		if err != nil {
			return nil, 0, err
		}
		if total == 0 {
			return nil, total, nil
		}
	}

	records := []*model.OAuthClient{}
	order, limit, offset := params.ConvertToPage()
	err = d.db.WithContext(ctx).Order(order).Limit(limit).Offset(offset).Where(queryStr, args...).Find(&records).Error
	if err != nil {
		return nil, 0, err
	}

	return records, total, err
}

// GetByCondition get a oauth client by custom condition
func (d *oauthClientDao) GetByCondition(ctx context.Context, c *query.Conditions) (*model.OAuthClient, error) {
	queryStr, args, err := c.ConvertToGorm(query.WithWhitelistNames(model.OAuthClientColumnNames))
	if err != nil {
		return nil, err
	}

	table := &model.OAuthClient{}
	err = d.db.WithContext(ctx).Where(queryStr, args...).First(table).Error
	if err != nil {
		return nil, err
	}

	return table, nil
}

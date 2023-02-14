package pool

import (
	"github.com/evgeniums/go-backend-helpers/pkg/common"
)

type PoolServiceAssociation interface {
	common.Object
	Pool() string
	Role() string
	Service() string
}

type PoolId struct {
	POOL_ID string `gorm:"index;index:,unique,composite:u" json:"pool_id"`
}

type WithRole struct {
	ROLE string `gorm:"index;index:,unique,composite:u" json:"type" validate:"required,alphanum_" vmessage:"Role name can contain only digits and letters"`
}

type PoolServiceAssociationCmd struct {
	WithRole
	SERVICE_ID string `gorm:"index" json:"service_id" validate:"required" vmessage:"Service ID can not be empty"`
}

type PoolServiceAssociationEssentials struct {
	PoolId
	PoolServiceAssociationCmd
}

type PoolServiceAssociationBase struct {
	common.ObjectBase
	PoolServiceAssociationEssentials
}

func (p *PoolServiceAssociationEssentials) Pool() string {
	return p.POOL_ID
}

func (p *WithRole) Role() string {
	return p.ROLE
}

func (p *PoolServiceAssociationCmd) Service() string {
	return p.SERVICE_ID
}

func (PoolServiceAssociationBase) TableName() string {
	return "pool_service_associations"
}

type PoolServiceBinding struct {
	common.ObjectBase
	WithRole
	PoolId    string `json:"pools.id"`
	ServiceId string `json:"pool_services.id"`
}

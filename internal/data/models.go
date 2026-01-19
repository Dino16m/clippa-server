package data

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Party struct {
	gorm.Model
	ID            uuid.UUID `gorm:"primarykey"`
	Name          string
	Password      string
	LeaderAddress string
	CertPEM       string    `gorm:"type:text"`
	KeyPEM        string    `gorm:"type:text"`
}

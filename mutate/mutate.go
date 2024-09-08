package mutate

import (
	"gorm.io/gorm"

	"github.com/sugawani/testcontainers-go-with-flyway/models"
)

type Mutate struct {
	db *gorm.DB
}

func NewMutate(db *gorm.DB) *Mutate {
	return &Mutate{db: db}
}

func (m *Mutate) Execute(name string) (*models.User, error) {
	u := models.NewUser(name)
	if err := m.db.Create(&u).Error; err != nil {
		return nil, err
	}
	return u, nil
}

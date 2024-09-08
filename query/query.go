package query

import (
	"gorm.io/gorm"

	main "github.com/sugawani/testcontainers-go-with-flyway/models"
)

type Query struct {
	db *gorm.DB
}

func NewQuery(db *gorm.DB) *Query {
	return &Query{db: db}
}

func (q *Query) Execute(userID main.ID) (*main.User, error) {
	var u *main.User
	if err := q.db.First(&u, userID).Error; err != nil {
		return nil, err
	}

	return u, nil
}

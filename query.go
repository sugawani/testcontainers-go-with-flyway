package main

import (
	"gorm.io/gorm"
)

type Query struct {
	db *gorm.DB
}

func NewQuery(db *gorm.DB) *Query {
	return &Query{db: db}
}

func (q *Query) Execute(userID ID) (*User, error) {
	var u *User
	if err := q.db.First(&u, userID).Error; err != nil {
		return nil, err
	}

	return u, nil
}

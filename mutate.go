package main

import (
	"gorm.io/gorm"
)

type Mutate struct {
	db *gorm.DB
}

func NewMutate(db *gorm.DB) *Mutate {
	return &Mutate{db: db}
}

func (m *Mutate) Execute(name string) (*User, error) {
	u := NewUser(name)
	if err := m.db.Create(&u).Error; err != nil {
		return nil, err
	}
	return u, nil
}

package mutate

import (
	"database/sql"

	"github.com/sugawani/testcontainers-go-with-flyway/models"
)

type Mutate struct {
	db *sql.DB
}

func NewMutate(db *sql.DB) *Mutate {
	return &Mutate{db: db}
}

func (m *Mutate) Execute(name string) (*models.User, error) {
	exec, err := m.db.Exec("INSERT INTO users (name) VALUES (?)", name)
	if err != nil {
		return nil, err
	}
	id, err := exec.LastInsertId()
	if err != nil {
		return nil, err
	}
	u := models.NewUser(models.ID(id), name)
	return u, nil
}

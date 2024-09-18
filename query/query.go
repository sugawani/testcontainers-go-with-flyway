package query

import (
	"database/sql"

	main "github.com/sugawani/testcontainers-go-with-flyway/models"
)

type Query struct {
	db *sql.DB
}

func NewQuery(db *sql.DB) *Query {
	return &Query{db: db}
}

func (q *Query) Execute(userID main.ID) (*main.User, error) {
	var u main.User
	row := q.db.QueryRow("SELECT * FROM users WHERE id = ?", userID)
	err := row.Scan(&u.ID, &u.Name)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

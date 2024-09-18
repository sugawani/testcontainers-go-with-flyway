package models

type ID int64

type User struct {
	ID   ID
	Name string
}

func NewUser(id ID, name string) *User {
	return &User{
		ID:   id,
		Name: name,
	}
}

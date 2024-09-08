package models

type ID int

type User struct {
	ID   ID
	Name string
}

func NewUser(name string) *User {
	return &User{
		Name: name,
	}
}

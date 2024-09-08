package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func beforeCleanupUser(db *gorm.DB, t *testing.T) {
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&User{}).Error; err != nil {
		t.Fatal("failed to beforeCleanup", err)
	}
}

func Test_Mutate(t *testing.T) {
	cases := map[string]struct {
		want string
	}{
		"create user":  {want: "created user1"},
		"create user2": {want: "created user2"},
		"create user3": {want: "created user3"},
		"create user4": {want: "created user4"},
	}

	ctx := context.Background()
	db, cleanup := NewTestDB(ctx)
	t.Cleanup(cleanup)

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			beforeCleanupUser(db, t)

			m := NewMutate(db)
			actual, err := m.Execute(tt.want)
			assert.Equal(t, tt.want, actual.Name)
			assert.NoError(t, err)
		})
	}
}

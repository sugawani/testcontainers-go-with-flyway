package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func Test_Query(t *testing.T) {
	createUser := func(db *gorm.DB) {
		db.Create(&User{ID: 1, Name: "name"})
	}
	noCreateUser := func(db *gorm.DB) {}
	wantErrAssertFunc := func(t assert.TestingT, err error, i ...interface{}) bool {
		return assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	}
	cases := map[string]struct {
		createFunc func(db *gorm.DB)
		want       *User
		assertErr  assert.ErrorAssertionFunc
	}{
		"user exists":      {createFunc: createUser, want: &User{ID: 1, Name: "name"}, assertErr: assert.NoError},
		"user exists2":     {createFunc: createUser, want: &User{ID: 1, Name: "name"}, assertErr: assert.NoError},
		"user exists3":     {createFunc: createUser, want: &User{ID: 1, Name: "name"}, assertErr: assert.NoError},
		"user not exists":  {createFunc: noCreateUser, want: nil, assertErr: wantErrAssertFunc},
		"user not exists2": {createFunc: noCreateUser, want: nil, assertErr: wantErrAssertFunc},
		"user not exists3": {createFunc: noCreateUser, want: nil, assertErr: wantErrAssertFunc},
	}

	ctx := context.Background()
	db, cleanup := NewTestDB(ctx)
	t.Cleanup(cleanup)

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			beforeCleanupUser(db, t)

			tt.createFunc(db)
			q := NewQuery(db)
			actual, err := q.Execute(1)
			assert.Equal(t, tt.want, actual)
			tt.assertErr(t, err)
		})
	}
}

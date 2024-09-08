package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func Test_Query(t *testing.T) {
	createUser := func(db *gorm.DB) {
		db.Create(&User{ID: 1, Name: "name"})
	}
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
		"user not exists":  {createFunc: func(*gorm.DB) {}, want: nil, assertErr: wantErrAssertFunc},
		"user not exists2": {createFunc: func(*gorm.DB) {}, want: nil, assertErr: wantErrAssertFunc},
		"user not exists3": {createFunc: func(*gorm.DB) {}, want: nil, assertErr: wantErrAssertFunc},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			db, cleanup, nw := NewTestDB(ctx)
			fmt.Printf("test: %s, network name: %s\n", name, nw)
			t.Cleanup(cleanup)

			tt.createFunc(db)

			q := NewQuery(db)
			actual, err := q.Execute(1)
			if !assert.Equal(t, tt.want, actual) {
				var us []*User
				db.Find(&us)
				fmt.Printf("assertion error. users: %v\n", us)
			}
			tt.assertErr(t, err)
		})
	}
}

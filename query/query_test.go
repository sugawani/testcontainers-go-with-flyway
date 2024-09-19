package query

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/sugawani/testcontainers-go-with-flyway/models"
	"github.com/sugawani/testcontainers-go-with-flyway/util"
)

func beforeCleanupUser(db *gorm.DB, t *testing.T) {
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.User{}).Error; err != nil {
		t.Fatal("failed to beforeCleanup", err)
	}
}

func Test_Query(t *testing.T) {
	createUser := func(db *gorm.DB) {
		db.Create(&models.User{ID: 1, Name: "name"})
	}
	noCreateUser := func(db *gorm.DB) {}
	wantErrAssertFunc := func(t assert.TestingT, err error, i ...interface{}) bool {
		return assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	}
	cases := map[string]struct {
		createFunc func(db *gorm.DB)
		want       *models.User
		assertErr  assert.ErrorAssertionFunc
	}{
		"user exists":      {createFunc: createUser, want: &models.User{ID: 1, Name: "name"}, assertErr: assert.NoError},
		"user exists2":     {createFunc: createUser, want: &models.User{ID: 1, Name: "name"}, assertErr: assert.NoError},
		"user exists3":     {createFunc: createUser, want: &models.User{ID: 1, Name: "name"}, assertErr: assert.NoError},
		"user not exists":  {createFunc: noCreateUser, want: nil, assertErr: wantErrAssertFunc},
		"user not exists2": {createFunc: noCreateUser, want: nil, assertErr: wantErrAssertFunc},
		"user not exists3": {createFunc: noCreateUser, want: nil, assertErr: wantErrAssertFunc},
	}
	
	ctx := context.Background()
	db, err := util.NewTestDB(ctx)
	if err != nil {
		t.Fatal("failed to create test db", err)
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			beforeCleanupUser(db, t)

			tt.createFunc(db)
			q := NewQuery(db)
			actual, err := q.Execute(1)
			tt.assertErr(t, err)
			assert.Equal(t, tt.want, actual)
		})
	}
}

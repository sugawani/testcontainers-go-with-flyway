package mutate

import (
	"context"
	"fmt"
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

func Test_Mutate(t *testing.T) {
	cases := map[string]struct {
		want string
	}{
		"create user":  {want: "created user1"},
		"create user2": {want: "created user2"},
		"create user3": {want: "created user3"},
		"create user4": {want: "created user4"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			db, cleanup, host, port := util.NewTestDB(ctx)
			fmt.Printf("name: %s, host: %s, port: %s\n", name, host, port)

			t.Cleanup(cleanup)
			beforeCleanupUser(db, t)

			m := NewMutate(db)
			actual, err := m.Execute(tt.want)
			assert.Equal(t, tt.want, actual.Name)
			assert.NoError(t, err)
		})
	}
}

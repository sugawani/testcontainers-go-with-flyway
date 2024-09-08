package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Mutate(t *testing.T) {
	cases := map[string]struct {
		want *User
	}{
		"create user":  {want: &User{ID: 1, Name: "created user1"}},
		"create user2": {want: &User{ID: 1, Name: "created user2"}},
		"create user3": {want: &User{ID: 1, Name: "created user3"}},
		"create user4": {want: &User{ID: 1, Name: "created user4"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			db, cleanup, nw, dsn := NewTestDB(ctx)
			fmt.Printf("test: %s, network name: %s, dsn: %s\n", name, nw, dsn)
			t.Cleanup(cleanup)

			m := NewMutate(db)
			actual, err := m.Execute(tt.want.Name)
			if !assert.Equal(t, tt.want, actual) {
				var us []*User
				db.Find(&us)
				for _, u := range us {
					fmt.Printf("assertion error. user.ID: %d, user.Name: %s\n", u.ID, u.Name)
				}
			}
			assert.NoError(t, err)
		})
	}
}

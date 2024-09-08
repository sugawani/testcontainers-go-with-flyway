package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Mutate(t *testing.T) {
	cases := map[string]struct {
		want *User
	}{
		"create user":  {want: &User{ID: 1, Name: "created user"}},
		"create user2": {want: &User{ID: 1, Name: "created user"}},
		"create user3": {want: &User{ID: 1, Name: "created user"}},
		"create user4": {want: &User{ID: 1, Name: "created user"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			db, cleanup := NewTestDB(ctx)
			t.Cleanup(cleanup)

			m := NewMutate(db)
			actual, err := m.Execute("created user")
			assert.Equal(t, tt.want, actual)
			assert.NoError(t, err)
		})
	}
}

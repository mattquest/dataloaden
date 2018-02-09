package example

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUserLoader(t *testing.T) {
	var fetches [][]string
	var mu sync.Mutex

	dl := &UserLoader{
		wait:     10 * time.Millisecond,
		maxBatch: 5,
		fetch: func(keys []string) ([]*User, []error) {
			mu.Lock()
			fetches = append(fetches, keys)
			mu.Unlock()

			users := make([]*User, len(keys))
			errors := make([]error, len(keys))

			for i, key := range keys {
				if strings.HasPrefix(key, "E") {
					errors[i] = fmt.Errorf("user not found")
				} else {
					users[i] = &User{ID: key, Name: "user " + key}
				}
			}
			return users, errors
		},
	}

	t.Run("fetch concurrent data", func(t *testing.T) {
		t.Run("load user successfully", func(t *testing.T) {
			t.Parallel()
			u, err := dl.Load("U1")
			require.NoError(t, err)
			require.Equal(t, u.ID, "U1")
		})

		t.Run("load failed user", func(t *testing.T) {
			t.Parallel()
			u, err := dl.Load("E1")
			require.Error(t, err)
			require.Nil(t, u)
		})

		t.Run("load many users", func(t *testing.T) {
			t.Parallel()
			u, err := dl.LoadAll([]string{"U2", "E2", "E3", "U4"})
			require.Equal(t, u[0].Name, "user U2")
			require.Equal(t, u[3].Name, "user U4")
			require.Error(t, err[1])
			require.Error(t, err[2])
		})

		t.Run("load thunk", func(t *testing.T) {
			t.Parallel()
			thunk1 := dl.LoadThunk("U5")
			thunk2 := dl.LoadThunk("E5")

			u1, err1 := thunk1()
			require.NoError(t, err1)
			require.Equal(t, "user U5", u1.Name)

			u2, err2 := thunk2()
			require.Error(t, err2)
			require.Nil(t, u2)
		})
	})

	t.Run("it sent two batches", func(t *testing.T) {
		mu.Lock()
		defer mu.Unlock()

		require.Len(t, fetches, 2)
		require.Len(t, fetches[0], 5)
		require.Len(t, fetches[1], 3)
	})

	t.Run("fetch more", func(t *testing.T) {

		t.Run("previously cached", func(t *testing.T) {
			t.Parallel()
			u, err := dl.Load("U1")
			require.NoError(t, err)
			require.Equal(t, u.ID, "U1")
		})

		t.Run("load many users", func(t *testing.T) {
			t.Parallel()
			u, err := dl.LoadAll([]string{"U2", "U4"})
			require.NoError(t, err[0])
			require.NoError(t, err[1])
			require.Equal(t, u[0].Name, "user U2")
			require.Equal(t, u[1].Name, "user U4")
		})
	})

	t.Run("no round trips", func(t *testing.T) {
		mu.Lock()
		defer mu.Unlock()

		require.Len(t, fetches, 2)
	})

	t.Run("fetch partial", func(t *testing.T) {
		t.Run("errors not in cache cache value", func(t *testing.T) {
			t.Parallel()
			u, err := dl.Load("E2")
			require.Nil(t, u)
			require.Error(t, err)
		})

		t.Run("load all", func(t *testing.T) {
			t.Parallel()
			u, err := dl.LoadAll([]string{"U1", "U4", "E1", "U9", "U5"})
			require.Equal(t, u[0].ID, "U1")
			require.Equal(t, u[1].ID, "U4")
			require.Error(t, err[2])
			require.Equal(t, u[3].ID, "U9")
			require.Equal(t, u[4].ID, "U5")
		})
	})

	t.Run("one partial trip", func(t *testing.T) {
		mu.Lock()
		defer mu.Unlock()

		require.Len(t, fetches, 3)
		require.Len(t, fetches[2], 3) // E1 U9 E2 in some random order
	})

}

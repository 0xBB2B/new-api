package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedUserNamesLookupUsers(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Create(&User{Id: 7, Username: "zhangsan", DisplayName: "张三", Password: "password", AffCode: "aff-7"}).Error)
	require.NoError(t, DB.Create(&User{Id: 8, Username: "lisi", DisplayName: "", Password: "password", AffCode: "aff-8"}).Error)
	require.NoError(t, DB.Create(&User{Id: 9, Username: "deleted", Password: "password", AffCode: "aff-9"}).Error)
	require.NoError(t, DB.Delete(&User{Id: 9}).Error)
}

func TestGetUserNamesByIdsReturnsExistingNonDeletedUsersDeduped(t *testing.T) {
	truncateTables(t)
	seedUserNamesLookupUsers(t)

	result, err := GetUserNamesByIds([]int{7, 8, 9, 7, 0})
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, UserNames{Username: "zhangsan", DisplayName: "张三"}, result[7])
	assert.Equal(t, UserNames{Username: "lisi", DisplayName: ""}, result[8])
	_, hasDeleted := result[9]
	assert.False(t, hasDeleted)
	_, hasZero := result[0]
	assert.False(t, hasZero)
}

func TestGetUserNamesByIdsWithoutValidIdsReturnsEmptyMap(t *testing.T) {
	truncateTables(t)
	seedUserNamesLookupUsers(t)

	cases := []struct {
		name string
		ids  []int
	}{
		{name: "all zero", ids: []int{0, 0}},
		{name: "nil", ids: nil},
		{name: "empty slice", ids: []int{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := GetUserNamesByIds(tc.ids)
			require.NoError(t, err)
			assert.Len(t, result, 0)
		})
	}
}

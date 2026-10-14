package model

import (
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	utilizationNow = int64(1_800_000_000)
	utilizationDay = int64(24 * 3600)
)

type utilizationRecord struct {
	age    int64
	tokens int
}

type utilizationUser struct {
	role    int
	status  int
	deleted bool
	records []utilizationRecord
}

func enabledUser(records ...utilizationRecord) utilizationUser {
	return utilizationUser{role: common.RoleCommonUser, status: common.UserStatusEnabled, records: records}
}

func excludedUsers() []utilizationUser {
	heavy := []utilizationRecord{{age: 3600, tokens: 500_000_000}}
	return []utilizationUser{
		{role: common.RoleCommonUser, status: common.UserStatusDisabled, records: heavy},
		{role: common.RoleCommonUser, status: common.UserStatusEnabled, deleted: true, records: heavy},
		{role: common.RoleAdminUser, status: common.UserStatusEnabled, records: heavy},
		{role: common.RoleRootUser, status: common.UserStatusEnabled, records: heavy},
	}
}

func baseUtilizationUsers() []utilizationUser {
	var users []utilizationUser
	for range 13 {
		users = append(users, enabledUser(utilizationRecord{age: 3600, tokens: 200_000_000}))
	}
	for range 8 {
		users = append(users, enabledUser(utilizationRecord{age: 2 * 3600, tokens: 5_000_000}))
	}
	for range 13 {
		users = append(users, enabledUser(utilizationRecord{age: 8 * utilizationDay, tokens: 2_000_000}))
	}
	for range 2 {
		users = append(users, enabledUser(utilizationRecord{age: 3 * 3600, tokens: 500_000}))
	}
	return users
}

func seedUtilizationUsers(t *testing.T, users []utilizationUser) {
	t.Helper()
	for i, spec := range users {
		user := User{
			Username: fmt.Sprintf("util_u%d", i),
			Password: "placeholder-password",
			Role:     spec.role,
			Status:   spec.status,
			AffCode:  fmt.Sprintf("util_aff%d", i),
		}
		require.NoError(t, DB.Create(&user).Error)
		for _, record := range spec.records {
			require.NoError(t, DB.Create(&QuotaData{
				UserID:    user.Id,
				Username:  user.Username,
				ModelName: "util-model",
				CreatedAt: utilizationNow - record.age,
				TokenUsed: record.tokens,
				Count:     1,
			}).Error)
		}
		if spec.deleted {
			require.NoError(t, DB.Delete(&user).Error)
		}
	}
}

func clearUtilizationTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Unscoped().Where("1 = 1").Delete(&QuotaData{}).Error)
	require.NoError(t, DB.Unscoped().Where("1 = 1").Delete(&User{}).Error)
}

func TestGetUserUtilization(t *testing.T) {
	base := baseUtilizationUsers()
	baseWant := UserUtilization{TotalUsers: 36, DeepUsers: 13, FrequentUsers: 21, ModerateUsers: 13, InactiveUsers: 2}
	with := func(extra ...utilizationUser) []utilizationUser { return slices.Concat(base, extra) }
	plus := func(want UserUtilization, total, deep, frequent, moderate, inactive int64) UserUtilization {
		return UserUtilization{
			TotalUsers:    want.TotalUsers + total,
			DeepUsers:     want.DeepUsers + deep,
			FrequentUsers: want.FrequentUsers + frequent,
			ModerateUsers: want.ModerateUsers + moderate,
			InactiveUsers: want.InactiveUsers + inactive,
		}
	}

	cases := []struct {
		name  string
		users []utilizationUser
		want  UserUtilization
	}{
		{"base_distribution", base, baseWant},
		{"exactly_100M_in_7d_is_frequent_not_deep", with(enabledUser(utilizationRecord{3600, 100_000_000})), plus(baseWant, 1, 0, 1, 0, 0)},
		{"one_over_100M_in_7d_is_deep", with(enabledUser(utilizationRecord{3600, 100_000_001})), plus(baseWant, 1, 1, 1, 0, 0)},
		{"exactly_1M_in_7d_and_30d_is_inactive", with(enabledUser(utilizationRecord{3600, 1_000_000})), plus(baseWant, 1, 0, 0, 0, 1)},
		{"one_over_1M_in_7d_is_frequent", with(enabledUser(utilizationRecord{3600, 1_000_001})), plus(baseWant, 1, 0, 1, 0, 0)},
		{"no_records_is_inactive", with(enabledUser()), plus(baseWant, 1, 0, 0, 0, 1)},
		{"zero_token_records_only_is_inactive", with(enabledUser(utilizationRecord{3600, 0})), plus(baseWant, 1, 0, 0, 0, 1)},
		{"1B_tokens_31d_ago_is_inactive", with(enabledUser(utilizationRecord{31 * utilizationDay, 1_000_000_000})), plus(baseWant, 1, 0, 0, 0, 1)},
		{"record_exactly_7d_old_counts_in_7d_window", with(enabledUser(utilizationRecord{7 * utilizationDay, 5_000_000})), plus(baseWant, 1, 0, 1, 0, 0)},
		{"record_just_over_7d_old_only_counts_in_30d_window", with(enabledUser(utilizationRecord{7*utilizationDay + 1, 5_000_000})), plus(baseWant, 1, 0, 0, 1, 0)},
		{"record_exactly_30d_old_counts_in_30d_window", with(enabledUser(utilizationRecord{30 * utilizationDay, 2_000_000})), plus(baseWant, 1, 0, 0, 1, 0)},
		{"record_just_over_30d_old_is_outside_window", with(enabledUser(utilizationRecord{30*utilizationDay + 1, 2_000_000})), plus(baseWant, 1, 0, 0, 0, 1)},
		{"records_in_7d_accumulate_to_frequent", with(enabledUser(utilizationRecord{3600, 600_000}, utilizationRecord{7200, 600_000})), plus(baseWant, 1, 0, 1, 0, 0)},
		{"records_across_windows_accumulate_to_moderate", with(enabledUser(utilizationRecord{3600, 600_000}, utilizationRecord{8 * utilizationDay, 600_000})), plus(baseWant, 1, 0, 0, 1, 0)},
		{"disabled_deleted_admin_root_are_excluded", with(excludedUsers()...), baseWant},
		{"no_enabled_common_users_gives_all_zero", excludedUsers(), UserUtilization{}},
		{"empty_tables_give_all_zero", nil, UserUtilization{}},
	}

	runCases := func(t *testing.T) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				clearUtilizationTables(t)
				t.Cleanup(func() { clearUtilizationTables(t) })
				seedUtilizationUsers(t, tc.users)

				got, err := GetUserUtilization(utilizationNow)
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tc.want, *got)
				assert.Equal(t, got.TotalUsers, got.DeepUsers+(got.FrequentUsers-got.DeepUsers)+got.ModerateUsers+got.InactiveUsers)
			})
		}
	}

	t.Run("sqlite", runCases)

	external := []struct {
		name         string
		dsnEnv       string
		databaseType common.DatabaseType
	}{
		{"mysql", "TEST_MYSQL_DSN", common.DatabaseTypeMySQL},
		{"postgres", "TEST_POSTGRES_DSN", common.DatabaseTypePostgreSQL},
	}
	for _, ext := range external {
		t.Run(ext.name, func(t *testing.T) {
			dsn := os.Getenv(ext.dsnEnv)
			if dsn == "" {
				t.Skip("test database DSN is not configured")
			}
			t.Setenv("USER_UTILIZATION_TEST_DSN", dsn)
			db, _, err := chooseDB("USER_UTILIZATION_TEST_DSN", false)
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			require.NoError(t, db.AutoMigrate(&User{}, &QuotaData{}))

			previousDB, previousLogDB := DB, LOG_DB
			previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
			DB, LOG_DB = db, db
			common.SetDatabaseTypes(ext.databaseType, ext.databaseType)
			initCol()
			t.Cleanup(func() {
				clearUtilizationTables(t)
				DB, LOG_DB = previousDB, previousLogDB
				common.SetDatabaseTypes(previousMain, previousLog)
				initCol()
			})

			runCases(t)
		})
	}
}

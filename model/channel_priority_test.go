package model

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestChannelPriorityPlan(t *testing.T) {
	for _, tc := range []struct {
		name       string
		values     []int64
		id, target int
		position   string
		want       map[int]int64
	}{
		{"midpoint", []int64{100, 80, 0}, 3, 2, "before", map[int]int64{3: 90}},
		{"top preserves peers", []int64{0, 0, 0}, 3, 1, "before", map[int]int64{3: 10}},
		{"bottom", []int64{0, 0, 0}, 1, 3, "after", map[int]int64{1: -10}},
		{"same tier", []int64{100, 80, 0}, 3, 2, "same", map[int]int64{3: 80}},
		{"same slot", []int64{100, 80, 0}, 2, 1, "after", map[int]int64{}},
		{"same tier no op", []int64{0, 0}, 1, 2, "same", map[int]int64{}},
		{"shift smaller upper tier", []int64{100, 99, 99, 0}, 4, 2, "before", map[int]int64{1: 101, 4: 100}},
		{"shift smaller lower tier", []int64{100, 100, 99, 0}, 4, 3, "before", map[int]int64{3: 98, 4: 99}},
		{"shift complete adjacent tiers", []int64{102, 101, 100, 99, 0}, 5, 3, "after", map[int]int64{4: 98, 5: 99}},
		{"upper safe bound", []int64{maxSortablePriority, 0}, 2, 1, "before", map[int]int64{1: maxSortablePriority - 1, 2: maxSortablePriority}},
		{"lower safe bound", []int64{-maxSortablePriority, 0}, 2, 1, "after", map[int]int64{1: -maxSortablePriority + 1, 2: -maxSortablePriority}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := make([]ChannelPriority, len(tc.values))
			for i, value := range tc.values {
				items[i] = ChannelPriority{ID: i + 1, Priority: &value}
			}
			changes, err := planChannelPriorityMove(items, ChannelPriorityMove{ID: tc.id, TargetID: tc.target, Position: tc.position})
			require.NoError(t, err)
			got := map[int]int64{}
			for _, change := range changes {
				require.NotNil(t, change.Priority)
				got[change.ID] = *change.Priority
			}
			assert.Equal(t, tc.want, got)
		})
	}
	_, err := planChannelPriorityMove([]ChannelPriority{{ID: 1}, {ID: 2}}, ChannelPriorityMove{ID: 1, TargetID: 2, Position: "invalid"})
	assert.ErrorIs(t, err, ErrChannelPriorityInvalid)
}

func TestChannelPriorityDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "priority_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			previousDB, previousType := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(&Ability{}, &Channel{}))
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				initCol()
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
			var version string
			query := "SELECT version()"
			if dialect == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("database version: %s", version)
			high, low, zero := int64(100), int64(99), int64(0)
			w1, w2, w3, w4 := uint(11), uint(22), uint(33), uint(44)
			channels := []Channel{
				{Name: "high", Priority: &high, Weight: &w1, Key: "preserved", Status: 1},
				{Name: "low", Priority: &low, Weight: &w2, Status: 1},
				{Name: "peer", Priority: &low, Weight: &w3, Status: 1},
				{Name: "moving", Priority: &zero, Weight: &w4, Status: 1},
			}
			require.NoError(t, db.Create(&channels).Error)
			for _, channel := range channels {
				for _, group := range []string{"default", "second"} {
					require.NoError(t, db.Create(&Ability{Group: group, Model: "test", ChannelId: channel.Id, Priority: channel.Priority, Weight: uint(*channel.Weight), Enabled: true}).Error)
				}
			}
			initial, err := GetChannelPriorityOrder()
			require.NoError(t, err)
			move := ChannelPriorityMove{Revision: initial.Revision, ID: channels[3].Id, TargetID: channels[1].Id, Position: "before", Preview: true}
			preview, err := ReorderChannelPriority(move)
			require.NoError(t, err)
			require.Len(t, preview.Changes, 2)
			unchanged, err := GetChannelPriorityOrder()
			require.NoError(t, err)
			assert.Equal(t, initial, unchanged, "preview does not persist")
			move.Preview = false
			applied, err := ReorderChannelPriority(move)
			require.NoError(t, err)
			assert.Equal(t, preview, applied)
			for _, change := range applied.Changes {
				var abilities []Ability
				require.NoError(t, db.Where("channel_id = ?", change.ID).Find(&abilities).Error)
				require.Len(t, abilities, 2)
				for _, ability := range abilities {
					assert.Equal(t, change.Priority, ability.Priority)
					assert.True(t, ability.Enabled)
				}
			}
			var saved Channel
			require.NoError(t, db.First(&saved, channels[0].Id).Error)
			assert.Equal(t, "preserved", saved.Key)
			assert.Equal(t, channels[0].Weight, saved.Weight)
			_, err = ReorderChannelPriority(move)
			assert.ErrorIs(t, err, ErrChannelPriorityConflict)
			undo := ChannelPriorityMove{Revision: applied.Revision}
			for _, change := range applied.Changes {
				undo.Restore = append(undo.Restore, ChannelPriorityChange{ID: change.ID, Previous: change.Priority, Priority: change.Previous})
			}
			restored, err := ReorderChannelPriority(undo)
			require.NoError(t, err)
			assert.Equal(t, initial.Items, restored.Items)
			assert.Equal(t, initial.Revision, restored.Revision)
			if dialect != "sqlite" {
				sqlDB.SetMaxOpenConns(2)
			}
			start := make(chan struct{})
			errorsCh := make(chan error, 2)
			var workers sync.WaitGroup
			for range 2 {
				workers.Go(func() { <-start; _, moveErr := ReorderChannelPriority(move); errorsCh <- moveErr })
			}
			close(start)
			workers.Wait()
			close(errorsCh)
			succeeded, conflicted := 0, 0
			for moveErr := range errorsCh {
				if moveErr == nil {
					succeeded++
				} else {
					require.ErrorIs(t, moveErr, ErrChannelPriorityConflict)
					conflicted++
				}
			}
			assert.Equal(t, 1, succeeded)
			assert.Equal(t, 1, conflicted)
			_, err = ReorderChannelPriority(undo)
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			// An ability write failure must roll back every channel change too.
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("priority_test_failure", func(tx *gorm.DB) {
				if tx.Statement.Table == "priority_test_abilities" {
					tx.AddError(errors.New("forced routing write failure"))
				}
			}))
			_, err = ReorderChannelPriority(move)
			require.ErrorContains(t, err, "forced routing write failure")
			require.NoError(t, db.Callback().Update().Remove("priority_test_failure"))
			unchanged, err = GetChannelPriorityOrder()
			require.NoError(t, err)
			assert.Equal(t, initial, unchanged)
			// Null priority is an effective zero and undo must restore the null itself.
			require.NoError(t, db.Model(&Channel{}).Where("id = ?", channels[3].Id).Update("priority", nil).Error)
			nullable, err := GetChannelPriorityOrder()
			require.NoError(t, err)
			var sorted []ChannelPriority
			require.NoError(t, NewChannelSortOptions("priority", "asc", false).Apply(db.Model(&Channel{}).Select("id", "priority")).Find(&sorted).Error)
			assert.Equal(t, []int{channels[3].Id, channels[2].Id, channels[1].Id, channels[0].Id}, []int{sorted[0].ID, sorted[1].ID, sorted[2].ID, sorted[3].ID})
			move.Revision = nullable.Revision
			applied, err = ReorderChannelPriority(move)
			require.NoError(t, err)
			undo = ChannelPriorityMove{Revision: applied.Revision}
			for _, change := range applied.Changes {
				undo.Restore = append(undo.Restore, ChannelPriorityChange{ID: change.ID, Previous: change.Priority, Priority: change.Previous})
			}
			restored, err = ReorderChannelPriority(undo)
			require.NoError(t, err)
			assert.Equal(t, nullable.Items, restored.Items)
		})
	}
}

type subscriptionFixture struct {
	db                *gorm.DB
	group, model      string
	a, b, c, d, s     int
	multiKeyChannelID int
}

func newSubscriptionFixture(t *testing.T, dialect string, cacheOn bool) *subscriptionFixture {
	t.Helper()
	var driver gorm.Dialector
	switch dialect {
	case "sqlite":
		driver = sqlite.Open(":memory:")
	case "mysql":
		dsn := os.Getenv("TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("TEST_MYSQL_DSN not configured")
		}
		driver = mysql.Open(dsn)
	case "postgres":
		dsn := os.Getenv("TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("TEST_POSTGRES_DSN not configured")
		}
		driver = postgres.Open(dsn)
	}
	db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "subscription_test_"}})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	previousDB, previousType, previousCache := DB, common.MainDatabaseType(), common.MemoryCacheEnabled
	DB = db
	common.SetMainDatabaseType(common.DatabaseType(dialect))
	initCol()
	t.Cleanup(func() {
		require.NoError(t, db.Migrator().DropTable(&Ability{}, &Channel{}))
		DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.MemoryCacheEnabled = previousCache
		initCol()
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

	f := &subscriptionFixture{db: db, group: "g", model: "m"}
	weight := uint(10)
	add := func(name string, channelType int, priority int64) int {
		channel := Channel{Name: name, Type: channelType, Priority: &priority, Weight: &weight, Status: common.ChannelStatusEnabled, Group: f.group, Models: f.model}
		require.NoError(t, db.Create(&channel).Error)
		require.NoError(t, db.Create(&Ability{Group: f.group, Model: f.model, ChannelId: channel.Id, Priority: &priority, Weight: weight, Enabled: true}).Error)
		return channel.Id
	}
	f.a = add("claude-sub", constant.ChannelTypeClaudeSubscription, 0)
	f.b = add("codex-sub", constant.ChannelTypeCodex, 0)
	f.c = add("openai-c", constant.ChannelTypeOpenAI, 10)
	f.d = add("openai-d", constant.ChannelTypeOpenAI, 5)
	f.s = add("sub2api", constant.ChannelTypeSub2API, 20)

	multiKey := Channel{
		Name: "multi", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Group: "multi", Models: "multi",
		Key: "k0\nk1\nk2", Weight: &weight,
		ChannelInfo: ChannelInfo{
			IsMultiKey: true, MultiKeySize: 3, MultiKeyMode: constant.MultiKeyModePolling, MultiKeyPollingIndex: 1,
			MultiKeyStatusList: map[int]int{1: common.ChannelStatusManuallyDisabled},
		},
	}
	require.NoError(t, db.Create(&multiKey).Error)
	require.NoError(t, db.Create(&Ability{Group: "multi", Model: "multi", ChannelId: multiKey.Id, Weight: weight, Enabled: true}).Error)
	f.multiKeyChannelID = multiKey.Id

	common.MemoryCacheEnabled = cacheOn
	InitChannelCache()
	return f
}

func TestSubscriptionChannelOrdering(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		for _, cacheOn := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cache=%v", dialect, cacheOn), func(t *testing.T) {
				f := newSubscriptionFixture(t, dialect, cacheOn)

				for retry, want := range map[int][]int{
					0: {f.a, f.b},
					1: {f.s},
					2: {f.c},
					3: {f.d},
					9: {f.d},
				} {
					got, err := GetRandomSatisfiedChannel(f.group, f.model, retry, nil)
					require.NoError(t, err)
					require.NotNil(t, got, "retry %d", retry)
					assert.Contains(t, want, got.Id, "retry %d", retry)
				}

				tried := map[int]bool{}
				isTried := func(id int) bool { return tried[id] }
				for _, step := range []struct {
					mark []int
					want []int
				}{
					{nil, []int{f.a, f.b}},
					{[]int{f.a, f.b}, []int{f.s}},
					{[]int{f.s, f.c}, []int{f.d}},
					{[]int{f.d}, nil},
				} {
					for _, id := range step.mark {
						tried[id] = true
					}
					got, err := GetUntriedSatisfiedChannel(f.group, f.model, nil, isTried)
					require.NoError(t, err)
					if step.want == nil {
						assert.Nil(t, got)
						continue
					}
					require.NotNil(t, got)
					assert.Contains(t, step.want, got.Id)
				}
			})
		}
	}
}

func TestGetUntriedSatisfiedChannelNormalizedModelAndFilters(t *testing.T) {
	openAIOnly := []dto.ChannelFilter{{
		Kind: dto.FilterTaskPluginIdentity, TaskPluginKey: "any", TaskPluginChannelTypes: []int{constant.ChannelTypeOpenAI},
	}}
	for _, cacheOn := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			model   string
			filters []dto.ChannelFilter
			want    func(f *subscriptionFixture) []int
		}{
			{"exact model without filters", "m", nil, func(f *subscriptionFixture) []int { return []int{f.a, f.b} }},
			{"normalized fallback without filters", "m@effort:high", nil, func(f *subscriptionFixture) []int { return []int{f.a, f.b} }},
			{"exact model with excluding filter", "m", openAIOnly, func(f *subscriptionFixture) []int { return []int{f.c} }},
			{"normalized fallback with excluding filter", "m@effort:high", openAIOnly, func(f *subscriptionFixture) []int { return []int{f.c} }},
		} {
			t.Run(fmt.Sprintf("cache=%v/%s", cacheOn, tc.name), func(t *testing.T) {
				f := newSubscriptionFixture(t, "sqlite", cacheOn)
				want := tc.want(f)
				for range 30 {
					got, err := GetUntriedSatisfiedChannel(f.group, tc.model, tc.filters, func(int) bool { return false })
					require.NoError(t, err)
					require.NotNil(t, got)
					assert.Contains(t, want, got.Id)
				}
			})
		}
	}
}

func TestPriorityWithoutSubscriptionChannelsKeepsOrder(t *testing.T) {
	for _, cacheOn := range []bool{false, true} {
		t.Run(fmt.Sprintf("cache=%v", cacheOn), func(t *testing.T) {
			f := newSubscriptionFixture(t, "sqlite", cacheOn)
			require.NoError(t, f.db.Where("id IN ?", []int{f.a, f.b}).Delete(&Channel{}).Error)
			require.NoError(t, f.db.Where("channel_id IN ?", []int{f.a, f.b}).Delete(&Ability{}).Error)
			InitChannelCache()

			for retry, want := range []int{f.s, f.c, f.d} {
				got, err := GetRandomSatisfiedChannel(f.group, f.model, retry, nil)
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, want, got.Id, "retry %d", retry)
			}
		})
	}
}

func TestIsSubscriptionChannelType(t *testing.T) {
	assert.True(t, IsSubscriptionChannelType(constant.ChannelTypeCodex))
	assert.True(t, IsSubscriptionChannelType(constant.ChannelTypeClaudeSubscription))
	assert.False(t, IsSubscriptionChannelType(constant.ChannelTypeSub2API))
	assert.False(t, IsSubscriptionChannelType(constant.ChannelTypeOpenAI))
}

func TestEnabledKeyIndexes(t *testing.T) {
	multi := &Channel{
		Key: "k0\nk1\nk2",
		ChannelInfo: ChannelInfo{
			IsMultiKey: true, MultiKeySize: 3,
			MultiKeyStatusList: map[int]int{1: common.ChannelStatusManuallyDisabled},
		},
	}
	assert.Equal(t, []int{0, 2}, multi.EnabledKeyIndexes())
	assert.Equal(t, []int{0}, (&Channel{Key: "single"}).EnabledKeyIndexes())
}

func TestGetNextUntriedEnabledKey(t *testing.T) {
	multiKeyChannel := func(mode constant.MultiKeyMode) *Channel {
		return &Channel{
			Id:  987001,
			Key: "k0\nk1\nk2",
			ChannelInfo: ChannelInfo{
				IsMultiKey: true, MultiKeySize: 3, MultiKeyMode: mode,
				MultiKeyStatusList: map[int]int{1: common.ChannelStatusManuallyDisabled},
			},
		}
	}
	requireExhausted := func(t *testing.T, apiErr *types.NewAPIError) {
		t.Helper()
		require.NotNil(t, apiErr)
		assert.NotEqual(t, types.ErrorCodeChannelNoAvailableKey, apiErr.GetErrorCode())
		assert.False(t, strings.HasPrefix(string(apiErr.GetErrorCode()), "channel:"))
	}

	t.Run("random skips tried and disabled", func(t *testing.T) {
		channel := multiKeyChannel(constant.MultiKeyModeRandom)
		key, index, apiErr := channel.GetNextUntriedEnabledKey(map[int]bool{0: true})
		require.Nil(t, apiErr)
		assert.Equal(t, 2, index)
		assert.Equal(t, "k2", key)

		_, _, apiErr = channel.GetNextUntriedEnabledKey(map[int]bool{0: true, 2: true})
		requireExhausted(t, apiErr)
	})

	t.Run("single key", func(t *testing.T) {
		channel := &Channel{Key: "only"}
		key, index, apiErr := channel.GetNextUntriedEnabledKey(nil)
		require.Nil(t, apiErr)
		assert.Equal(t, "only", key)
		assert.Equal(t, 0, index)

		_, _, apiErr = channel.GetNextUntriedEnabledKey(map[int]bool{0: true})
		requireExhausted(t, apiErr)
	})

	t.Run("polling advances position", func(t *testing.T) {
		for _, cacheOn := range []bool{false, true} {
			t.Run(fmt.Sprintf("cache=%v", cacheOn), func(t *testing.T) {
				f := newSubscriptionFixture(t, "sqlite", cacheOn)
				var channel Channel
				require.NoError(t, f.db.First(&channel, f.multiKeyChannelID).Error)
				require.Equal(t, 1, channel.ChannelInfo.MultiKeyPollingIndex)

				key, index, apiErr := channel.GetNextUntriedEnabledKey(map[int]bool{0: true})
				require.Nil(t, apiErr)
				assert.Equal(t, 2, index)
				assert.Equal(t, "k2", key)
				assert.Equal(t, 0, channel.ChannelInfo.MultiKeyPollingIndex)

				_, _, apiErr = channel.GetNextUntriedEnabledKey(map[int]bool{0: true, 2: true})
				requireExhausted(t, apiErr)
			})
		}
	})
}

func TestGetNextUntriedEnabledKeyPollingPersistsWithoutMemoryCache(t *testing.T) {
	f := newSubscriptionFixture(t, "sqlite", false)
	var channel Channel
	require.NoError(t, f.db.First(&channel, f.multiKeyChannelID).Error)
	require.Equal(t, 1, channel.ChannelInfo.MultiKeyPollingIndex)

	_, index, apiErr := channel.GetNextUntriedEnabledKey(map[int]bool{0: true})
	require.Nil(t, apiErr)
	require.Equal(t, 2, index)

	var reloaded Channel
	require.NoError(t, f.db.First(&reloaded, f.multiKeyChannelID).Error)
	assert.Equal(t, 0, reloaded.ChannelInfo.MultiKeyPollingIndex)
}

func TestGetNextEnabledKeyUnchanged(t *testing.T) {
	channel := &Channel{
		Key: "k0\nk1",
		ChannelInfo: ChannelInfo{
			IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModeRandom,
			MultiKeyStatusList: map[int]int{0: common.ChannelStatusManuallyDisabled, 1: common.ChannelStatusManuallyDisabled},
		},
	}
	_, _, apiErr := channel.GetNextEnabledKey()
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeChannelNoAvailableKey, apiErr.GetErrorCode())
}

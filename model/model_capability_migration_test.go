package model

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	legacyThinkingOptionKey = "smart_routing_setting.thinking_capabilities"
	capabilityMigrationDone = "model_capability_migration_done"
	legacyOverrideJSON      = `{"glm-5.3":{"no_thinking":false,"efforts":["low","high"]},"my-model":{"no_thinking":true,"efforts":[]}}`
)

var (
	effortsFull    = []string{"none", "low", "medium", "high", "xhigh"}
	effortsMax     = []string{"none", "low", "medium", "high", "xhigh", "max"}
	effortsPro     = []string{"medium", "high", "xhigh"}
	effortsCodex   = []string{"low", "medium", "high", "xhigh"}
	effortsGLM     = []string{"low", "high", "max"}
	effortsNoneHM  = []string{"none", "high", "max"}
	effortsDeepSk  = []string{"none", "low", "high", "max"}
	effortsQwen38  = []string{"none", "low", "medium", "xhigh"}
	effortsQwen4   = []string{"none", "low", "high", "xhigh"}
	noThinking     = []string{}
	snapshotModels = map[string][]string{
		"gpt-4.1": noThinking, "gpt-4o": noThinking, "gpt-4.1-mini": noThinking,
		"gpt-5.4": effortsFull, "gpt-5.4-mini": effortsFull, "gpt-5.4-nano": effortsFull, "gpt-5.5": effortsFull,
		"gpt-5.4-pro": effortsPro, "gpt-5.5-pro": effortsPro,
		"gpt-5.2-codex": effortsCodex, "gpt-5.3-codex": effortsCodex,
		"gpt-5.6": effortsMax, "gpt-5.6-sol": effortsMax, "gpt-5.6-terra": effortsMax, "gpt-5.6-luna": effortsMax,
		"gpt-6-sol": effortsMax, "gpt-6-luna": effortsMax, "gpt-6.1-terra": effortsMax, "gpt-6-astra": effortsMax,
		"gpt-6.1-sol": effortsMax, "gpt-6.2-sol": effortsMax, "gpt-6.2-astra": effortsMax,
		"glm-5.3": effortsGLM, "glm-5.3-flash": effortsGLM, "glm-5.3-flashx": effortsGLM,
		"glm-5.2":         effortsNoneHM,
		"deepseek-v4-pro": effortsDeepSk, "deepseek-v4-flash": effortsDeepSk, "deepseek-v4.1-flash": effortsDeepSk, "kimi-k3.1-pro": effortsDeepSk,
		"qwen3.8-max": effortsQwen38, "qwen4-max": effortsQwen4,
	}
)

func expectedAfterMigration() map[string][]string {
	expected := map[string][]string{}
	for name, efforts := range snapshotModels {
		expected[name] = efforts
	}
	expected["glm-5.3"] = []string{"low", "high"}
	expected["my-model"] = noThinking
	return expected
}

var capabilityTestDialect = "sqlite"

func forEachCapabilityDialect(t *testing.T, body func(t *testing.T)) {
	t.Helper()
	for _, dialect := range []struct{ kind, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(dialect.kind, func(t *testing.T) {
			if dialect.env != "" && os.Getenv(dialect.env) == "" {
				t.Skip("set " + dialect.env + " to run this database")
			}
			capabilityTestDialect = dialect.kind
			t.Cleanup(func() { capabilityTestDialect = "sqlite" })
			body(t)
		})
	}
}

func openCapabilityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	var driver gorm.Dialector
	databaseType := common.DatabaseTypeSQLite
	switch capabilityTestDialect {
	case "mysql":
		driver = mysql.Open(os.Getenv("TEST_MYSQL_DSN"))
		databaseType = common.DatabaseTypeMySQL
	case "postgres":
		driver = postgres.New(postgres.Config{DSN: os.Getenv("TEST_POSTGRES_DSN"), PreferSimpleProtocol: true})
		databaseType = common.DatabaseTypePostgreSQL
	default:
		driver = sqlite.Open(filepath.Join(t.TempDir(), "capability.db"))
	}
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(databaseType, databaseType)
	initCol()
	t.Cleanup(func() {
		common.SetDatabaseTypes(previousMain, previousLog)
		initCol()
	})
	db, err := gorm.Open(driver, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&Model{}, &Option{}, &Vendor{}, &Channel{}, &Ability{}, &SystemTask{}))
	for _, table := range []any{&Model{}, &Option{}, &Vendor{}, &Channel{}, &Ability{}, &SystemTask{}} {
		require.NoError(t, db.Unscoped().Where("1 = 1").Delete(table).Error)
	}
	return db
}

func setupCapabilityMigrationDB(t *testing.T, legacyOption string) *gorm.DB {
	t.Helper()
	db := openCapabilityTestDB(t)
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })
	if legacyOption != "" {
		require.NoError(t, db.Create(&Option{Key: legacyThinkingOptionKey, Value: legacyOption}).Error)
	}
	return db
}

func seedCapabilityMigrationRecords(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&Vendor{Id: 7, Name: "openai", Status: 1}).Error)
	require.NoError(t, db.Create(&Model{ModelName: "gpt-5.5", Description: "x", Capabilities: `{"context_length":1000}`, NameRule: NameRuleExact}).Error)
	prefix := &Model{
		ModelName: "gpt-6", Description: "GPT-6 家族", Icon: "OpenAI", Tags: "chat,reasoning", VendorID: 7,
		Endpoints: `{"openai":{"path":"/v1/chat/completions","method":"POST"}}`, Status: 1, NameRule: NameRulePrefix,
	}
	require.NoError(t, db.Create(prefix).Error)
	require.NoError(t, db.Model(&Model{}).Where("id = ?", prefix.Id).Update("sync_official", 0).Error)
}

func recordsByName(t *testing.T, db *gorm.DB) map[string]Model {
	t.Helper()
	var records []Model
	require.NoError(t, db.Find(&records).Error)
	byName := make(map[string]Model, len(records))
	for _, record := range records {
		_, duplicated := byName[record.ModelName]
		require.False(t, duplicated, "duplicate record %s", record.ModelName)
		byName[record.ModelName] = record
	}
	return byName
}

func effortsOfRecord(t *testing.T, record Model) []string {
	t.Helper()
	capability, err := ParseModelCapability(record.Capabilities)
	require.NoError(t, err)
	efforts, configured := ModelThinkingSupport(capability)
	require.True(t, configured, "%s has no reasoning.supported_efforts", record.ModelName)
	if len(efforts) == 0 {
		return []string{}
	}
	return efforts
}

func optionExists(t *testing.T, db *gorm.DB, key string) bool {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" = ?", key).Count(&count).Error)
	return count > 0
}

func assertMigratedEfforts(t *testing.T, db *gorm.DB) {
	t.Helper()
	byName := recordsByName(t, db)
	for name, want := range expectedAfterMigration() {
		record, ok := byName[name]
		if !assert.True(t, ok, "missing record %s", name) {
			continue
		}
		assert.Equal(t, NameRuleExact, record.NameRule, name)
		assert.Equal(t, want, effortsOfRecord(t, record), name)
	}
}

func TestMigrateLegacyThinkingCapabilities_WritesExactRecordsFromOverrideAndSnapshot(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		db := setupCapabilityMigrationDB(t, legacyOverrideJSON)
		seedCapabilityMigrationRecords(t, db)

		require.NoError(t, MigrateLegacyThinkingCapabilities())

		assertMigratedEfforts(t, db)
		byName := recordsByName(t, db)
		assert.Len(t, byName, len(expectedAfterMigration())+1, "snapshot + my-model + prefix record")
		assert.Equal(t, []string{"low", "high"}, effortsOfRecord(t, byName["glm-5.3"]))
		assert.Equal(t, []string{}, effortsOfRecord(t, byName["my-model"]))
		assert.Equal(t, []string{}, effortsOfRecord(t, byName["gpt-4o"]))
	})
}

func TestMigrateLegacyThinkingCapabilities_ExistingExactRecordOnlyGetsEfforts(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		db := setupCapabilityMigrationDB(t, legacyOverrideJSON)
		seedCapabilityMigrationRecords(t, db)

		require.NoError(t, MigrateLegacyThinkingCapabilities())

		record := recordsByName(t, db)["gpt-5.5"]
		assert.Equal(t, "x", record.Description)
		capability, err := ParseModelCapability(record.Capabilities)
		require.NoError(t, err)
		require.NotNil(t, capability.ContextLength)
		assert.EqualValues(t, 1000, *capability.ContextLength)
		assert.Equal(t, effortsFull, effortsOfRecord(t, record))
	})
}

func TestMigrateLegacyThinkingCapabilities_NewRecordCopiesMatchingRuleMetadata(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		db := setupCapabilityMigrationDB(t, legacyOverrideJSON)
		seedCapabilityMigrationRecords(t, db)

		require.NoError(t, MigrateLegacyThinkingCapabilities())

		byName := recordsByName(t, db)
		prefix := byName["gpt-6"]
		created := byName["gpt-6.2-sol"]
		assert.Equal(t, prefix.Description, created.Description)
		assert.Equal(t, prefix.Icon, created.Icon)
		assert.Equal(t, prefix.Tags, created.Tags)
		assert.Equal(t, prefix.VendorID, created.VendorID)
		assert.Equal(t, prefix.Endpoints, created.Endpoints)
		assert.Equal(t, prefix.Status, created.Status)
		assert.Equal(t, 0, created.SyncOfficial)
		assert.Equal(t, "GPT-6 家族", created.Description)
		assert.Equal(t, 7, created.VendorID)

		plain := byName["glm-5.2"]
		assert.Empty(t, plain.Description)
		assert.Empty(t, plain.Icon)
		assert.Empty(t, plain.Tags)
		assert.Zero(t, plain.VendorID)
		assert.Empty(t, plain.Endpoints)
		assert.Equal(t, 1, plain.Status)
		assert.Equal(t, 1, plain.SyncOfficial)
		assert.Equal(t, NameRuleExact, plain.NameRule)
	})
}

func TestMigrateLegacyThinkingCapabilities_CopySourceOrderPrefixSuffixContainsThenSmallestId(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		db := setupCapabilityMigrationDB(t, "")
		rules := []Model{
			{ModelName: "-flash", Description: "suffix", NameRule: NameRuleSuffix},
			{ModelName: "v4", Description: "contains", NameRule: NameRuleContains},
			{ModelName: "glm-", Description: "prefix-glm", NameRule: NameRulePrefix},
			{ModelName: "5.2", Description: "suffix-5.2", NameRule: NameRuleSuffix},
			{ModelName: "kimi-k3", Description: "prefix-first", NameRule: NameRulePrefix},
			{ModelName: "kimi", Description: "prefix-second", NameRule: NameRulePrefix},
		}
		for i := range rules {
			require.NoError(t, db.Create(&rules[i]).Error)
		}

		require.NoError(t, MigrateLegacyThinkingCapabilities())

		byName := recordsByName(t, db)
		assert.Equal(t, "prefix-glm", byName["glm-5.2"].Description, "prefix beats suffix")
		assert.Equal(t, "suffix", byName["deepseek-v4-flash"].Description, "suffix beats contains")
		assert.Equal(t, "contains", byName["deepseek-v4-pro"].Description)
		assert.Equal(t, "prefix-first", byName["kimi-k3.1-pro"].Description, "smallest id wins within a rule")
	})
}

func TestMigrateLegacyThinkingCapabilities_RunsOnlyOnce(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		db := setupCapabilityMigrationDB(t, legacyOverrideJSON)
		seedCapabilityMigrationRecords(t, db)
		require.NoError(t, MigrateLegacyThinkingCapabilities())
		countAfterFirst := len(recordsByName(t, db))
		assert.True(t, optionExists(t, db, capabilityMigrationDone))
		assert.False(t, optionExists(t, db, legacyThinkingOptionKey))

		require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "gpt-6.2-sol").
			Update("capabilities", `{"reasoning":{"supported_efforts":["low","high"]}}`).Error)
		require.NoError(t, db.Create(&Model{ModelName: "gpt-4o-extra", NameRule: NameRuleExact}).Error)
		require.NoError(t, db.Where("model_name = ?", "gpt-4o").Delete(&Model{}).Error)

		require.NoError(t, MigrateLegacyThinkingCapabilities())

		byName := recordsByName(t, db)
		assert.Len(t, byName, countAfterFirst, "no records created again (one added, one soft-deleted)")
		assert.Equal(t, []string{"low", "high"}, effortsOfRecord(t, byName["gpt-6.2-sol"]))
		_, restored := byName["gpt-4o"]
		assert.False(t, restored)
	})
}

func TestMigrateLegacyThinkingCapabilities_InvalidOverrideJSONFallsBackToSnapshot(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		db := setupCapabilityMigrationDB(t, `{bad`)

		require.NoError(t, MigrateLegacyThinkingCapabilities())

		byName := recordsByName(t, db)
		assert.Equal(t, effortsGLM, effortsOfRecord(t, byName["glm-5.3"]))
		assert.Equal(t, snapshotModels["gpt-4o"], effortsOfRecord(t, byName["gpt-4o"]))
		assert.Len(t, byName, len(snapshotModels))
		assert.False(t, optionExists(t, db, legacyThinkingOptionKey))
		assert.True(t, optionExists(t, db, capabilityMigrationDone))
	})
}

func TestMigrateLegacyThinkingCapabilities_FailureRollsBackAndRetryMatchesFullRun(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		db := setupCapabilityMigrationDB(t, legacyOverrideJSON)
		seedCapabilityMigrationRecords(t, db)
		before := recordsByName(t, db)

		creates := 0
		injectFailure := true
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_model_create", func(tx *gorm.DB) {
			if !injectFailure || tx.Statement.Table != "models" {
				return
			}
			creates++
			if creates == 5 {
				_ = tx.AddError(errors.New("injected create failure"))
			}
		}))

		require.Error(t, MigrateLegacyThinkingCapabilities())

		assert.Equal(t, before, recordsByName(t, db), "records restored to the pre-migration state")
		assert.True(t, optionExists(t, db, legacyThinkingOptionKey))
		assert.False(t, optionExists(t, db, capabilityMigrationDone))

		injectFailure = false
		require.NoError(t, MigrateLegacyThinkingCapabilities())

		assertMigratedEfforts(t, db)
		assert.Len(t, recordsByName(t, db), len(expectedAfterMigration())+1)
		assert.False(t, optionExists(t, db, legacyThinkingOptionKey))
		assert.True(t, optionExists(t, db, capabilityMigrationDone))
	})
}

func TestMigrateLegacyThinkingCapabilities_MigratedEffortsVisibleThroughCatalogLookup(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		db := setupCapabilityMigrationDB(t, legacyOverrideJSON)
		seedCapabilityMigrationRecords(t, db)
		require.NoError(t, MigrateLegacyThinkingCapabilities())

		RefreshPricing()

		for name, want := range expectedAfterMigration() {
			efforts, configured := ModelThinkingSupport(GetModelCapabilityByName(name))
			assert.True(t, configured, name)
			if len(efforts) == 0 {
				efforts = []string{}
			}
			assert.Equal(t, want, efforts, name)
		}
	})
}

func TestMigrateLegacyThinkingCapabilities_OverrideWithUnknownEffortIsSkippedWhole(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		cases := []struct{ name, override string }{
			{"unknown_value", `{"gpt-5.5":{"no_thinking":false,"efforts":["ultra","low"]}}`},
			{"different_case", `{"gpt-5.5":{"no_thinking":false,"efforts":["Low"]}}`},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				db := setupCapabilityMigrationDB(t, tc.override)
				require.NoError(t, db.Create(&Model{ModelName: "gpt-5.5", Capabilities: `{"context_length":1000}`, NameRule: NameRuleExact}).Error)

				require.NoError(t, MigrateLegacyThinkingCapabilities())

				record := recordsByName(t, db)["gpt-5.5"]
				assert.Equal(t, effortsFull, effortsOfRecord(t, record))
				capability, err := ParseModelCapability(record.Capabilities)
				require.NoError(t, err)
				require.NotNil(t, capability.ContextLength)
				assert.EqualValues(t, 1000, *capability.ContextLength)

				RefreshPricing()
				efforts, configured := ModelThinkingSupport(GetModelCapabilityByName("gpt-5.5"))
				assert.True(t, configured)
				assert.Equal(t, effortsFull, efforts)
			})
		}
	})
}

func TestMigrateLegacyThinkingCapabilities_OverrideWithInvalidModelNameIsSkipped(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		longName := strings.Repeat("a", 129)
		override := `{"":{"no_thinking":true,"efforts":[]},"` + longName + `":{"no_thinking":true,"efforts":[]},"my-model":{"no_thinking":true,"efforts":[]}}`
		db := setupCapabilityMigrationDB(t, override)

		require.NoError(t, MigrateLegacyThinkingCapabilities())

		byName := recordsByName(t, db)
		_, emptyCreated := byName[""]
		_, longCreated := byName[longName]
		assert.False(t, emptyCreated)
		assert.False(t, longCreated)
		assert.Len(t, byName, len(snapshotModels)+1, "snapshot models plus my-model")
		assert.Equal(t, []string{}, effortsOfRecord(t, byName["my-model"]))
		assert.True(t, optionExists(t, db, capabilityMigrationDone))
	})
}

func TestMigrateLegacyThinkingCapabilities_ThinkingResolvedByExactOriginalName(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		setupCapabilityMigrationDB(t, "")
		require.NoError(t, MigrateLegacyThinkingCapabilities())

		RefreshPricing()

		efforts, configured := ModelThinkingSupport(GetModelCapabilityByName("gpt-5.5"))
		assert.True(t, configured)
		assert.Equal(t, effortsFull, efforts)
		_, configured = ModelThinkingSupport(GetModelCapabilityByName("gpt-5.5-2026-03-05"))
		assert.False(t, configured)
	})
}

func TestMigrateLegacyThinkingCapabilities_ExistingPrefixRuleOfSameNameBlocksNewExactRecord(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		db := setupCapabilityMigrationDB(t, "")
		prefix := &Model{ModelName: "gpt-5.4", Description: "prefix", NameRule: NameRulePrefix, Status: 1, Capabilities: `{"context_length":1000}`}
		require.NoError(t, db.Create(prefix).Error)
		var before Model
		require.NoError(t, db.First(&before, prefix.Id).Error)

		require.NoError(t, MigrateLegacyThinkingCapabilities())

		var count int64
		require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "gpt-5.4").Count(&count).Error)
		assert.EqualValues(t, 1, count)
		var after Model
		require.NoError(t, db.First(&after, prefix.Id).Error)
		assert.Equal(t, before, after)
	})
}

func TestMigrateLegacyThinkingCapabilities_ExistingExactRecordGetsNewUpdatedTime(t *testing.T) {
	forEachCapabilityDialect(t, func(t *testing.T) {
		db := setupCapabilityMigrationDB(t, "")
		require.NoError(t, db.Create(&Model{ModelName: "gpt-5.5", NameRule: NameRuleExact, Status: 1, UpdatedTime: 100}).Error)

		require.NoError(t, MigrateLegacyThinkingCapabilities())

		var record Model
		require.NoError(t, db.Where("model_name = ?", "gpt-5.5").First(&record).Error)
		assert.Greater(t, record.UpdatedTime, int64(100))
	})
}

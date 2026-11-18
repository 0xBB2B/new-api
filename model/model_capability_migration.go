package model

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const modelCapabilityMigrationDoneKey = "model_capability_migration_done"
const legacyThinkingCapabilitiesOptionKey = "smart_routing_setting.thinking_capabilities"

// 升级前运行时内置表的最终内容；只供一次性搬迁读取，运行时取值不查它。
var legacyBuiltinThinkingEfforts = map[string][]string{
	"gpt-4.1":             {},
	"gpt-4o":              {},
	"gpt-4.1-mini":        {},
	"gpt-5.4":             {"none", "low", "medium", "high", "xhigh"},
	"gpt-5.4-mini":        {"none", "low", "medium", "high", "xhigh"},
	"gpt-5.4-nano":        {"none", "low", "medium", "high", "xhigh"},
	"gpt-5.5":             {"none", "low", "medium", "high", "xhigh"},
	"gpt-5.4-pro":         {"medium", "high", "xhigh"},
	"gpt-5.5-pro":         {"medium", "high", "xhigh"},
	"gpt-5.2-codex":       {"low", "medium", "high", "xhigh"},
	"gpt-5.3-codex":       {"low", "medium", "high", "xhigh"},
	"gpt-5.6":             {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-5.6-sol":         {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-5.6-terra":       {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-5.6-luna":        {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6-sol":           {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6-luna":          {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6.1-terra":       {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6-astra":         {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6.1-sol":         {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6.2-sol":         {"none", "low", "medium", "high", "xhigh", "max"},
	"gpt-6.2-astra":       {"none", "low", "medium", "high", "xhigh", "max"},
	"glm-5.3":             {"low", "high", "max"},
	"glm-5.3-flash":       {"low", "high", "max"},
	"glm-5.3-flashx":      {"low", "high", "max"},
	"glm-5.2":             {"none", "high", "max"},
	"deepseek-v4-pro":     {"none", "low", "high", "max"},
	"deepseek-v4-flash":   {"none", "low", "high", "max"},
	"deepseek-v4.1-flash": {"none", "low", "high", "max"},
	"kimi-k3.1-pro":       {"none", "low", "high", "max"},
	"qwen3.8-max":         {"none", "low", "medium", "xhigh"},
	"qwen4-max":           {"none", "low", "high", "xhigh"},
}

// MigrateLegacyThinkingCapabilities 把旧的思考档位覆盖选项和内置表一次性写成完全匹配的模型记录。
func MigrateLegacyThinkingCapabilities() error {
	var done int64
	if err := DB.Model(&Option{}).Where(commonKeyCol+" = ?", modelCapabilityMigrationDoneKey).Count(&done).Error; err != nil {
		return err
	}
	if done > 0 {
		return nil
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var override Option
		result := tx.Where(commonKeyCol+" = ?", legacyThinkingCapabilitiesOptionKey).Limit(1).Find(&override)
		if result.Error != nil {
			return result.Error
		}
		plan := legacyThinkingCapabilityPlan(override.Value)
		for _, name := range slices.Sorted(maps.Keys(plan)) {
			if err := upsertExactRecordThinking(tx, name, plan[name]); err != nil {
				return err
			}
		}
		if err := tx.Where(commonKeyCol+" = ?", legacyThinkingCapabilitiesOptionKey).Delete(&Option{}).Error; err != nil {
			return err
		}
		marker := Option{Key: modelCapabilityMigrationDoneKey, Value: strconv.FormatInt(common.GetTimestamp(), 10)}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&marker).Error
	})
	if err != nil {
		return err
	}
	BumpSyncTopic(SyncTopicModels)
	return nil
}

func legacyThinkingCapabilityPlan(overrideJSON string) map[string][]string {
	plan := make(map[string][]string, len(legacyBuiltinThinkingEfforts))
	for name, efforts := range legacyBuiltinThinkingEfforts {
		plan[name] = efforts
	}
	if overrideJSON == "" {
		return plan
	}
	var overrides map[string]struct {
		NoThinking bool     `json:"no_thinking"`
		Efforts    []string `json:"efforts"`
	}
	if err := common.UnmarshalJsonStr(overrideJSON, &overrides); err != nil {
		common.SysError(common.LogText("failed to parse legacy thinking capability overrides: %s", err.Error()))
		return plan
	}
	for name, override := range overrides {
		if name == "" || utf8.RuneCountInString(name) > 128 {
			common.SysError(common.LogText("skip legacy thinking capability override with invalid model name length: %d characters", utf8.RuneCountInString(name)))
			continue
		}
		if slices.ContainsFunc(override.Efforts, func(effort string) bool { return !slices.Contains(capabilityEfforts, effort) }) {
			common.SysError(common.LogText("skip legacy thinking capability override of model %s: unknown effort value", name))
			continue
		}
		if override.NoThinking || override.Efforts == nil {
			plan[name] = []string{}
			continue
		}
		plan[name] = override.Efforts
	}
	return plan
}

func upsertExactRecordThinking(tx *gorm.DB, name string, efforts []string) error {
	var record Model
	err := tx.Where("model_name = ? AND name_rule = ?", name, NameRuleExact).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var ruleCount int64
		if err := tx.Model(&Model{}).Where("model_name = ? AND name_rule <> ?", name, NameRuleExact).Count(&ruleCount).Error; err != nil {
			return err
		}
		if ruleCount > 0 {
			common.SysError(common.LogText("skip legacy thinking capability of model %s: name is used by a non-exact record", name))
			return nil
		}
		_, err = NewExactModelRecord(tx, name, ModelCapability{Efforts: &efforts}.MarshalCanonical())
		return err
	}
	if err != nil {
		return err
	}
	capability := parseRecordCapability(record)
	capability.Efforts = &efforts
	return tx.Model(&Model{}).Where("id = ?", record.Id).Updates(map[string]any{"capabilities": capability.MarshalCanonical(), "updated_time": common.GetTimestamp()}).Error
}

// NewExactModelRecord 在 tx 内新建完全匹配记录；名字已被前缀/后缀/包含记录命中时沿用它的目录信息。
func NewExactModelRecord(tx *gorm.DB, name, capabilities string) (*Model, error) {
	record := &Model{ModelName: name, Capabilities: capabilities, Status: 1, SyncOfficial: 1, NameRule: NameRuleExact}
	var rules []Model
	if err := tx.Where("name_rule <> ?", NameRuleExact).Order("id").Find(&rules).Error; err != nil {
		return nil, err
	}
	if source := resolveModelMetadata(rules, []string{name})[name]; source != nil {
		record.Description, record.Icon, record.Tags = source.Description, source.Icon, source.Tags
		record.VendorID, record.Endpoints = source.VendorID, source.Endpoints
		record.Status, record.SyncOfficial = source.Status, source.SyncOfficial
	}
	now := common.GetTimestamp()
	record.CreatedTime, record.UpdatedTime = now, now
	status, syncOfficial := record.Status, record.SyncOfficial
	if err := tx.Create(record).Error; err != nil {
		return nil, err
	}
	record.Status, record.SyncOfficial = status, syncOfficial
	err := tx.Model(&Model{}).Where("id = ?", record.Id).Updates(map[string]any{"status": status, "sync_official": syncOfficial}).Error
	return record, err
}

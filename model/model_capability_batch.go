package model

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	CapabilityActionCreate    = "create"
	CapabilityActionUpdate    = "update"
	CapabilityActionClear     = "clear"
	CapabilityActionUnchanged = "unchanged"
)

type ModelCapabilityListing struct {
	Capabilities map[string]json.RawMessage `json:"capabilities"`
	NonExact     []ModelCapabilityRuleRef   `json:"non_exact"`
	Unconfigured []string                   `json:"unconfigured"`
}

type ModelCapabilityRuleRef struct {
	ModelName string `json:"model_name"`
	NameRule  int    `json:"name_rule"`
}

type ModelCapabilityChange struct {
	ModelName string          `json:"model_name"`
	Action    string          `json:"action"`
	Before    json.RawMessage `json:"before"`
	After     json.RawMessage `json:"after"`
}

type ModelCapabilityError struct {
	ModelName string          `json:"model_name"`
	Message   *common.Message `json:"message"`
}

type ModelCapabilityPlan struct {
	Version string                  `json:"version"`
	Items   []ModelCapabilityChange `json:"items"`
	Errors  []ModelCapabilityError  `json:"errors"`
	Summary map[string]int          `json:"summary"`

	exactIDs map[string]int
}

func capabilityJSON(text string) json.RawMessage {
	if text == "" {
		return json.RawMessage("{}")
	}
	return json.RawMessage(text)
}

func loadCapabilityRecords(tx *gorm.DB, lock bool) ([]Model, error) {
	if lock {
		tx = lockForUpdate(tx)
	}
	var records []Model
	err := tx.Order("id").Find(&records).Error
	return records, err
}

func ListExactModelCapabilities() (*ModelCapabilityListing, error) {
	records, err := loadCapabilityRecords(DB, false)
	if err != nil {
		return nil, err
	}
	listing := &ModelCapabilityListing{
		Capabilities: map[string]json.RawMessage{},
		NonExact:     []ModelCapabilityRuleRef{},
		Unconfigured: []string{},
	}
	for _, record := range records {
		if record.NameRule != NameRuleExact {
			listing.NonExact = append(listing.NonExact, ModelCapabilityRuleRef{ModelName: record.ModelName, NameRule: record.NameRule})
			continue
		}
		if canonical := parseRecordCapability(record).MarshalCanonical(); canonical != "" {
			listing.Capabilities[record.ModelName] = json.RawMessage(canonical)
		} else {
			listing.Unconfigured = append(listing.Unconfigured, record.ModelName)
		}
	}
	slices.Sort(listing.Unconfigured)
	return listing, nil
}

func PlanModelCapabilityReplace(tx *gorm.DB, submitted map[string]json.RawMessage, lock bool) (*ModelCapabilityPlan, error) {
	records, err := loadCapabilityRecords(tx, lock)
	if err != nil {
		return nil, err
	}
	exact := map[string]Model{}
	ruleNames := map[string]bool{}
	var rules []Model
	for _, record := range records {
		if record.NameRule == NameRuleExact {
			exact[record.ModelName] = record
		} else {
			rules = append(rules, record)
			ruleNames[record.ModelName] = true
		}
	}

	plan := &ModelCapabilityPlan{
		Items:    []ModelCapabilityChange{},
		Errors:   []ModelCapabilityError{},
		Summary:  map[string]int{CapabilityActionCreate: 0, CapabilityActionUpdate: 0, CapabilityActionClear: 0, CapabilityActionUnchanged: 0},
		exactIDs: map[string]int{},
	}
	var involved []Model
	var createNames []string
	for _, name := range slices.Sorted(maps.Keys(submitted)) {
		if length := utf8.RuneCountInString(name); length < 1 || length > 128 {
			plan.Errors = append(plan.Errors, ModelCapabilityError{ModelName: name, Message: common.NewMessage("Model name must be 1 to 128 characters")})
			continue
		}
		if ruleNames[name] {
			plan.Errors = append(plan.Errors, ModelCapabilityError{ModelName: name, Message: common.NewMessage(
				"{{name}} is used by a prefix, suffix, or contains rule record. Edit that record in the drawer.", map[string]any{"name": name})})
			continue
		}
		capability, err := ParseModelCapability(string(submitted[name]))
		if err != nil {
			causes := []error{err}
			if joined, ok := err.(interface{ Unwrap() []error }); ok {
				causes = joined.Unwrap()
			}
			for _, cause := range causes {
				message, ok := cause.(*common.Message)
				if !ok {
					message = common.NewMessage(cause.Error())
				}
				plan.Errors = append(plan.Errors, ModelCapabilityError{ModelName: name, Message: message})
			}
			continue
		}
		after := capability.MarshalCanonical()
		record, found := exact[name]
		change := ModelCapabilityChange{ModelName: name, After: capabilityJSON(after)}
		switch {
		case !found:
			change.Action = CapabilityActionCreate
			createNames = append(createNames, name)
		default:
			before := parseRecordCapability(record).MarshalCanonical()
			change.Before = capabilityJSON(before)
			involved = append(involved, record)
			plan.exactIDs[name] = record.Id
			switch {
			case before == after:
				change.Action = CapabilityActionUnchanged
			case after == "":
				change.Action = CapabilityActionClear
			default:
				change.Action = CapabilityActionUpdate
			}
		}
		plan.Items = append(plan.Items, change)
	}
	for _, name := range slices.Sorted(maps.Keys(exact)) {
		if _, ok := submitted[name]; ok {
			continue
		}
		record := exact[name]
		before := parseRecordCapability(record).MarshalCanonical()
		if before == "" {
			continue
		}
		involved = append(involved, record)
		plan.exactIDs[name] = record.Id
		plan.Items = append(plan.Items, ModelCapabilityChange{
			ModelName: name, Action: CapabilityActionClear, Before: capabilityJSON(before), After: capabilityJSON(""),
		})
	}
	sort.Slice(plan.Items, func(i, j int) bool { return plan.Items[i].ModelName < plan.Items[j].ModelName })
	for _, item := range plan.Items {
		plan.Summary[item.Action]++
	}
	plan.Version = modelCapabilityReplaceVersion(involved, createNames, resolveModelMetadata(rules, createNames))
	return plan, nil
}

// modelCapabilityReplaceVersion 覆盖涉及记录、待新建的名字和它们复制来源的当前状态；任何一项变化都让预览作废。
func modelCapabilityReplaceVersion(involved []Model, createNames []string, sources map[string]*Model) string {
	slices.SortFunc(involved, func(a, b Model) int { return a.Id - b.Id })
	recordVersions := make([]string, 0, len(involved))
	for i := range involved {
		recordVersions = append(recordVersions, MetadataRecordVersion(&involved[i], nil, nil))
	}
	sourceVersions := make([]string, 0, len(createNames))
	for _, name := range createNames {
		version := ""
		if source := sources[name]; source != nil {
			version = MetadataRecordVersion(source, nil, nil)
		}
		sourceVersions = append(sourceVersions, version)
	}
	encoded, _ := common.Marshal([]any{recordVersions, createNames, sourceVersions})
	return fmt.Sprintf("%x", sha256.Sum256(encoded))
}

func ApplyModelCapabilityReplace(submitted map[string]json.RawMessage, version string) (map[string]int, error) {
	var counts map[string]int
	err := metadataTransaction(func(tx *gorm.DB) error {
		plan, err := PlanModelCapabilityReplace(tx, submitted, true)
		if err != nil {
			return err
		}
		if len(plan.Errors) > 0 {
			return plan.Errors[0].Message
		}
		if plan.Version != version {
			return ErrMetadataSyncConflict
		}
		now := common.GetTimestamp()
		for _, item := range plan.Items {
			text := string(item.After)
			if text == "{}" {
				text = ""
			}
			switch item.Action {
			case CapabilityActionCreate:
				if _, err := NewExactModelRecord(tx, item.ModelName, text); err != nil {
					return err
				}
			case CapabilityActionUpdate, CapabilityActionClear:
				if err := tx.Model(&Model{}).Where("id = ?", plan.exactIDs[item.ModelName]).Updates(map[string]any{"capabilities": text, "updated_time": now}).Error; err != nil {
					return err
				}
			}
		}
		counts = map[string]int{
			CapabilityActionCreate: plan.Summary[CapabilityActionCreate],
			CapabilityActionUpdate: plan.Summary[CapabilityActionUpdate],
			CapabilityActionClear:  plan.Summary[CapabilityActionClear],
		}
		return nil
	})
	return counts, err
}

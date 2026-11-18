package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const maxCapabilityNumber = 9007199254740991

var (
	capabilityInputModalities  = []string{"text", "image", "file", "audio"}
	capabilityOutputModalities = []string{"text", "image", "audio"}
	capabilityEfforts          = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}
)

// ModelCapability 的指针和切片指针为 nil 表示未配置；Efforts 指向空切片表示不支持思考。
type ModelCapability struct {
	ContextLength       *int64
	MaxCompletionTokens *int64
	InputModalities     *[]string
	OutputModalities    *[]string
	Efforts             *[]string
}

var (
	capabilityRecords     []Model
	capabilityRecordsLock sync.RWMutex
)

func parseCapabilityObject(raw []byte, path string, allowed []string) (map[string][]byte, []error) {
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(raw, &fields); err != nil || fields == nil {
		display := path
		if display == "" {
			display = "$"
		}
		return nil, []error{capabilityTypeError("{{path}} must be an object", display)}
	}
	var errs []error
	result := make(map[string][]byte, len(fields))
	for key, value := range fields {
		if !slices.Contains(allowed, key) {
			errs = append(errs, common.NewMessage("{{path}} is not a capability field", map[string]any{"path": joinCapabilityPath(path, key)}))
			continue
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			continue
		}
		result[key] = value
	}
	return result, errs
}

func capabilityTypeError(key, path string) error {
	return common.NewMessage(key, map[string]any{"path": path})
}

func joinCapabilityPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func parseCapabilityNumber(raw []byte, path string) (*int64, error) {
	number, err := strconv.ParseInt(string(bytes.TrimSpace(raw)), 10, 64)
	if err != nil || number < 1 || number > maxCapabilityNumber {
		return nil, common.NewMessage("{{path}} must be an integer from 1 to 9007199254740991", map[string]any{"path": path})
	}
	return &number, nil
}

func parseCapabilityList(raw []byte, path string, allowed []string) (*[]string, []error) {
	unknownValue := func(value string) error {
		return common.NewMessage("{{path}} contains an unknown value {{value}}. Allowed values: {{allowed}}", map[string]any{
			"path": path, "value": value, "allowed": strings.Join(allowed, ", "),
		})
	}
	var items []any
	if err := common.Unmarshal(raw, &items); err != nil {
		return nil, []error{capabilityTypeError("{{path}} must be an array of strings", path)}
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, []error{capabilityTypeError("{{path}} must be an array of strings", path)}
		}
		values = append(values, text)
	}
	var errs []error
	for _, value := range values {
		if !slices.Contains(allowed, value) {
			errs = append(errs, unknownValue(value))
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	sorted := make([]string, 0, len(values))
	for _, candidate := range allowed {
		if slices.Contains(values, candidate) {
			sorted = append(sorted, candidate)
		}
	}
	return &sorted, nil
}

// ParseModelCapability 解析能力 JSON 文本并规范化；空串等同于未配置。
func ParseModelCapability(text string) (ModelCapability, error) {
	var capability ModelCapability
	if strings.TrimSpace(text) == "" {
		return capability, nil
	}
	top, errs := parseCapabilityObject([]byte(text), "", []string{"context_length", "top_provider", "architecture", "reasoning"})
	if raw, ok := top["context_length"]; ok {
		number, err := parseCapabilityNumber(raw, "context_length")
		if err != nil {
			errs = append(errs, err)
		}
		capability.ContextLength = number
	}
	if raw, ok := top["top_provider"]; ok {
		fields, subErrs := parseCapabilityObject(raw, "top_provider", []string{"max_completion_tokens"})
		errs = append(errs, subErrs...)
		if value, ok := fields["max_completion_tokens"]; ok {
			number, err := parseCapabilityNumber(value, "top_provider.max_completion_tokens")
			if err != nil {
				errs = append(errs, err)
			}
			capability.MaxCompletionTokens = number
		}
	}
	if raw, ok := top["architecture"]; ok {
		fields, subErrs := parseCapabilityObject(raw, "architecture", []string{"input_modalities", "output_modalities"})
		errs = append(errs, subErrs...)
		if value, ok := fields["input_modalities"]; ok {
			list, listErrs := parseCapabilityList(value, "architecture.input_modalities", capabilityInputModalities)
			errs = append(errs, listErrs...)
			capability.InputModalities = list
		}
		if value, ok := fields["output_modalities"]; ok {
			list, listErrs := parseCapabilityList(value, "architecture.output_modalities", capabilityOutputModalities)
			errs = append(errs, listErrs...)
			capability.OutputModalities = list
		}
	}
	if raw, ok := top["reasoning"]; ok {
		fields, subErrs := parseCapabilityObject(raw, "reasoning", []string{"supported_efforts"})
		errs = append(errs, subErrs...)
		if value, ok := fields["supported_efforts"]; ok {
			list, listErrs := parseCapabilityList(value, "reasoning.supported_efforts", capabilityEfforts)
			errs = append(errs, listErrs...)
			capability.Efforts = list
		}
	}
	if len(errs) > 0 {
		return ModelCapability{}, errors.Join(errs...)
	}
	return capability.Normalize(), nil
}

// Normalize 把空模态数组变为未配置；其余字段解析时已去重排序。
func (c ModelCapability) Normalize() ModelCapability {
	if c.InputModalities != nil && len(*c.InputModalities) == 0 {
		c.InputModalities = nil
	}
	if c.OutputModalities != nil && len(*c.OutputModalities) == 0 {
		c.OutputModalities = nil
	}
	return c
}

func (c ModelCapability) IsEmpty() bool {
	return c.ContextLength == nil && c.MaxCompletionTokens == nil &&
		c.InputModalities == nil && c.OutputModalities == nil && c.Efforts == nil
}

// MarshalCanonical 输出键顺序固定的规范 JSON，全部未配置时为空串。
func (c ModelCapability) MarshalCanonical() string {
	type topProvider struct {
		MaxCompletionTokens int64 `json:"max_completion_tokens"`
	}
	type architecture struct {
		InputModalities  []string `json:"input_modalities,omitempty"`
		OutputModalities []string `json:"output_modalities,omitempty"`
	}
	type reasoning struct {
		SupportedEfforts []string `json:"supported_efforts"`
	}
	type document struct {
		ContextLength *int64        `json:"context_length,omitempty"`
		TopProvider   *topProvider  `json:"top_provider,omitempty"`
		Architecture  *architecture `json:"architecture,omitempty"`
		Reasoning     *reasoning    `json:"reasoning,omitempty"`
	}
	c = c.Normalize()
	if c.IsEmpty() {
		return ""
	}
	doc := document{ContextLength: c.ContextLength}
	if c.MaxCompletionTokens != nil {
		doc.TopProvider = &topProvider{MaxCompletionTokens: *c.MaxCompletionTokens}
	}
	if c.InputModalities != nil || c.OutputModalities != nil {
		doc.Architecture = &architecture{}
		if c.InputModalities != nil {
			doc.Architecture.InputModalities = *c.InputModalities
		}
		if c.OutputModalities != nil {
			doc.Architecture.OutputModalities = *c.OutputModalities
		}
	}
	if c.Efforts != nil {
		doc.Reasoning = &reasoning{SupportedEfforts: slices.Clone(*c.Efforts)}
	}
	encoded, err := common.Marshal(doc)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// ModelThinkingSupport 返回思考配置：configured 为 false 表示未配置；
// 已配置且档位为空表示不支持思考。
func ModelThinkingSupport(c ModelCapability) (efforts []string, configured bool) {
	if c.Efforts == nil {
		return nil, false
	}
	return slices.Clone(*c.Efforts), true
}

func setCapabilityRecords(records []Model) {
	capabilityRecordsLock.Lock()
	capabilityRecords = records
	capabilityRecordsLock.Unlock()
}

// GetModelCapabilityByName 按目录命中顺序找到记录，整条记录的能力生效。
func GetModelCapabilityByName(name string) ModelCapability {
	capabilityRecordsLock.RLock()
	records := capabilityRecords
	capabilityRecordsLock.RUnlock()
	record := resolveModelMetadata(records, []string{name})[name]
	if record == nil {
		return ModelCapability{}
	}
	capability, err := ParseModelCapability(record.Capabilities)
	if err != nil {
		return ModelCapability{}
	}
	return capability
}

func ValidateModelCapabilityText(text string) (string, error) {
	capability, err := ParseModelCapability(text)
	if err != nil {
		return "", err
	}
	return capability.MarshalCanonical(), nil
}

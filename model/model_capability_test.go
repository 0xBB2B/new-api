package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const fullCapabilityJSON = `{"context_length":1050000,"top_provider":{"max_completion_tokens":128000},"architecture":{"input_modalities":["text","image"],"output_modalities":["text"]},"reasoning":{"supported_efforts":["none","low","medium","high","xhigh","max"]}}`

func TestValidateModelCapabilityTextCanonical(t *testing.T) {
	cases := []struct{ name, input, expected string }{
		{"full_document_unchanged", fullCapabilityJSON, fullCapabilityJSON},
		{"empty_string_is_unconfigured", ``, ``},
		{"empty_object_is_unconfigured", `{}`, ``},
		{"max_safe_integer_context", `{"context_length":9007199254740991}`, `{"context_length":9007199254740991}`},
		{"max_safe_integer_completion", `{"top_provider":{"max_completion_tokens":9007199254740991}}`, `{"top_provider":{"max_completion_tokens":9007199254740991}}`},
		{"minimum_value_one", `{"context_length":1}`, `{"context_length":1}`},
		{"efforts_deduplicated_and_ordered", `{"reasoning":{"supported_efforts":["high","low","high"]}}`, `{"reasoning":{"supported_efforts":["low","high"]}}`},
		{"efforts_full_order", `{"reasoning":{"supported_efforts":["max","xhigh","high","medium","low","minimal","none"]}}`, `{"reasoning":{"supported_efforts":["none","minimal","low","medium","high","xhigh","max"]}}`},
		{"input_modalities_ordered", `{"architecture":{"input_modalities":["audio","file","image","text","audio"]}}`, `{"architecture":{"input_modalities":["text","image","file","audio"]}}`},
		{"output_modalities_ordered", `{"architecture":{"output_modalities":["audio","image","text","image"]}}`, `{"architecture":{"output_modalities":["text","image","audio"]}}`},
		{"keys_reordered", `{"reasoning":{"supported_efforts":["low"]},"architecture":{"output_modalities":["text"],"input_modalities":["text"]},"top_provider":{"max_completion_tokens":5},"context_length":9}`, `{"context_length":9,"top_provider":{"max_completion_tokens":5},"architecture":{"input_modalities":["text"],"output_modalities":["text"]},"reasoning":{"supported_efforts":["low"]}}`},
		{"empty_input_modalities_unconfigured", `{"architecture":{"input_modalities":[]}}`, ``},
		{"empty_input_modalities_keeps_output", `{"architecture":{"input_modalities":[],"output_modalities":["text"]}}`, `{"architecture":{"output_modalities":["text"]}}`},
		{"empty_output_modalities_unconfigured", `{"architecture":{"output_modalities":[]}}`, ``},
		{"null_context_unconfigured", `{"context_length":null}`, ``},
		{"null_fields_everywhere", `{"context_length":null,"top_provider":null,"architecture":{"input_modalities":null,"output_modalities":null},"reasoning":null}`, ``},
		{"null_completion_omits_container", `{"top_provider":{"max_completion_tokens":null},"context_length":3}`, `{"context_length":3}`},
		{"empty_efforts_means_no_reasoning", `{"reasoning":{"supported_efforts":[]}}`, `{"reasoning":{"supported_efforts":[]}}`},
		{"empty_containers_omitted", `{"top_provider":{},"architecture":{},"reasoning":{}}`, ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := ValidateModelCapabilityText(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
			_, err = ParseModelCapability(tc.input)
			require.NoError(t, err)
		})
	}
}

func TestValidateModelCapabilityTextRejects(t *testing.T) {
	cases := []struct{ name, input string }{
		{"video_input_modality", `{"architecture":{"input_modalities":["video"]}}`},
		{"video_output_modality", `{"architecture":{"output_modalities":["video"]}}`},
		{"file_output_modality", `{"architecture":{"output_modalities":["file"]}}`},
		{"unknown_effort", `{"reasoning":{"supported_efforts":["ultra"]}}`},
		{"unknown_top_level_field", `{"foo":1}`},
		{"unknown_top_provider_field", `{"top_provider":{"foo":1}}`},
		{"unknown_architecture_field", `{"architecture":{"modality":"text"}}`},
		{"unknown_reasoning_field", `{"reasoning":{"foo":1}}`},
		{"context_zero", `{"context_length":0}`},
		{"context_negative", `{"context_length":-5}`},
		{"context_fraction", `{"context_length":1.5}`},
		{"context_above_safe_integer", `{"context_length":9007199254740992}`},
		{"context_string", `{"context_length":"100"}`},
		{"completion_zero", `{"top_provider":{"max_completion_tokens":0}}`},
		{"completion_negative", `{"top_provider":{"max_completion_tokens":-1}}`},
		{"completion_fraction", `{"top_provider":{"max_completion_tokens":1.5}}`},
		{"completion_above_safe_integer", `{"top_provider":{"max_completion_tokens":9007199254740992}}`},
		{"not_json", `not json`},
		{"array_document", `[]`},
		{"modalities_not_array", `{"architecture":{"input_modalities":"text"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateModelCapabilityText(tc.input)
			assert.Error(t, err)
			_, err = ParseModelCapability(tc.input)
			assert.Error(t, err)
		})
	}
}

func capabilityErrorMessages(t *testing.T, err error) []*common.Message {
	t.Helper()
	require.Error(t, err)
	var messages []*common.Message
	var walk func(error)
	walk = func(current error) {
		if joined, ok := current.(interface{ Unwrap() []error }); ok {
			for _, inner := range joined.Unwrap() {
				walk(inner)
			}
			return
		}
		var message *common.Message
		require.True(t, errors.As(current, &message), "error is not a common.Message: %v", current)
		messages = append(messages, message)
	}
	walk(err)
	return messages
}

func TestModelCapabilityErrorMessages(t *testing.T) {
	const (
		unknownKey = "{{path}} contains an unknown value {{value}}. Allowed values: {{allowed}}"
		numberKey  = "{{path}} must be an integer from 1 to 9007199254740991"
		fieldKey   = "{{path}} is not a capability field"
	)
	cases := []struct {
		name, input string
		key         string
		params      map[string]any
	}{
		{"unknown_input_modality", `{"architecture":{"input_modalities":["video"]}}`, unknownKey,
			map[string]any{"path": "architecture.input_modalities", "value": "video", "allowed": "text, image, file, audio"}},
		{"unknown_output_modality", `{"architecture":{"output_modalities":["file"]}}`, unknownKey,
			map[string]any{"path": "architecture.output_modalities", "value": "file", "allowed": "text, image, audio"}},
		{"unknown_effort", `{"reasoning":{"supported_efforts":["ultra"]}}`, unknownKey,
			map[string]any{"path": "reasoning.supported_efforts", "value": "ultra", "allowed": "none, minimal, low, medium, high, xhigh, max"}},
		{"context_zero", `{"context_length":0}`, numberKey, map[string]any{"path": "context_length"}},
		{"completion_fraction", `{"top_provider":{"max_completion_tokens":1.5}}`, numberKey, map[string]any{"path": "top_provider.max_completion_tokens"}},
		{"extra_top_level_field", `{"foo":1}`, fieldKey, map[string]any{"path": "foo"}},
		{"extra_nested_field", `{"reasoning":{"foo":1}}`, fieldKey, map[string]any{"path": "reasoning.foo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseModelCapability(tc.input)
			messages := capabilityErrorMessages(t, err)
			require.Len(t, messages, 1)
			assert.Equal(t, tc.key, messages[0].Key)
			assert.Equal(t, tc.params, messages[0].Params)
			_, err = ValidateModelCapabilityText(tc.input)
			messages = capabilityErrorMessages(t, err)
			require.Len(t, messages, 1)
			assert.Equal(t, tc.key, messages[0].Key)
			assert.Equal(t, tc.params, messages[0].Params)
		})
	}
}

func TestModelCapabilityWrongTypeErrors(t *testing.T) {
	const (
		objectKey = "{{path}} must be an object"
		arrayKey  = "{{path}} must be an array of strings"
	)
	cases := []struct{ input, key, path string }{
		{`{"top_provider":5}`, objectKey, "top_provider"},
		{`{"architecture":[]}`, objectKey, "architecture"},
		{`{"reasoning":"high"}`, objectKey, "reasoning"},
		{`{"architecture":{"input_modalities":"text"}}`, arrayKey, "architecture.input_modalities"},
		{`{"architecture":{"output_modalities":[1]}}`, arrayKey, "architecture.output_modalities"},
		{`{"reasoning":{"supported_efforts":["low",null]}}`, arrayKey, "reasoning.supported_efforts"},
		{`{"reasoning":{"supported_efforts":{}}}`, arrayKey, "reasoning.supported_efforts"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			_, err := ValidateModelCapabilityText(tc.input)
			messages := capabilityErrorMessages(t, err)
			require.Len(t, messages, 1)
			assert.Equal(t, tc.key, messages[0].Key)
			assert.Equal(t, map[string]any{"path": tc.path}, messages[0].Params)
		})
	}
	for _, input := range []string{`null`, `[]`, `5`, `"text"`} {
		t.Run("root/"+input, func(t *testing.T) {
			_, err := ValidateModelCapabilityText(input)
			messages := capabilityErrorMessages(t, err)
			require.Len(t, messages, 1)
			assert.Equal(t, objectKey, messages[0].Key)
			path, _ := messages[0].Params["path"].(string)
			assert.NotEmpty(t, path)
			assert.Len(t, messages[0].Params, 1)
		})
	}
}

func TestModelCapabilityCacheIndependentOfAbilities(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}, &Model{}, &Vendor{}))
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })
	require.NoError(t, db.Create(&Model{ModelName: "cap-cache-model", Capabilities: `{"reasoning":{"supported_efforts":["low","high"]}}`}).Error)

	abilityQueries := 0
	failAbilities := func(tx *gorm.DB) {
		if tx.Statement.Table == "abilities" {
			abilityQueries++
			tx.AddError(errors.New("abilities unavailable"))
		}
	}
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:fail_abilities_query", failAbilities))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("test:fail_abilities_row", failAbilities))
	t.Cleanup(func() {
		require.NoError(t, db.Callback().Query().Remove("test:fail_abilities_query"))
		require.NoError(t, db.Callback().Row().Remove("test:fail_abilities_row"))
	})

	RefreshPricing()

	assert.Positive(t, abilityQueries)
	efforts, configured := ModelThinkingSupport(GetModelCapabilityByName("cap-cache-model"))
	assert.True(t, configured)
	assert.Equal(t, []string{"low", "high"}, efforts)
}

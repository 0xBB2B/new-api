package controller

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTaskLogTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func setupTaskUserDisplayNameDatabase(t *testing.T) {
	t.Helper()
	db := newTaskLogTestDatabase(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "zhangsan", DisplayName: "张三", Password: "password", AffCode: "aff-7"}).Error)
	require.NoError(t, db.Create(&model.User{Id: 8, Username: "lisi", DisplayName: "", Password: "password", AffCode: "aff-8"}).Error)
}

func TestTaskLogDTOSeparatesUserAdminAndRootDetails(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_public",
		Platform: "document-parser",
		PrivateData: model.TaskPrivateData{
			Key:            "channel-secret-canary",
			UpstreamTaskID: "upstream-private",
			NodeName:       "node-a",
			Execution: &model.TaskExecutionSnapshot{
				RequestID:   "request-public",
				RequestPath: "/v1/documents",
				TaskPlugin: &model.TaskPluginSnapshot{
					Key:     "document-parser",
					Name:    "Document Parser",
					Version: "1.2.3",
					Author: &model.TaskPluginAuthorSnapshot{
						Name: "Community Author",
						URL:  "https://plugins.example/author",
					},
					APIVersion: 1,
					Generation: 42,
				},
			},
		},
	}

	userDtos, err := tasksToDto([]*model.Task{task}, false, common.RoleCommonUser)
	require.NoError(t, err)
	userView := userDtos[0]
	assert.Nil(t, userView.AdminInfo)
	assert.Nil(t, userView.RootInfo)

	adminDtos, err := tasksToDto([]*model.Task{task}, false, common.RoleAdminUser)
	require.NoError(t, err)
	adminView := adminDtos[0]
	require.NotNil(t, adminView.AdminInfo)
	require.NotNil(t, adminView.AdminInfo.TaskPlugin)
	assert.Equal(t, "document-parser", adminView.AdminInfo.TaskPlugin.Key)
	assert.Equal(t, "Document Parser", adminView.AdminInfo.TaskPlugin.Name)
	assert.Equal(t, "1.2.3", adminView.AdminInfo.TaskPlugin.Version)
	require.NotNil(t, adminView.AdminInfo.TaskPlugin.Author)
	assert.Equal(t, "Community Author", adminView.AdminInfo.TaskPlugin.Author.Name)
	assert.Equal(t, "https://plugins.example/author", adminView.AdminInfo.TaskPlugin.Author.URL)
	assert.Equal(t, "request-public", adminView.AdminInfo.RequestID)
	assert.Equal(t, "/v1/documents", adminView.AdminInfo.RequestPath)
	assert.Nil(t, adminView.RootInfo)

	rootDtos, err := tasksToDto([]*model.Task{task}, false, common.RoleRootUser)
	require.NoError(t, err)
	rootView := rootDtos[0]
	require.NotNil(t, rootView.AdminInfo)
	require.NotNil(t, rootView.RootInfo)
	require.NotNil(t, rootView.RootInfo.TaskPlugin)
	assert.Equal(t, 1, rootView.RootInfo.TaskPlugin.APIVersion)
	assert.Equal(t, uint64(42), rootView.RootInfo.TaskPlugin.Generation)
	assert.Equal(t, "upstream-private", rootView.RootInfo.UpstreamTaskID)
	assert.Equal(t, "node-a", rootView.RootInfo.NodeName)

	adminJSON, err := common.Marshal(adminView)
	require.NoError(t, err)
	assert.NotContains(t, string(adminJSON), "channel-secret-canary")
	assert.NotContains(t, string(adminJSON), "upstream-private")

	rootJSON, err := common.Marshal(rootView)
	require.NoError(t, err)
	assert.NotContains(t, string(rootJSON), "channel-secret-canary")
	assert.Contains(t, string(rootJSON), "upstream-private")
}

func TestTaskLogDTODoesNotInventHistoricalPluginProvenance(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_without_snapshot",
		Platform: "document-parser",
	}

	adminDtos, err := tasksToDto([]*model.Task{task}, false, common.RoleAdminUser)
	require.NoError(t, err)
	adminView := adminDtos[0]

	assert.Nil(t, adminView.AdminInfo)
	assert.Nil(t, adminView.RootInfo)
}

func TestTaskLogDTOReplacesLegacyVideoURLWithAvailabilityFlag(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_legacy_video",
		Platform:   "jimeng",
		Action:     constant.TaskActionTextToVideo,
		Status:     model.TaskStatusSuccess,
		FailReason: "https://private-upstream.invalid/video.mp4?signature=secret",
	}

	dtos, err := tasksToDto([]*model.Task{task}, false, common.RoleCommonUser)
	require.NoError(t, err)
	view := dtos[0]
	assert.True(t, view.LegacyVideoAvailable)
	assert.Empty(t, view.ResultURL)
	assert.Empty(t, view.FailReason)
	encoded, err := common.Marshal(view)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "private-upstream.invalid")
	assert.NotContains(t, string(encoded), "result_url")
	assert.Contains(t, string(encoded), "legacy_video_available")
}

func TestTaskLogDTOKeepsFailureReasonAndDoesNotMarkPluginTaskLegacy(t *testing.T) {
	failed := &model.Task{
		TaskID:     "task_failed",
		Platform:   "jimeng",
		Action:     constant.TaskActionTextToVideo,
		Status:     model.TaskStatusFailure,
		FailReason: "provider rejected the request",
	}
	failedDtos, err := tasksToDto([]*model.Task{failed}, false, common.RoleCommonUser)
	require.NoError(t, err)
	failedView := failedDtos[0]
	assert.Equal(t, "provider rejected the request", failedView.FailReason)
	assert.False(t, failedView.LegacyVideoAvailable)

	pluginTask := &model.Task{
		TaskID:     "task_plugin_video",
		Platform:   "community-video",
		Action:     constant.TaskActionTextToVideo,
		Status:     model.TaskStatusSuccess,
		FailReason: "https://stale-upstream.invalid/plugin-video.mp4",
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://private-upstream.invalid/plugin-video.mp4",
			Execution: &model.TaskExecutionSnapshot{
				TaskPlugin: &model.TaskPluginSnapshot{Key: "community-video"},
			},
		},
	}
	pluginDtos, err := tasksToDto([]*model.Task{pluginTask}, false, common.RoleCommonUser)
	require.NoError(t, err)
	pluginView := pluginDtos[0]
	assert.False(t, pluginView.LegacyVideoAvailable)
	assert.Empty(t, pluginView.ResultURL)
	assert.Empty(t, pluginView.FailReason)
}

func TestTaskLogDTOFillsUsernameAndDisplayNameForAdminView(t *testing.T) {
	setupTaskUserDisplayNameDatabase(t)
	tasks := []*model.Task{
		{ID: 1, TaskID: "task_1", Platform: "jimeng", Status: model.TaskStatusSuccess, UserId: 7},
		{ID: 2, TaskID: "task_2", Platform: "jimeng", Status: model.TaskStatusSuccess, UserId: 8},
		{ID: 3, TaskID: "task_3", Platform: "jimeng", Status: model.TaskStatusSuccess, UserId: 9},
	}

	dtos, err := tasksToDto(tasks, true, common.RoleAdminUser)
	require.NoError(t, err)
	require.Len(t, dtos, 3)

	assert.Equal(t, "zhangsan", dtos[0].Username)
	assert.Equal(t, "张三", dtos[0].DisplayName)
	assert.Equal(t, "lisi", dtos[1].Username)
	assert.Empty(t, dtos[1].DisplayName)
	assert.Empty(t, dtos[2].Username)
	assert.Empty(t, dtos[2].DisplayName)

	firstJSON, err := common.Marshal(dtos[0])
	require.NoError(t, err)
	assert.Contains(t, string(firstJSON), `"display_name":"张三"`)

	secondJSON, err := common.Marshal(dtos[1])
	require.NoError(t, err)
	assert.NotContains(t, string(secondJSON), "display_name")
}

func TestTaskLogDTOOwnerViewDoesNotFillUserFields(t *testing.T) {
	tasks := []*model.Task{
		{ID: 1, TaskID: "task_1", Platform: "jimeng", Status: model.TaskStatusSuccess, UserId: 7},
		{ID: 2, TaskID: "task_2", Platform: "jimeng", Status: model.TaskStatusSuccess, UserId: 8},
		{ID: 3, TaskID: "task_3", Platform: "jimeng", Status: model.TaskStatusSuccess, UserId: 9},
	}

	dtos, err := tasksToDto(tasks, false, common.RoleCommonUser)
	require.NoError(t, err)
	require.Len(t, dtos, 3)
	for _, item := range dtos {
		assert.Empty(t, item.Username)
		assert.Empty(t, item.DisplayName)
	}
}

func TestTaskLogDTOReturnsErrorWhenUserQueryFails(t *testing.T) {
	newTaskLogTestDatabase(t)

	tasks := []*model.Task{
		{ID: 1, TaskID: "task_1", Platform: "jimeng", Status: model.TaskStatusSuccess, UserId: 7},
	}

	dtos, err := tasksToDto(tasks, true, common.RoleAdminUser)
	assert.Error(t, err)
	assert.Nil(t, dtos)
}

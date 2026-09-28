package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheGetNextSatisfiedChannelCyclesByID(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "channel-id-retry-model"
	for _, channelID := range []int{1, 3, 4, 7} {
		createChannelSelectAutoGroupsChannel(t, db, channelID, "default", modelName)
	}
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "default",
		ModelName:   modelName,
		RequestPath: "/v1/chat/completions",
		Retry:       common.GetPointer(1),
	}

	for _, test := range []struct {
		afterChannelID int
		wantChannelID  int
	}{
		{afterChannelID: 1, wantChannelID: 3},
		{afterChannelID: 3, wantChannelID: 4},
		{afterChannelID: 4, wantChannelID: 7},
		{afterChannelID: 7, wantChannelID: 1},
	} {
		channel, selectedGroup, err := CacheGetNextSatisfiedChannel(param, test.afterChannelID)
		require.NoError(t, err)
		require.NotNil(t, channel)
		assert.Equal(t, "default", selectedGroup)
		assert.Equal(t, test.wantChannelID, channel.Id)
	}
}

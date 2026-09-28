package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetNextSatisfiedChannelCyclesByID(t *testing.T) {
	for _, memoryCacheEnabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("memory_cache_%t", memoryCacheEnabled), func(t *testing.T) {
			originalMemoryCacheEnabled := common.MemoryCacheEnabled
			common.MemoryCacheEnabled = memoryCacheEnabled
			const modelName = "channel-id-retry-model"
			channelIDs := []int{910001, 910003, 910004, 910007}
			require.NoError(t, DB.Where("model = ?", modelName).Delete(&Ability{}).Error)
			require.NoError(t, DB.Where("id IN ?", channelIDs).Delete(&Channel{}).Error)
			t.Cleanup(func() {
				require.NoError(t, DB.Where("model = ?", modelName).Delete(&Ability{}).Error)
				require.NoError(t, DB.Where("id IN ?", channelIDs).Delete(&Channel{}).Error)
				common.MemoryCacheEnabled = originalMemoryCacheEnabled
				InitChannelCache()
			})

			priority := int64(0)
			weight := uint(100)
			for _, channelID := range channelIDs {
				channel := &Channel{
					Id:       channelID,
					Type:     constant.ChannelTypeOpenAI,
					Key:      fmt.Sprintf("key-%d", channelID),
					Status:   common.ChannelStatusEnabled,
					Name:     fmt.Sprintf("channel-%d", channelID),
					Weight:   &weight,
					Models:   modelName,
					Group:    "default",
					Priority: &priority,
				}
				require.NoError(t, DB.Create(channel).Error)
				require.NoError(t, DB.Create(&Ability{
					Group:     "default",
					Model:     modelName,
					ChannelId: channelID,
					Enabled:   true,
					Priority:  &priority,
					Weight:    weight,
				}).Error)
			}
			if memoryCacheEnabled {
				InitChannelCache()
			}

			for _, test := range []struct {
				afterChannelID int
				wantChannelID  int
			}{
				{afterChannelID: 910001, wantChannelID: 910003},
				{afterChannelID: 910003, wantChannelID: 910004},
				{afterChannelID: 910004, wantChannelID: 910007},
				{afterChannelID: 910007, wantChannelID: 910001},
			} {
				channel, err := GetNextSatisfiedChannel("default", modelName, test.afterChannelID, "/v1/chat/completions")
				require.NoError(t, err)
				require.NotNil(t, channel)
				assert.Equal(t, test.wantChannelID, channel.Id)
			}
		})
	}
}

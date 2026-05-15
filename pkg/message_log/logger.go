package messagelog

import (
	"bytes"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/message_log_setting"
)

type LogEntry struct {
	RequestId         string
	UserId            int
	TokenId           int
	ChannelId         int
	ModelName         string
	UpstreamModelName string
	GroupName         string
	RequestURL        string
	RequestMethod     string
	RequestHeaders    map[string]string
	RequestBody       []byte
	ResponseStatus    int
	ResponseHeaders   map[string]string
	ResponseBody      *bytes.Buffer
	IsStream          bool
	CreatedAt         int64
}

var (
	logChan  chan *LogEntry
	initOnce sync.Once
)

const (
	channelSize    = 4096
	batchSize      = 50
	flushInterval  = time.Second
)

func Init() {
	initOnce.Do(func() {
		logChan = make(chan *LogEntry, channelSize)
		go startWorker()
		go startCleanupLoop()
	})
}

func ShouldLog() bool {
	return message_log_setting.IsEnabled()
}

func Submit(entry *LogEntry) {
	if entry == nil {
		return
	}
	select {
	case logChan <- entry:
	default:
		common.SysLog("message_log: channel full, dropping entry")
	}
}

func startWorker() {
	batch := make([]*model.MessageLog, 0, batchSize)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case entry := <-logChan:
			record := entryToModel(entry)
			batch = append(batch, record)
			if len(batch) >= batchSize {
				flush(batch)
				batch = make([]*model.MessageLog, 0, batchSize)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flush(batch)
				batch = make([]*model.MessageLog, 0, batchSize)
			}
		}
	}
}

func flush(batch []*model.MessageLog) {
	if err := model.BatchCreateMessageLogs(batch); err != nil {
		common.SysError("message_log: batch insert failed: " + err.Error())
	}
}

func entryToModel(e *LogEntry) *model.MessageLog {
	var respBody []byte
	if e.ResponseBody != nil {
		respBody = e.ResponseBody.Bytes()
	}
	bodySize := int64(len(e.RequestBody)) + int64(len(respBody))
	return &model.MessageLog{
		RequestId:         e.RequestId,
		UserId:            e.UserId,
		TokenId:           e.TokenId,
		ChannelId:         e.ChannelId,
		ModelName:         e.ModelName,
		UpstreamModelName: e.UpstreamModelName,
		GroupName:         e.GroupName,
		RequestURL:        e.RequestURL,
		RequestMethod:     e.RequestMethod,
		RequestHeaders:    MarshalHeaders(e.RequestHeaders),
		RequestBody:       e.RequestBody,
		ResponseStatus:    e.ResponseStatus,
		ResponseHeaders:   MarshalHeaders(e.ResponseHeaders),
		ResponseBody:      respBody,
		IsStream:          e.IsStream,
		BodySize:          bodySize,
		CreatedAt:         e.CreatedAt,
	}
}

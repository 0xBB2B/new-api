package helper

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/MoeForge/moego/ojson"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// streamKeepAliveWriter 在业务写入前保活，写入开始时等待心跳退出。
type streamKeepAliveWriter struct {
	gin.ResponseWriter
	mutex       sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
	pingContext *gin.Context
}

func (writer *streamKeepAliveWriter) stop() {
	writer.cancel()
	<-writer.done
}

func (writer *streamKeepAliveWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (writer *streamKeepAliveWriter) Write(data []byte) (int, error) {
	writer.stop()
	return writer.ResponseWriter.Write(data)
}

func (writer *streamKeepAliveWriter) WriteString(data string) (int, error) {
	writer.stop()
	return writer.ResponseWriter.WriteString(data)
}

func (writer *streamKeepAliveWriter) WriteHeader(status int) {
	writer.stop()
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *streamKeepAliveWriter) WriteHeaderNow() {
	writer.stop()
	writer.ResponseWriter.WriteHeaderNow()
}

func (writer *streamKeepAliveWriter) Flush() {
	writer.stop()
	writer.ResponseWriter.Flush()
}

func (writer *streamKeepAliveWriter) Status() int {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return writer.ResponseWriter.Status()
}

func (writer *streamKeepAliveWriter) Size() int {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return writer.ResponseWriter.Size()
}

func (writer *streamKeepAliveWriter) Written() bool {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return writer.ResponseWriter.Written()
}

func (writer *streamKeepAliveWriter) ping() error {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return PingData(writer.pingContext)
}

// StartStreamKeepAlive 按 interval 发送心跳，覆盖请求准备和重试；业务写入开始后交给流处理器。
// 内层请求复用外层的保活，内层清理不会提前结束它。
func StartStreamKeepAlive(c *gin.Context, interval time.Duration) func() {
	if previous := keepAliveWriter(c); previous != nil {
		select {
		case <-previous.done:
			if c.Writer == gin.ResponseWriter(previous) {
				c.Writer = previous.ResponseWriter
			}
		default:
			return func() {}
		}
	}
	if requestContextDone(c) {
		return func() {}
	}
	if interval <= 0 {
		interval = DefaultPingInterval
	}
	SetEventStreamHeaders(c)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Minute)
	pingContext := c.Copy()
	// 心跳使用原 writer，业务写入才会触发交接。
	pingContext.Writer = c.Writer
	writer := &streamKeepAliveWriter{ResponseWriter: c.Writer, cancel: cancel, done: make(chan struct{}), pingContext: pingContext}
	c.Writer = writer
	go writer.keepAlive(ctx, interval)
	return writer.stop
}

// keepAliveWriter 返回 c.Writer 链上的请求保活，包括被其他 writer（如请求插件的回复检查）包裹的情况。
func keepAliveWriter(c *gin.Context) *streamKeepAliveWriter {
	var writer http.ResponseWriter = c.Writer
	for writer != nil {
		if keepAlive, ok := writer.(*streamKeepAliveWriter); ok {
			return keepAlive
		}
		unwrapper, ok := writer.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return nil
		}
		writer = unwrapper.Unwrap()
	}
	return nil
}

func (writer *streamKeepAliveWriter) keepAlive(ctx context.Context, interval time.Duration) {
	defer close(writer.done)
	defer writer.cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.LogDebug(writer.pingContext, "SSE ping goroutine panic recovered: %v", recovered)
		}
	}()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil || writer.ping() != nil {
				return
			}
		}
	}
}

func FlushWriter(c *gin.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("flush panic recovered: %v", r)
		}
	}()

	if c == nil || c.Writer == nil {
		return nil
	}

	if requestContextDone(c) {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return errors.New("streaming error: flusher not found")
	}

	flusher.Flush()
	// Gin's http.Flusher wrapper discards the underlying FlushError result.
	// Flush wrappers first so their buffers are preserved, then surface any
	// stored write error from an error-aware writer underneath them.
	var writer http.ResponseWriter = c.Writer
	for {
		if flusher, ok := writer.(interface{ FlushError() error }); ok {
			return flusher.FlushError()
		}
		unwrapper, ok := writer.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			break
		}
		writer = unwrapper.Unwrap()
	}
	return nil
}

func requestContextDone(c *gin.Context) bool {
	return c != nil && c.Request != nil && c.Request.Context().Err() != nil
}

func SetEventStreamHeaders(c *gin.Context) {
	// 检查是否已经设置过头部
	if _, exists := c.Get("event_stream_headers_set"); exists {
		return
	}

	// 设置标志，表示头部已经设置过
	c.Set("event_stream_headers_set", true)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer = &streamResponseWriter{ResponseWriter: c.Writer}
}

func MarkOutputSent(c *gin.Context) {
	common.SetContextKey(c, constant.ContextKeyStreamOutputSent, true)
}

const (
	streamHoldKey    = "stream_hold"
	failoverProbeKey = "stream_failover_probe"
)

type chunkKind int

const (
	chunkBody chunkKind = iota
	chunkOpening
	chunkFailure
	chunkTerminal
)

// streamHold 扣住正文开始前的开场事件，让正文前失败的尝试对客户端无痕，可以换渠道重试。
type streamHold struct {
	queue     []func() error
	bodyBegun bool
	failure   *types.NewAPIError
}

func streamHoldOf(c *gin.Context) *streamHold {
	value, _ := c.Get(streamHoldKey)
	hold, _ := value.(*streamHold)
	return hold
}

func (h *streamHold) flush() error {
	queue := h.queue
	h.queue = nil
	for _, write := range queue {
		if err := write(); err != nil {
			return err
		}
	}
	return nil
}

// fail 处理正文前的失败：探针确认会换渠道则丢弃扣住的事件并记下失败，否则补发扣住的事件并结束扣留。
func (h *streamHold) fail(c *gin.Context, failure *types.NewAPIError) (bool, error) {
	if value, _ := c.Get(failoverProbeKey); value != nil {
		if probe, ok := value.(func(*types.NewAPIError) bool); ok && probe(failure) {
			h.queue = nil
			h.failure = failure
			return true, nil
		}
	}
	h.bodyBegun = true
	return false, h.flush()
}

func StartStreamHold(c *gin.Context) {
	c.Set(streamHoldKey, &streamHold{})
}

func ResetStreamHold(c *gin.Context) {
	c.Set(streamHoldKey, nil)
	c.Set(claudeSignatureBufferKey, nil)
}

func SetFailoverProbe(c *gin.Context, probe func(*types.NewAPIError) bool) {
	c.Set(failoverProbeKey, probe)
}

func PreContentFailure(c *gin.Context) *types.NewAPIError {
	if hold := streamHoldOf(c); hold != nil {
		return hold.failure
	}
	return nil
}

// RecordStreamFailure 记录不经过写出函数的正文前失败（如 Claude 原生流的 error 事件）。
func RecordStreamFailure(c *gin.Context, failure *types.NewAPIError) {
	if hold := streamHoldOf(c); hold != nil && !hold.bodyBegun && hold.failure == nil {
		_, _ = hold.fail(c, failure)
	}
}

// FinishStreamHold 在流结束时调用。ended 表示上游已发出不经过写出函数的终止标记（如 Chat 的 [DONE]）。
func FinishStreamHold(c *gin.Context, ended bool) *types.NewAPIError {
	hold := streamHoldOf(c)
	if hold == nil || hold.bodyBegun || requestContextDone(c) || common.GetContextKeyBool(c, constant.ContextKeyStreamOutputSent) {
		return nil
	}
	if hold.failure != nil {
		return hold.failure
	}
	if !ended {
		failure := types.NewOpenAIError(errors.New("upstream stream ended before any content"), types.ErrorCodeBadResponse, http.StatusBadGateway)
		if failover, _ := hold.fail(c, failure); failover {
			return failure
		}
		return nil
	}
	hold.bodyBegun = true
	_ = hold.flush()
	return nil
}

func emit(c *gin.Context, classify func() chunkKind, failure func() *types.NewAPIError, write func() error) error {
	hold := streamHoldOf(c)
	if hold == nil || hold.bodyBegun {
		return write()
	}
	if hold.failure != nil {
		return nil
	}
	switch classify() {
	case chunkOpening:
		hold.queue = append(hold.queue, write)
		return nil
	case chunkFailure:
		failover, err := hold.fail(c, failure())
		if failover || err != nil {
			return err
		}
		return write()
	}
	hold.bodyBegun = true
	if err := hold.flush(); err != nil {
		return err
	}
	return write()
}

func upstreamStatus(code any) int {
	if status, err := strconv.Atoi(fmt.Sprint(code)); err == nil && status >= http.StatusBadRequest && status < 600 {
		return status
	}
	return http.StatusInternalServerError
}

func claudeChunkKind(eventType string) chunkKind {
	switch eventType {
	case "message_start", "ping":
		return chunkOpening
	case "error":
		return chunkFailure
	case "message_stop":
		return chunkTerminal
	}
	return chunkBody
}

func claudeStreamFailure(claudeError *types.ClaudeError) *types.NewAPIError {
	if claudeError == nil {
		claudeError = &types.ClaudeError{}
	}
	return types.WithClaudeError(*claudeError, http.StatusInternalServerError)
}

func responsesChunkKind(resp *dto.ResponsesStreamResponse) chunkKind {
	if service.IsResponsesFailureEvent(resp) {
		return chunkFailure
	}
	switch resp.Type {
	case "response.created", "response.in_progress", "response.queued":
		return chunkOpening
	case "response.completed", "response.done", "response.incomplete":
		return chunkTerminal
	}
	return chunkBody
}

func responsesStreamFailure(resp *dto.ResponsesStreamResponse) *types.NewAPIError {
	if resp.Response != nil {
		if oaiErr := resp.Response.GetOpenAIError(); oaiErr != nil {
			return types.WithOpenAIError(*oaiErr, upstreamStatus(oaiErr.Code))
		}
	}
	if resp.Message != "" {
		return types.WithOpenAIError(types.OpenAIError{Message: resp.Message, Code: resp.Code}, upstreamStatus(resp.Code))
	}
	return types.NewOpenAIError(fmt.Errorf("responses stream error: %s", resp.Type), types.ErrorCodeBadResponse, http.StatusInternalServerError)
}

func chatChunkKind(str string) chunkKind {
	if str == "[DONE]" {
		return chunkTerminal
	}
	if ojson.Get(str, "error.type").String() != "" || ojson.Get(str, "error.message").String() != "" {
		return chunkFailure
	}
	var chunk dto.ChatCompletionsStreamResponse
	if err := common.UnmarshalJsonStr(str, &chunk); err != nil || chunk.Usage != nil {
		return chunkBody
	}
	if len(chunk.Choices) > 0 && !ojson.Get(str, "choices.0.delta").Exists() {
		return chunkBody
	}
	for _, choice := range chunk.Choices {
		if choice.Delta.GetContentString() != "" || choice.Delta.GetReasoningContent() != "" || len(choice.Delta.ToolCalls) > 0 ||
			(choice.FinishReason != nil && *choice.FinishReason != "") {
			return chunkBody
		}
	}
	return chunkOpening
}

func chatChunkFailure(str string) *types.NewAPIError {
	var chunk struct {
		Error types.OpenAIError `json:"error"`
	}
	_ = common.UnmarshalJsonStr(str, &chunk)
	return types.WithOpenAIError(chunk.Error, upstreamStatus(chunk.Error.Code))
}

// RecordChatChunkFailure 记录 Chat 流里的错误块；Claude 格式转换会丢掉原始错误字段，要在转换前检查。
func RecordChatChunkFailure(c *gin.Context, data string) {
	if chatChunkKind(data) == chunkFailure {
		RecordStreamFailure(c, chatChunkFailure(data))
	}
}

func ClaudeData(c *gin.Context, resp dto.ClaudeResponse) error {
	if requestContextDone(c) {
		return nil
	}
	return emit(c,
		func() chunkKind { return claudeChunkKind(resp.Type) },
		func() *types.NewAPIError { return claudeStreamFailure(resp.GetClaudeError()) },
		func() error {
			MarkOutputSent(c)

			jsonData, err := common.Marshal(resp)
			if err != nil {
				common.SysError(common.LogText("error marshalling stream response: %s", err.Error()))
			} else {
				c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("event: %s\n", resp.Type)})
				c.Render(-1, common.CustomEvent{Data: "data: " + string(jsonData)})
			}
			_ = FlushWriter(c)
			return nil
		})
}

const claudeSignatureBufferKey = "claude_signature_buffer"

func writeClaudeEvent(c *gin.Context, eventType string, data string) {
	_ = emit(c,
		func() chunkKind { return claudeChunkKind(eventType) },
		func() *types.NewAPIError {
			return claudeStreamFailure(&types.ClaudeError{
				Type:    ojson.Get(data, "error.type").String(),
				Message: ojson.Get(data, "error.message").String(),
			})
		},
		func() error {
			MarkOutputSent(c)
			tagged := string(service.TagClaudeThinkingEvent([]byte(data), common.GetContextKeyInt(c, constant.ContextKeyChannelId)))
			c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("event: %s\n", eventType)})
			c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("data: %s\n", tagged)})
			_ = FlushWriter(c)
			return nil
		})
}

func ClaudeChunkData(c *gin.Context, resp dto.ClaudeResponse, data string) {
	if requestContextDone(c) {
		return
	}

	// 上游可能把同一块的签名拆成多条 signature_delta，客户端只认完整签名，故按 index 暂存、在块结束时合并成一条。
	switch resp.Type {
	case "message_start":
		c.Set(claudeSignatureBufferKey, nil)
	case "content_block_delta":
		if ojson.Get(data, "delta.type").String() == "signature_delta" {
			buffer, _ := c.Get(claudeSignatureBufferKey)
			signatures, _ := buffer.(map[int64]string)
			if signatures == nil {
				signatures = map[int64]string{}
				c.Set(claudeSignatureBufferKey, signatures)
			}
			index := ojson.Get(data, "index").Int()
			signatures[index] += ojson.Get(data, "delta.signature").String()
			return
		}
	case "content_block_stop":
		buffer, _ := c.Get(claudeSignatureBufferKey)
		signatures, _ := buffer.(map[int64]string)
		index := ojson.Get(data, "index").Int()
		if signature, ok := signatures[index]; ok {
			merged, _ := ojson.Set(`{"type":"content_block_delta","delta":{"type":"signature_delta"}}`, "delta.signature", signature)
			merged, _ = ojson.Set(merged, "index", index)
			writeClaudeEvent(c, "content_block_delta", merged)
			delete(signatures, index)
		}
	}
	writeClaudeEvent(c, resp.Type, data)
}

func ResponseChunkData(c *gin.Context, resp dto.ResponsesStreamResponse, data string) error {
	if requestContextDone(c) {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	return emit(c,
		func() chunkKind { return responsesChunkKind(&resp) },
		func() *types.NewAPIError {
			full := resp
			if resp.Response == nil {
				_ = common.UnmarshalJsonStr(data, &full)
			}
			return responsesStreamFailure(&full)
		},
		func() error {
			MarkOutputSent(c)
			event := common.CustomEvent{Data: fmt.Sprintf("event: %s\n", resp.Type)}
			if err := event.Render(c.Writer); err != nil {
				return err
			}
			tagged := string(service.TagResponsesItemEvent([]byte(data), common.GetContextKeyInt(c, constant.ContextKeyChannelId)))
			event.Data = fmt.Sprintf("data: %s", tagged)
			if err := event.Render(c.Writer); err != nil {
				return err
			}
			return FlushWriter(c)
		})
}

func StringData(c *gin.Context, str string) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}

	if requestContextDone(c) {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	return emit(c,
		func() chunkKind { return chatChunkKind(str) },
		func() *types.NewAPIError { return chatChunkFailure(str) },
		func() error {
			MarkOutputSent(c)
			c.Render(-1, common.CustomEvent{Data: "data: " + str})
			return FlushWriter(c)
		})
}

func PingData(c *gin.Context) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}

	if requestContextDone(c) {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	if _, err := c.Writer.Write([]byte(": PING\n\n")); err != nil {
		return fmt.Errorf("write ping data failed: %w", err)
	}
	return FlushWriter(c)
}

func ObjectData(c *gin.Context, object any) error {
	if object == nil {
		return errors.New("object is nil")
	}
	jsonData, err := common.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}
	return StringData(c, string(jsonData))
}

func Done(c *gin.Context) {
	_ = StringData(c, "[DONE]")
}

func WssString(c *gin.Context, ws *websocket.Conn, str string) error {
	if ws == nil {
		logger.LogError(c, common.LogText("websocket connection is nil"))
		return errors.New("websocket connection is nil")
	}
	//common.LogInfo(c, fmt.Sprintf("sending message: %s", str))
	return ws.WriteMessage(1, []byte(str))
}

func WssObject(c *gin.Context, ws *websocket.Conn, object any) error {
	jsonData, err := common.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}
	if ws == nil {
		logger.LogError(c, common.LogText("websocket connection is nil"))
		return errors.New("websocket connection is nil")
	}
	//common.LogInfo(c, fmt.Sprintf("sending message: %s", jsonData))
	return ws.WriteMessage(1, jsonData)
}

func WssError(c *gin.Context, ws *websocket.Conn, openaiError types.OpenAIError) {
	if ws == nil {
		return
	}
	errorObj := &dto.RealtimeEvent{
		Type:    "error",
		EventId: GetLocalRealtimeID(c),
		Error:   &openaiError,
	}
	_ = WssObject(c, ws, errorObj)
}

func GetResponseID(c *gin.Context) string {
	logID := c.GetString(common.RequestIdKey)
	return fmt.Sprintf("chatcmpl-%s", logID)
}

func GetLocalRealtimeID(c *gin.Context) string {
	logID := c.GetString(common.RequestIdKey)
	return fmt.Sprintf("evt_%s", logID)
}

func GenerateStartEmptyResponse(id string, createAt int64, model string, systemFingerprint *string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: systemFingerprint,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:    "assistant",
					Content: common.GetPointer(""),
				},
			},
		},
	}
}

func GenerateStopResponse(id string, createAt int64, model string, finishReason string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: nil,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				FinishReason: &finishReason,
			},
		},
	}
}

func GenerateFinalUsageResponse(id string, createAt int64, model string, usage dto.Usage) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: nil,
		Choices:           make([]dto.ChatCompletionsStreamResponseChoice, 0),
		Usage:             &usage,
	}
}

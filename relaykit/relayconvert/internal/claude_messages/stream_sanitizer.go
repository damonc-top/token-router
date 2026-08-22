package claudemessages

import (
	"github.com/QuantumNous/new-api/relaykit/dto"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/tidwall/sjson"
)

// ClaudeStreamSanitizer repairs Anthropic-incompatible Claude SSE sequences
// emitted by some upstreams (notably MiniMax Messages). Valid streams are
// returned unchanged. The MiniMax failure mode reuses an in-use index for a
// new block type, then emits input_json_delta against the now-active text
// block ("Content block is not a input_json block").
type ClaudeStreamSanitizer struct {
	open []claudeStreamBlock
	next int
}

type claudeStreamBlock struct {
	assigned int
	typ      string
}

type ClaudeSanitizedEvent struct {
	Response dto.ClaudeResponse
	Data     string
}

func NewClaudeStreamSanitizer() *ClaudeStreamSanitizer {
	return &ClaudeStreamSanitizer{}
}

func (info *ClaudeResponseInfo) SanitizeStreamEvent(event *dto.ClaudeResponse, data string) []ClaudeSanitizedEvent {
	if event == nil {
		return nil
	}
	if info == nil {
		return []ClaudeSanitizedEvent{{Response: *event, Data: data}}
	}
	if info.StreamSanitizer == nil {
		info.StreamSanitizer = NewClaudeStreamSanitizer()
	}
	return info.StreamSanitizer.Process(event, data)
}

func (info *ClaudeResponseInfo) CloseSanitizedStream() []ClaudeSanitizedEvent {
	if info == nil || info.StreamSanitizer == nil {
		return nil
	}
	return info.StreamSanitizer.CloseRemaining()
}

func (s *ClaudeStreamSanitizer) Process(event *dto.ClaudeResponse, data string) []ClaudeSanitizedEvent {
	if event == nil {
		return nil
	}
	if s == nil {
		return []ClaudeSanitizedEvent{{Response: *event, Data: data}}
	}
	switch event.Type {
	case "content_block_start":
		return s.handleStart(event, data)
	case "content_block_delta":
		return s.handleDelta(event, data)
	case "content_block_stop":
		return s.handleStop(event, data)
	case "message_delta", "message_stop":
		out := s.CloseRemaining()
		return append(out, ClaudeSanitizedEvent{Response: *event, Data: data})
	default:
		return []ClaudeSanitizedEvent{{Response: *event, Data: data}}
	}
}

func (s *ClaudeStreamSanitizer) CloseRemaining() []ClaudeSanitizedEvent {
	if s == nil || len(s.open) == 0 {
		return nil
	}
	out := make([]ClaudeSanitizedEvent, 0, len(s.open))
	for i := len(s.open) - 1; i >= 0; i-- {
		out = append(out, makeClaudeStopEvent(s.open[i].assigned))
	}
	s.open = nil
	return out
}

func (s *ClaudeStreamSanitizer) handleStart(event *dto.ClaudeResponse, data string) []ClaudeSanitizedEvent {
	typ := ""
	if event.ContentBlock != nil {
		typ = event.ContentBlock.Type
	}
	if typ == "" {
		return []ClaudeSanitizedEvent{{Response: *event, Data: data}}
	}

	req := event.GetIndex()
	assigned := req
	var out []ClaudeSanitizedEvent
	if i := s.findAssigned(req); i >= 0 {
		existing := s.open[i]
		switch {
		case existing.typ == typ && !isClaudeToolBlockType(typ):
			return nil
		case isClaudeToolBlockType(existing.typ):
			assigned = s.alloc()
		default:
			out = append(out, s.removeAt(i))
			assigned = req
		}
	}
	if assigned >= s.next {
		s.next = assigned + 1
	}
	s.open = append(s.open, claudeStreamBlock{assigned: assigned, typ: typ})
	return append(out, remapClaudeStreamIndex(event, data, assigned))
}

func (s *ClaudeStreamSanitizer) handleDelta(event *dto.ClaudeResponse, data string) []ClaudeSanitizedEvent {
	deltaType := ""
	if event.Delta != nil {
		deltaType = event.Delta.Type
	}
	targetType := claudeDeltaTargetBlockType(deltaType)
	if targetType == "" {
		if len(s.open) == 0 {
			return []ClaudeSanitizedEvent{{Response: *event, Data: data}}
		}
		return []ClaudeSanitizedEvent{remapClaudeStreamIndex(event, data, s.open[len(s.open)-1].assigned)}
	}

	if idx, ok := s.blockAt(event.GetIndex(), targetType); ok {
		return []ClaudeSanitizedEvent{remapClaudeStreamIndex(event, data, idx)}
	}
	if idx, ok := s.latestOpen(targetType); ok {
		return []ClaudeSanitizedEvent{remapClaudeStreamIndex(event, data, idx)}
	}
	if isClaudeToolBlockType(targetType) {
		return nil
	}

	out := s.ensureOpen(targetType)
	idx, ok := s.latestOpen(targetType)
	if !ok {
		return out
	}
	return append(out, remapClaudeStreamIndex(event, data, idx))
}

func (s *ClaudeStreamSanitizer) handleStop(event *dto.ClaudeResponse, data string) []ClaudeSanitizedEvent {
	if i := s.findAssigned(event.GetIndex()); i >= 0 {
		assigned := s.open[i].assigned
		s.open = append(s.open[:i], s.open[i+1:]...)
		return []ClaudeSanitizedEvent{remapClaudeStreamIndex(event, data, assigned)}
	}
	if len(s.open) == 0 {
		return nil
	}
	last := s.open[len(s.open)-1]
	s.open = s.open[:len(s.open)-1]
	return []ClaudeSanitizedEvent{remapClaudeStreamIndex(event, data, last.assigned)}
}

func (s *ClaudeStreamSanitizer) removeAt(i int) ClaudeSanitizedEvent {
	ev := makeClaudeStopEvent(s.open[i].assigned)
	s.open = append(s.open[:i], s.open[i+1:]...)
	return ev
}

func (s *ClaudeStreamSanitizer) findAssigned(idx int) int {
	for i := range s.open {
		if s.open[i].assigned == idx {
			return i
		}
	}
	return -1
}

func (s *ClaudeStreamSanitizer) blockAt(idx int, typ string) (int, bool) {
	i := s.findAssigned(idx)
	if i < 0 {
		return 0, false
	}
	if s.open[i].typ == typ || (isClaudeToolBlockType(typ) && isClaudeToolBlockType(s.open[i].typ)) {
		return s.open[i].assigned, true
	}
	return 0, false
}

func (s *ClaudeStreamSanitizer) latestOpen(typ string) (int, bool) {
	for i := len(s.open) - 1; i >= 0; i-- {
		if s.open[i].typ == typ || (isClaudeToolBlockType(typ) && isClaudeToolBlockType(s.open[i].typ)) {
			return s.open[i].assigned, true
		}
	}
	return 0, false
}

func (s *ClaudeStreamSanitizer) alloc() int {
	for {
		if s.findAssigned(s.next) < 0 {
			idx := s.next
			s.next++
			return idx
		}
		s.next++
	}
}

func (s *ClaudeStreamSanitizer) ensureOpen(typ string) []ClaudeSanitizedEvent {
	if _, ok := s.latestOpen(typ); ok {
		return nil
	}
	var out []ClaudeSanitizedEvent
	if len(s.open) > 0 {
		last := s.open[len(s.open)-1]
		if !isClaudeToolBlockType(last.typ) && last.typ != typ {
			out = append(out, s.removeAt(len(s.open)-1))
		}
	}
	assigned := s.alloc()
	s.open = append(s.open, claudeStreamBlock{assigned: assigned, typ: typ})
	start := dto.ClaudeResponse{
		Type:  "content_block_start",
		Index: kitutil.GetPointer(assigned),
		ContentBlock: &dto.ClaudeMediaMessage{
			Type: typ,
		},
	}
	if typ == "text" {
		start.ContentBlock.Text = kitutil.GetPointer("")
	}
	if typ == "thinking" {
		start.ContentBlock.Thinking = kitutil.GetPointer("")
	}
	body, err := kitutil.Marshal(start)
	if err != nil {
		return out
	}
	return append(out, ClaudeSanitizedEvent{Response: start, Data: string(body)})
}

func isClaudeToolBlockType(typ string) bool {
	return typ == "tool_use" || typ == "server_tool_use"
}

func claudeDeltaTargetBlockType(deltaType string) string {
	switch deltaType {
	case "input_json_delta":
		return "tool_use"
	case "text_delta", "citations_delta":
		return "text"
	case "thinking_delta", "signature_delta":
		return "thinking"
	default:
		return ""
	}
}

func remapClaudeStreamIndex(event *dto.ClaudeResponse, data string, index int) ClaudeSanitizedEvent {
	cloned := *event
	cloned.SetIndex(index)
	if event.GetIndex() == index {
		return ClaudeSanitizedEvent{Response: cloned, Data: data}
	}
	patched, err := sjson.Set(data, "index", index)
	if err != nil {
		body, mErr := kitutil.Marshal(cloned)
		if mErr != nil {
			return ClaudeSanitizedEvent{Response: cloned, Data: data}
		}
		return ClaudeSanitizedEvent{Response: cloned, Data: string(body)}
	}
	return ClaudeSanitizedEvent{Response: cloned, Data: patched}
}

func makeClaudeStopEvent(index int) ClaudeSanitizedEvent {
	resp := dto.ClaudeResponse{
		Type:  "content_block_stop",
		Index: kitutil.GetPointer(index),
	}
	body, err := kitutil.Marshal(resp)
	if err != nil {
		return ClaudeSanitizedEvent{Response: resp, Data: ""}
	}
	return ClaudeSanitizedEvent{Response: resp, Data: string(body)}
}

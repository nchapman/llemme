package components

import "testing"

func TestMessages_AddMessage(t *testing.T) {
	m := NewMessages()
	m.SetSize(80, 24) // Need size set for refresh to work

	if len(m.MessagesList()) != 0 {
		t.Errorf("expected 0 messages initially, got %d", len(m.MessagesList()))
	}

	m.AddMessage(Message{Role: RoleUser, Content: "Hello"})
	if len(m.MessagesList()) != 1 {
		t.Errorf("expected 1 message, got %d", len(m.MessagesList()))
	}

	m.AddMessage(Message{Role: RoleAssistant, Content: "Hi there"})
	if len(m.MessagesList()) != 2 {
		t.Errorf("expected 2 messages, got %d", len(m.MessagesList()))
	}

	// Verify message content
	msgs := m.MessagesList()
	if msgs[0].Role != RoleUser || msgs[0].Content != "Hello" {
		t.Errorf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != RoleAssistant || msgs[1].Content != "Hi there" {
		t.Errorf("unexpected second message: %+v", msgs[1])
	}
}

func TestMessages_ClearMessages(t *testing.T) {
	m := NewMessages()
	m.SetSize(80, 24)

	m.AddMessage(Message{Role: RoleUser, Content: "Hello"})
	m.AddMessage(Message{Role: RoleAssistant, Content: "Hi"})

	if len(m.MessagesList()) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m.MessagesList()))
	}

	m.ClearMessages()

	if len(m.MessagesList()) != 0 {
		t.Errorf("expected 0 messages after clear, got %d", len(m.MessagesList()))
	}
}

func TestMessages_StreamingState(t *testing.T) {
	m := NewMessages()
	m.SetSize(80, 24)

	// Initially not streaming
	if m.IsStreaming() {
		t.Error("expected not streaming initially")
	}

	// Start streaming
	m.StartStreaming()
	if !m.IsStreaming() {
		t.Error("expected streaming after StartStreaming")
	}

	// Append content
	m.AppendStreamContent("Hello ")
	m.AppendStreamContent("world")
	if !m.IsStreaming() {
		t.Error("expected still streaming after append")
	}

	// Message count should still be 0 (streaming not finished)
	if len(m.MessagesList()) != 0 {
		t.Errorf("expected 0 messages while streaming, got %d", len(m.MessagesList()))
	}

	// Finish streaming
	m.FinishStreaming()
	if m.IsStreaming() {
		t.Error("expected not streaming after FinishStreaming")
	}

	// Now message should be added
	if len(m.MessagesList()) != 1 {
		t.Errorf("expected 1 message after finish, got %d", len(m.MessagesList()))
	}

	msg := m.MessagesList()[0]
	if msg.Role != RoleAssistant {
		t.Errorf("expected RoleAssistant, got %s", msg.Role)
	}
	if msg.Content != "Hello world" {
		t.Errorf("expected 'Hello world', got '%s'", msg.Content)
	}
}

func TestMessages_StreamingWithThinking(t *testing.T) {
	m := NewMessages()
	m.SetSize(80, 24)

	m.StartStreaming()
	m.AppendStreamThinking("Let me think...")
	m.AppendStreamThinking(" about this.")
	m.AppendStreamContent("Here's my answer.")
	m.FinishStreaming()

	if len(m.MessagesList()) != 1 {
		t.Fatalf("expected 1 message, got %d", len(m.MessagesList()))
	}

	msg := m.MessagesList()[0]
	if msg.Thinking != "Let me think... about this." {
		t.Errorf("unexpected thinking: '%s'", msg.Thinking)
	}
	if msg.Content != "Here's my answer." {
		t.Errorf("unexpected content: '%s'", msg.Content)
	}
}

func TestMessages_CancelStreaming(t *testing.T) {
	m := NewMessages()
	m.SetSize(80, 24)

	m.StartStreaming()
	m.AppendStreamContent("Partial content...")
	m.CancelStreaming()

	if m.IsStreaming() {
		t.Error("expected not streaming after cancel")
	}

	// Cancelled stream should not add message
	if len(m.MessagesList()) != 0 {
		t.Errorf("expected 0 messages after cancel, got %d", len(m.MessagesList()))
	}
}

func TestMessages_FinishStreamingIdempotent(t *testing.T) {
	m := NewMessages()
	m.SetSize(80, 24)

	m.StartStreaming()
	m.AppendStreamContent("Content")
	m.FinishStreaming()

	// Second finish should be no-op
	m.FinishStreaming()

	if len(m.MessagesList()) != 1 {
		t.Errorf("expected 1 message after double finish, got %d", len(m.MessagesList()))
	}
}

func TestMessages_GetSize(t *testing.T) {
	m := NewMessages()

	w, h := m.GetSize()
	if w != 0 || h != 0 {
		t.Errorf("expected initial size 0x0, got %dx%d", w, h)
	}

	m.SetSize(80, 24)
	w, h = m.GetSize()
	if w != 80 || h != 24 {
		t.Errorf("expected size 80x24, got %dx%d", w, h)
	}
}

func TestMessages_MessageRoles(t *testing.T) {
	m := NewMessages()
	m.SetSize(80, 24)

	m.AddMessage(Message{Role: RoleUser, Content: "User msg"})
	m.AddMessage(Message{Role: RoleAssistant, Content: "Assistant msg"})
	m.AddMessage(Message{Role: RoleSystem, Content: "System msg"})
	m.AddMessage(Message{Role: RoleError, Content: "Error msg"})

	msgs := m.MessagesList()
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}

	expectedRoles := []MessageRole{RoleUser, RoleAssistant, RoleSystem, RoleError}
	for i, expected := range expectedRoles {
		if msgs[i].Role != expected {
			t.Errorf("message %d: expected role %s, got %s", i, expected, msgs[i].Role)
		}
	}
}

func TestMessages_StreamingEmptyContent(t *testing.T) {
	m := NewMessages()
	m.SetSize(80, 24)

	m.StartStreaming()
	// Don't append anything
	m.FinishStreaming()

	// Should still add empty message
	if len(m.MessagesList()) != 1 {
		t.Errorf("expected 1 message even with empty content, got %d", len(m.MessagesList()))
	}

	msg := m.MessagesList()[0]
	if msg.Content != "" {
		t.Errorf("expected empty content, got '%s'", msg.Content)
	}
}

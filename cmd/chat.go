package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/server"
	"github.com/nchapman/lleme/internal/ui"
)

// ChatSession manages an interactive chat session with a model.
type ChatSession struct {
	api      *server.APIClient
	model    string
	cfg      *config.Config
	persona  *config.Persona
	messages []server.ChatMessage
	reader   *bufio.Reader

	// Session options (override config/persona)
	options sessionOptions

	// Track if server options changed (need reload)
	pendingReload bool
}

// sessionOptions holds runtime-adjustable options for the chat session.
type sessionOptions struct {
	// Request-time options (no restart needed)
	temp          float64
	topP          float64
	topK          int
	repeatPenalty float64
	minP          float64

	// Server options (require model reload)
	ctxSize   int
	gpuLayers int
	threads   int

	// Track explicitly set server options (allows setting to 0)
	ctxSizeSet   bool
	gpuLayersSet bool
	threadsSet   bool
}

// NewChatSession creates a new chat session.
func NewChatSession(api *server.APIClient, model string, cfg *config.Config, persona *config.Persona) *ChatSession {
	return &ChatSession{
		api:      api,
		model:    model,
		cfg:      cfg,
		persona:  persona,
		messages: []server.ChatMessage{},
		reader:   bufio.NewReader(os.Stdin),
	}
}

// SetInitialServerOptions sets the initial server options from CLI flags.
// These are used as the baseline for /reload.
func (s *ChatSession) SetInitialServerOptions(ctxSize, gpuLayers, threads int, ctxSizeSet, gpuLayersSet, threadsSet bool) {
	s.options.ctxSize = ctxSize
	s.options.gpuLayers = gpuLayers
	s.options.threads = threads
	s.options.ctxSizeSet = ctxSizeSet
	s.options.gpuLayersSet = gpuLayersSet
	s.options.threadsSet = threadsSet
}

// Run starts the interactive chat session.
func (s *ChatSession) Run(initialPrompt string) {
	// Check if input is piped (non-interactive)
	stat, _ := os.Stdin.Stat()
	isPiped := (stat.Mode() & os.ModeCharDevice) == 0

	// Initialize with system prompt
	s.initSystemPrompt()

	if !isPiped {
		fmt.Printf("\n%s  %s\n\n", ui.Box(s.model), ui.Muted("Type /? for help"))
	}

	// Handle initial prompt if provided
	if initialPrompt != "" {
		if !isPiped {
			fmt.Printf("%s %s\n", ui.Muted("You:"), initialPrompt)
		}
		s.messages = append(s.messages, server.ChatMessage{Role: "user", Content: initialPrompt})
		response := s.streamResponse(!isPiped)
		if response != "" {
			s.messages = append(s.messages, server.ChatMessage{Role: "assistant", Content: response})
		}
		if isPiped {
			return // Exit after one-shot when piped
		}
		fmt.Println()
	}

	// Interactive loop
	if isPiped {
		return
	}

	for {
		fmt.Print(ui.Muted("You: "))
		input, err := s.reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			if s.handleCommand(input) {
				continue
			}
			// Command indicated exit
			return
		}

		s.messages = append(s.messages, server.ChatMessage{Role: "user", Content: input})
		response := s.streamResponse(true)
		if response != "" {
			s.messages = append(s.messages, server.ChatMessage{Role: "assistant", Content: response})
		}
		fmt.Println()
	}
}

// initSystemPrompt sets up the initial system message.
func (s *ChatSession) initSystemPrompt() {
	sysPrompt := systemPrompt
	if sysPrompt == "" && s.persona != nil && s.persona.System != "" {
		sysPrompt = s.persona.System
	}
	if sysPrompt == "" {
		sysPrompt = config.DefaultSystemPrompt()
	}
	s.messages = []server.ChatMessage{{Role: "system", Content: sysPrompt}}
}

// handleCommand processes a slash command. Returns true to continue, false to exit.
func (s *ChatSession) handleCommand(input string) bool {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return true
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "/help", "/?":
		s.showHelp()

	case "/bye", "/exit", "/quit":
		fmt.Println(ui.Muted("Goodbye!"))
		return false

	case "/clear":
		s.initSystemPrompt()
		fmt.Println(ui.Muted("Conversation cleared"))

	case "/system":
		if len(args) == 0 {
			// Show current system prompt
			if len(s.messages) > 0 && s.messages[0].Role == "system" {
				fmt.Printf("%s\n%s\n", ui.Bold("System prompt:"), s.messages[0].Content)
			}
		} else {
			// Set new system prompt
			newPrompt := strings.Join(args, " ")
			s.messages = []server.ChatMessage{{Role: "system", Content: newPrompt}}
			fmt.Println(ui.Muted("System prompt updated, conversation cleared"))
		}

	case "/set":
		if len(args) < 2 {
			fmt.Println(ui.Muted("Usage: /set <option> <value>"))
			fmt.Println(ui.Muted("Options: temp, top-p, top-k, repeat-penalty, min-p, ctx-size, gpu-layers, threads"))
			return true
		}
		s.handleSet(args[0], args[1])

	case "/reload":
		s.handleReload()

	case "/show":
		s.showSettings()

	default:
		fmt.Printf("%s Unknown command: %s (type /? for help)\n", ui.ErrorMsg("Error:"), cmd)
	}

	return true
}

// handleSet processes the /set command.
func (s *ChatSession) handleSet(option, value string) {
	option = strings.ToLower(option)

	// Parse value
	floatVal, floatErr := strconv.ParseFloat(value, 64)
	intVal, intErr := strconv.Atoi(value)

	switch option {
	case "temp", "temperature":
		if floatErr != nil {
			fmt.Printf("%s Invalid value for temp: %s\n", ui.ErrorMsg("Error:"), value)
			return
		}
		s.options.temp = floatVal
		fmt.Printf("%s temp = %g\n", ui.Success("Set"), floatVal)

	case "top-p":
		if floatErr != nil {
			fmt.Printf("%s Invalid value for top-p: %s\n", ui.ErrorMsg("Error:"), value)
			return
		}
		s.options.topP = floatVal
		fmt.Printf("%s top-p = %g\n", ui.Success("Set"), floatVal)

	case "top-k":
		if intErr != nil {
			fmt.Printf("%s Invalid value for top-k: %s\n", ui.ErrorMsg("Error:"), value)
			return
		}
		s.options.topK = intVal
		fmt.Printf("%s top-k = %d\n", ui.Success("Set"), intVal)

	case "repeat-penalty":
		if floatErr != nil {
			fmt.Printf("%s Invalid value for repeat-penalty: %s\n", ui.ErrorMsg("Error:"), value)
			return
		}
		s.options.repeatPenalty = floatVal
		fmt.Printf("%s repeat-penalty = %g\n", ui.Success("Set"), floatVal)

	case "min-p":
		if floatErr != nil {
			fmt.Printf("%s Invalid value for min-p: %s\n", ui.ErrorMsg("Error:"), value)
			return
		}
		s.options.minP = floatVal
		fmt.Printf("%s min-p = %g\n", ui.Success("Set"), floatVal)

	case "ctx-size":
		if intErr != nil {
			fmt.Printf("%s Invalid value for ctx-size: %s\n", ui.ErrorMsg("Error:"), value)
			return
		}
		s.options.ctxSize = intVal
		s.options.ctxSizeSet = true
		s.pendingReload = true
		fmt.Printf("%s ctx-size = %d %s\n", ui.Success("Set"), intVal, ui.Muted("(use /reload to apply)"))

	case "gpu-layers":
		if intErr != nil {
			fmt.Printf("%s Invalid value for gpu-layers: %s\n", ui.ErrorMsg("Error:"), value)
			return
		}
		s.options.gpuLayers = intVal
		s.options.gpuLayersSet = true
		s.pendingReload = true
		fmt.Printf("%s gpu-layers = %d %s\n", ui.Success("Set"), intVal, ui.Muted("(use /reload to apply)"))

	case "threads":
		if intErr != nil {
			fmt.Printf("%s Invalid value for threads: %s\n", ui.ErrorMsg("Error:"), value)
			return
		}
		s.options.threads = intVal
		s.options.threadsSet = true
		s.pendingReload = true
		fmt.Printf("%s threads = %d %s\n", ui.Success("Set"), intVal, ui.Muted("(use /reload to apply)"))

	default:
		fmt.Printf("%s Unknown option: %s\n", ui.ErrorMsg("Error:"), option)
		fmt.Println(ui.Muted("Options: temp, top-p, top-k, repeat-penalty, min-p, ctx-size, gpu-layers, threads"))
	}
}

// handleReload unloads and reloads the model to apply server option changes.
func (s *ChatSession) handleReload() {
	if !s.pendingReload {
		fmt.Println(ui.Muted("No pending server option changes to apply"))
		return
	}

	fmt.Println(ui.Muted("Reloading model..."))

	// Stop the current model
	if err := s.api.StopModel(s.model); err != nil {
		fmt.Printf("%s Failed to stop model: %v\n", ui.ErrorMsg("Error:"), err)
		return
	}

	// Reload with persona options as base, session options override
	opts := &server.RunOptions{}
	if s.persona != nil {
		opts.Options = s.persona.GetServerOptions()
	}
	// Session options override persona options (use Set flags to allow zero values)
	if s.options.ctxSizeSet {
		opts.CtxSize = server.IntPtr(s.options.ctxSize)
	}
	if s.options.gpuLayersSet {
		opts.GpuLayers = server.IntPtr(s.options.gpuLayers)
	}
	if s.options.threadsSet {
		opts.Threads = server.IntPtr(s.options.threads)
	}
	if err := s.api.Run(s.model, opts); err != nil {
		fmt.Printf("%s Failed to reload model: %v\n", ui.ErrorMsg("Error:"), err)
		return
	}

	s.pendingReload = false
	fmt.Printf("%s Model reloaded\n", ui.Success("✓"))
}

// showHelp displays available commands.
func (s *ChatSession) showHelp() {
	fmt.Println(ui.Header("Commands"))
	fmt.Println()
	fmt.Println("  /help, /?              Show this help")
	fmt.Println("  /clear                 Clear conversation history")
	fmt.Println("  /system [prompt]       Show or set system prompt")
	fmt.Println("  /set <option> <value>  Change a setting")
	fmt.Println("  /show                  Show current settings")
	fmt.Println("  /reload                Reload model (apply server options)")
	fmt.Println("  /bye, /exit, /quit     Exit chat")
	fmt.Println()
	fmt.Println(ui.Bold("Options for /set:"))
	fmt.Println("  temp, top-p, top-k, repeat-penalty, min-p")
	fmt.Println("  ctx-size*, gpu-layers*, threads*  " + ui.Muted("(* require /reload)"))
	fmt.Println()
}

// showSettings displays current session settings.
func (s *ChatSession) showSettings() {
	fmt.Println(ui.Header("Current Settings"))
	fmt.Println()
	fmt.Printf("  %s %s\n", ui.Bold("Model:"), s.model)
	fmt.Println()

	// Show system prompt (truncated if long)
	if len(s.messages) > 0 && s.messages[0].Role == "system" {
		prompt := s.messages[0].Content
		if len(prompt) > 80 {
			prompt = prompt[:77] + "..."
		}
		fmt.Printf("  %s %s\n", ui.Bold("System:"), prompt)
	}
	fmt.Println()

	// Request-time options
	fmt.Println(ui.Bold("  Sampling:"))
	s.showOption("temp", s.options.temp, s.getConfigFloat("temp"))
	s.showOption("top-p", s.options.topP, s.getConfigFloat("top-p"))
	s.showOptionInt("top-k", s.options.topK, s.getConfigInt("top-k"))
	s.showOption("repeat-penalty", s.options.repeatPenalty, s.getConfigFloat("repeat-penalty"))
	s.showOption("min-p", s.options.minP, s.getConfigFloat("min-p"))
	fmt.Println()

	// Server options (use Set flags to correctly show zero values)
	fmt.Println(ui.Bold("  Server:"))
	s.showServerOption("ctx-size", s.options.ctxSize, s.options.ctxSizeSet, s.getConfigInt("ctx-size"))
	s.showServerOption("gpu-layers", s.options.gpuLayers, s.options.gpuLayersSet, s.getConfigInt("gpu-layers"))
	s.showServerOption("threads", s.options.threads, s.options.threadsSet, s.getConfigInt("threads"))
	fmt.Println()
}

func (s *ChatSession) showOption(name string, sessionVal, configVal float64) {
	if sessionVal != 0 {
		fmt.Printf("    %s = %g %s\n", name, sessionVal, ui.Muted("(session)"))
	} else if configVal != 0 {
		fmt.Printf("    %s = %g %s\n", name, configVal, ui.Muted("(config)"))
	} else {
		fmt.Printf("    %s = %s\n", name, ui.Muted("default"))
	}
}

func (s *ChatSession) showOptionInt(name string, sessionVal, configVal int) {
	if sessionVal != 0 {
		fmt.Printf("    %s = %d %s\n", name, sessionVal, ui.Muted("(session)"))
	} else if configVal != 0 {
		fmt.Printf("    %s = %d %s\n", name, configVal, ui.Muted("(config)"))
	} else {
		fmt.Printf("    %s = %s\n", name, ui.Muted("default"))
	}
}

// showServerOption displays a server option, using isSet to correctly show zero values.
func (s *ChatSession) showServerOption(name string, sessionVal int, isSet bool, configVal int) {
	if isSet {
		fmt.Printf("    %s = %d %s\n", name, sessionVal, ui.Muted("(session)"))
	} else if configVal != 0 {
		fmt.Printf("    %s = %d %s\n", name, configVal, ui.Muted("(config)"))
	} else {
		fmt.Printf("    %s = %s\n", name, ui.Muted("default"))
	}
}

func (s *ChatSession) getConfigFloat(key string) float64 {
	if s.persona != nil {
		if v := s.persona.GetFloatOption(key, 0); v != 0 {
			return v
		}
	}
	return s.cfg.LlamaCpp.GetFloatOption(key, 0)
}

func (s *ChatSession) getConfigInt(key string) int {
	if s.persona != nil {
		if val, ok := s.persona.Options[key]; ok {
			switch v := val.(type) {
			case int:
				return v
			case float64:
				return int(v)
			}
		}
	}
	return s.cfg.LlamaCpp.GetIntOption(key, 0)
}

// streamResponse sends a chat completion request and streams the response.
func (s *ChatSession) streamResponse(showSpinner bool) string {
	req := &server.ChatCompletionRequest{
		Model:           s.model,
		Messages:        s.messages,
		Stream:          true,
		MaxTokens:       tokens,
		ReasoningFormat: "auto",
	}

	// Apply options: session > persona > config > default
	req.Temperature = s.resolveFloat(s.options.temp, "temp")
	req.TopP = s.resolveFloat(s.options.topP, "top-p")
	req.TopK = s.resolveInt(s.options.topK, "top-k")
	req.MinP = s.resolveFloat(s.options.minP, "min-p")
	req.RepeatPenalty = s.resolveFloat(s.options.repeatPenalty, "repeat-penalty")

	var spinner *ui.Spinner
	spinnerRunning := false
	if showSpinner {
		spinner = ui.NewSpinner()
		spinner.Start("")
		spinnerRunning = true
	}

	var fullResponse strings.Builder
	hadReasoning := false
	inReasoning := false

	stopSpinner := func() {
		if spinnerRunning {
			spinner.Stop(true, "")
			spinnerRunning = false
		}
	}

	cb := server.StreamCallback{
		ReasoningCallback: func(reasoning string) {
			stopSpinner()
			inReasoning = true
			hadReasoning = true
			fmt.Print(ui.Muted(reasoning))
		},
		ContentCallback: func(content string) {
			stopSpinner()
			if inReasoning {
				fmt.Print("\n\n")
				inReasoning = false
			}
			fullResponse.WriteString(content)
			fmt.Print(content)
		},
	}

	err := s.api.StreamChatCompletion(context.Background(), req, cb)

	if spinnerRunning {
		spinner.Stop(false, "")
	}

	if hadReasoning && fullResponse.Len() == 0 {
		fmt.Println()
	}

	if err != nil {
		fmt.Printf("\n%s %v\n", ui.ErrorMsg("Error:"), err)
		return ""
	}

	fmt.Println()
	return fullResponse.String()
}

// resolveFloat returns the first non-zero value from: session, persona, config.
func (s *ChatSession) resolveFloat(sessionVal float64, key string) float64 {
	if sessionVal != 0 {
		return sessionVal
	}
	if s.persona != nil {
		if v := s.persona.GetFloatOption(key, 0); v != 0 {
			return v
		}
	}
	return s.cfg.LlamaCpp.GetFloatOption(key, 0)
}

// resolveInt returns the first non-zero value from: session, persona, config.
func (s *ChatSession) resolveInt(sessionVal int, key string) int {
	if sessionVal != 0 {
		return sessionVal
	}
	return s.getConfigInt(key)
}

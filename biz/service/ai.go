package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"video/biz/model"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// AIBotUserID AI助手在系统中的用户ID
const AIBotUserID = "ai_assistant_fanai"

// AIService AI聊天服务，封装了与大模型的交互、Agent Loop和ToolCall
type AIService struct {
	client *openai.Client // OpenAI兼容客户端
	config model.AIConfig // AI配置
}

// NewAIService 创建AI服务实例
// cfg: AI配置（包含API Key、模型、BaseURL等）
func NewAIService(cfg model.AIConfig) *AIService {
	if cfg.APIKey == "" {
		log.Println("[AIService] Warning: AI_API_KEY is empty, AI chat will be disabled")
		return &AIService{config: cfg}
	}

	// 构建客户端配置
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	// 创建OpenAI兼容客户端（支持国内API如DeepSeek）
	client := openai.NewClient(opts...)
	return &AIService{
		client: &client,
		config: cfg,
	}
}

// IsEnabled 检查AI服务是否已启用
func (s *AIService) IsEnabled() bool {
	return s.config.APIKey != ""
}

// AIDecision AI决策结果：是否应该回复以及原因
type AIDecision struct {
	ShouldRespond bool   `json:"should_respond"` // 是否回复
	Reason        string `json:"reason,omitempty"` // 原因
}

// ToolDefinition ToolCall工具定义
type ToolDefinition struct {
	Name        string                    // 工具名称
	Description string                    // 工具描述
	Parameters  openai.FunctionParameters // 参数schema
	Handler     func(args map[string]interface{}) (string, error) // 处理函数
}

// GetToolDefinitions 获取所有可用的ToolCall工具列表
func (s *AIService) GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "get_user_info",
			Description: "获取用户的个人信息，包括用户名、头像等",
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "要查询的用户ID",
					},
				},
				"required": []string{"user_id"},
			},
			Handler: s.handleGetUserInfo,
		},
		{
			Name:        "get_chat_context",
			Description: "获取当前聊天的最近消息上下文，用于了解对话背景",
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"room_id": map[string]interface{}{
						"type":        "string",
						"description": "聊天房间ID",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "获取的消息数量，默认10",
					},
				},
				"required": []string{"room_id"},
			},
			Handler: s.handleGetChatContext,
		},
	}
}

// ShouldRespond 判断AI是否应该加入当前对话
// ctx: 上下文
// messages: 聊天上下文消息列表
// 返回: AIDecision决策结果和错误
func (s *AIService) ShouldRespond(ctx context.Context, messages []ChatContextMessage) (*AIDecision, error) {
	if !s.IsEnabled() {
		return &AIDecision{ShouldRespond: false}, nil
	}

	// 构建聊天历史字符串
	var chatHistory strings.Builder
	for _, msg := range messages {
		chatHistory.WriteString(fmt.Sprintf("[%s]: %s\n", msg.SenderName, msg.Content))
	}

	// 构建决策prompt，让AI判断是否回复
	prompt := fmt.Sprintf(`你是一个聊天助手，正在观察两个用户的对话。请判断你是否应该加入对话。

规则：
1. 如果有人直接@你或提到你的名字(FanAI)，你应该回复
2. 如果对话中有问题没有人回答，你可以回复
3. 如果对话很活跃且不需要你的参与，不要回复
4. 如果对话中有有趣的话题，你可以偶尔加入分享观点
5. 不要过于频繁地回复，大约30%%的对话你才需要加入

当前对话内容：
%s

请用JSON格式回复，包含should_respond(boolean)和reason(string)字段。`, chatHistory.String())

	// 调用大模型进行决策
	resp, err := s.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: s.config.Model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(s.config.SystemPrompt),
			openai.UserMessage(prompt),
		},
		MaxTokens:   openai.Int(int64(s.config.MaxTokens / 4)), // 不需要太长的token
		Temperature: openai.Float(0.3), // 较低的temperature，决策更稳定
	})
	if err != nil {
		log.Printf("[AIService] ShouldRespond error: %v", err)
		return &AIDecision{ShouldRespond: false}, err
	}

	// 解析AI决策结果
	content := resp.Choices[0].Message.Content
	var decision AIDecision
	if err := json.Unmarshal([]byte(extractJSON(content)), &decision); err != nil {
		log.Printf("[AIService] Failed to parse decision: %s, raw: %s", err, content)
		return &AIDecision{ShouldRespond: false}, nil
	}

	return &decision, nil
}

// GenerateResponse 生成AI回复内容
// ctx: 上下文
// messages: 聊天上下文
// triggerMessage: 触发消息内容
// 返回: 生成的回复字符串和错误
func (s *AIService) GenerateResponse(ctx context.Context, messages []ChatContextMessage, triggerMessage string) (string, error) {
	if !s.IsEnabled() {
		return "", fmt.Errorf("AI service is not enabled")
	}

	// 构建对话上下文
	var conversationBuilder strings.Builder
	for _, msg := range messages {
		if msg.SenderID == AIBotUserID {
			conversationBuilder.WriteString(fmt.Sprintf("[FanAI(你)]: %s\n", msg.Content))
		} else {
			conversationBuilder.WriteString(fmt.Sprintf("[%s]: %s\n", msg.SenderName, msg.Content))
		}
	}

	// 准备ToolCall工具
	tools := s.GetToolDefinitions()
	var openaiTools []openai.ChatCompletionToolParam
	for _, t := range tools {
		openaiTools = append(openaiTools, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  t.Parameters,
			},
		})
	}

	params := openai.ChatCompletionNewParams{
		Model: s.config.Model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(s.config.SystemPrompt + "\n\n你正在参与一个聊天对话，请根据上下文自然地回复。回复要简洁有趣，像一个真实的朋友一样。"),
			openai.UserMessage(fmt.Sprintf("对话上下文：\n%s\n\n最新消息：[%s]: %s\n\n请回复：", conversationBuilder.String(), "用户", triggerMessage)),
		},
		MaxTokens:   openai.Int(int64(s.config.MaxTokens)),
		Temperature: openai.Float(s.config.Temperature),
	}
	if len(openaiTools) > 0 {
		params.Tools = openaiTools
	}

	// 启动Agent Loop，支持多轮ToolCall
	return s.agentLoop(ctx, params, tools, 3)
}

// agentLoop Agent Loop核心实现，支持多轮ToolCall
// ctx: 上下文
// params: 初始请求参数
// tools: 工具定义列表
// maxIterations: 最大迭代次数
// 返回: 最终生成的回复和错误
func (s *AIService) agentLoop(ctx context.Context, params openai.ChatCompletionNewParams, tools []ToolDefinition, maxIterations int) (string, error) {
	msgHistory := params.Messages // 保存消息历史，用于迭代

	for i := 0; i < maxIterations; i++ {
		params.Messages = msgHistory
		resp, err := s.client.Chat.Completions.New(ctx, params)
		if err != nil {
			log.Printf("[AIService] agentLoop iteration %d error: %v", i, err)
			return "", err
		}

		choice := resp.Choices[0]
		msg := choice.Message

		// 如果没有ToolCall，直接返回内容
		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}

		// 构建包含tool_calls的assistant消息
		assistantMsg := openai.ChatCompletionAssistantMessageParam{
			ToolCalls: make([]openai.ChatCompletionMessageToolCallParam, len(msg.ToolCalls)),
		}
		for j, tc := range msg.ToolCalls {
			assistantMsg.ToolCalls[j] = tc.ToParam()
		}
		msgHistory = append(msgHistory, openai.ChatCompletionMessageParamUnion{
			OfAssistant: &assistantMsg,
		})

		// 逐个处理ToolCall
		for _, toolCall := range msg.ToolCalls {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				log.Printf("[AIService] Failed to parse tool args: %v", err)
				msgHistory = append(msgHistory, openai.ChatCompletionMessageParamUnion{
					OfTool: &openai.ChatCompletionToolMessageParam{
						ToolCallID: toolCall.ID,
						Content:    openai.ChatCompletionToolMessageParamContentUnion{OfString: openai.String(fmt.Sprintf("Error parsing arguments: %v", err))},
					},
				})
				continue
			}

			var result string
			handlerFound := false
			for _, tool := range tools {
				if tool.Name == toolCall.Function.Name {
					handlerFound = true
					result, err = tool.Handler(args)
					if err != nil {
						result = fmt.Sprintf("Error: %v", err)
					}
					break
				}
			}
			if !handlerFound {
				result = fmt.Sprintf("Unknown tool: %s", toolCall.Function.Name)
			}

			// 将tool结果加入消息历史
			msgHistory = append(msgHistory, openai.ChatCompletionMessageParamUnion{
				OfTool: &openai.ChatCompletionToolMessageParam{
					ToolCallID: toolCall.ID,
					Content:    openai.ChatCompletionToolMessageParamContentUnion{OfString: openai.String(result)},
				},
			})
		}
	}

	return "", fmt.Errorf("agent loop exceeded max iterations")
}

// handleGetUserInfo ToolCall：获取用户信息
func (s *AIService) handleGetUserInfo(args map[string]interface{}) (string, error) {
	userID, ok := args["user_id"].(string)
	if !ok {
		return "", fmt.Errorf("user_id is required")
	}

	var user model.User
	if err := model.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return fmt.Sprintf("User not found: %s", userID), nil
	}

	result, _ := json.Marshal(map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"avatar_url": user.AvatarURL,
	})
	return string(result), nil
}

// handleGetChatContext ToolCall：获取聊天上下文
func (s *AIService) handleGetChatContext(args map[string]interface{}) (string, error) {
	roomID, ok := args["room_id"].(string)
	if !ok {
		return "", fmt.Errorf("room_id is required")
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	var messages []model.ChatMessage
	if err := model.DB.Where("room_id = ?", roomID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		return fmt.Sprintf("Failed to get chat context: %v", err), nil
	}

	type contextMsg struct {
		SenderID  string `json:"sender_id"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
	}

	var contextMessages []contextMsg
	for _, msg := range messages {
		contextMessages = append(contextMessages, contextMsg{
			SenderID:  msg.SenderID,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	result, _ := json.Marshal(contextMessages)
	return string(result), nil
}

// extractJSON 从文本中提取JSON部分（兼容AI可能返回额外说明的情况）
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end < start {
		return s
	}
	return s[start : end+1]
}

// ChatContextMessage 聊天上下文消息结构
type ChatContextMessage struct {
	SenderID   string // 发送者ID
	SenderName string // 发送者名称
	Content    string // 消息内容
}

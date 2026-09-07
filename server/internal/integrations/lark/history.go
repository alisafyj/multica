package lark

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/multica-ai/multica/server/internal/integrations/channel"
)

// MessagePageReader exposes provider pagination without changing the bounded
// single-page API used by inbound context enrichment.
type MessagePageReader interface {
	ListChatMessagesPage(context.Context, InstallationCredentials, ListMessagesParams, string) ([]LarkMessage, string, error)
}

// ReadThreadHistory reads only the caller-authorized topic. Neither a provider
// cursor nor a returned message can broaden that topic or its context window.
func ReadThreadHistory(ctx context.Context, api APIClient, creds InstallationCredentials, chatID, threadID, botOpenID string, opts channel.HistoryOptions) (channel.HistoryPage, error) {
	page := channel.HistoryPage{ChannelType: "feishu", ThreadID: threadID}
	if opts.BoundaryPending {
		return page, nil
	}
	reader, ok := api.(MessagePageReader)
	if !ok {
		return page, ErrAPIClientNotConfigured
	}
	if chatID == "" || threadID == "" {
		return page, errors.New("Feishu history requires a bound topic")
	}
	boundaryTime := func(id string) (int64, error) {
		if id == "" {
			return 0, nil
		}
		messages, err := api.GetMessage(ctx, creds, id)
		if err != nil {
			return 0, err
		}
		if len(messages) != 1 || messages[0].MessageID != id || !historyMessageInTopic(messages[0], chatID, threadID) || messages[0].Deleted || messages[0].UpperMessageID != "" {
			return 0, errors.New("Feishu history boundary is not in the bound topic")
		}
		millis := parseLarkMillis(messages[0].CreateTime)
		if millis <= 0 {
			return 0, errors.New("Feishu history boundary has no valid timestamp")
		}
		return millis, nil
	}
	start, err := boundaryTime(opts.After)
	if err != nil {
		return page, fmt.Errorf("read Feishu history start: %w", err)
	}
	end, err := boundaryTime(opts.Until)
	if err != nil {
		return page, fmt.Errorf("read Feishu history end: %w", err)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 30
	} else if limit > larkListMessagesMaxPageSize {
		limit = larkListMessagesMaxPageSize
	}
	messages, next, err := reader.ListChatMessagesPage(ctx, creds, ListMessagesParams{
		ChatID: ChatID(chatID), ThreadID: threadID, PageSize: limit,
	}, opts.Before)
	if err != nil {
		return page, err
	}
	page.NextCursor = next
	page.Messages = make([]channel.HistoryMessage, 0, len(messages))
	for _, message := range messages {
		if !historyMessageInTopic(message, chatID, threadID) {
			return channel.HistoryPage{}, errors.New("Feishu returned a message outside the authorized topic")
		}
		if message.Deleted || message.UpperMessageID != "" {
			continue
		}
		millis := parseLarkMillis(message.CreateTime)
		if millis <= 0 {
			return channel.HistoryPage{}, errors.New("Feishu history message has no valid timestamp")
		}
		// Message IDs do not order equal-millisecond messages. Fail closed at
		// the start edge except for the exact boundary message itself.
		if start > 0 && (millis < start || (millis == start && message.MessageID != opts.After)) {
			continue
		}
		if end > 0 && millis >= end {
			continue
		}
		text := resolveMentions(flattenContent(message.MessageType, message.Content), restMentionsToEvent(message.Mentions), "", "")
		role := channel.HistoryRoleUser
		author := message.SenderID
		if message.SenderType == "app" && (message.SenderID == creds.AppID || message.SenderID == botOpenID) {
			role, author = channel.HistoryRoleAssistant, "Bot"
		}
		page.Messages = append(page.Messages, channel.HistoryMessage{
			ID: message.MessageID, Author: author, AuthorID: message.SenderID,
			Role: role, Text: text, TS: message.CreateTime,
		})
	}
	sort.SliceStable(page.Messages, func(i, j int) bool {
		return parseLarkMillis(page.Messages[i].TS) < parseLarkMillis(page.Messages[j].TS)
	})
	return page, nil
}

func historyMessageInTopic(message LarkMessage, chatID, threadID string) bool {
	return message.MessageID != "" && message.ChatID == chatID && message.ThreadID == threadID
}

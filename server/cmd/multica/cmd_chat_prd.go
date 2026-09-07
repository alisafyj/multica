package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var chatPRDCmd = &cobra.Command{
	Use:   "prd",
	Short: "Draft and publish a PRD in the current Feishu topic",
	Long:  "PRD drafts stay local until the original human requester confirms the exact draft and version in the same topic. No user or session scope can be supplied by the caller.",
}

var chatPRDGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Read the current topic's PRD draft and publication state",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return fetchAndPrintChatSessionJSON(cmd, "/api/chat/prd")
	},
}

var chatPRDDraftCmd = &cobra.Command{
	Use:   "draft --source-message <root-message-id> --content-file <path>",
	Short: "Save your structured JSON as a local draft; never create a document",
	Args:  cobra.NoArgs,
	RunE:  runChatPRDDraft,
}

var chatPRDPublishCmd = &cobra.Command{
	Use:   "publish --draft <id> --version <n> --confirmation-message <message-id>",
	Short: "Verify a real initiator confirmation and publish the stored snapshot",
	Args:  cobra.NoArgs,
	RunE:  runChatPRDPublish,
}

var chatPRDTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "Read the configured PRD template's exact section headings",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return fetchAndPrintChatSessionJSON(cmd, "/api/chat/prd/template")
	},
}

func init() {
	chatPRDDraftCmd.Flags().String("source-message", "", "The original human PRD request's topic-root Feishu message ID")
	chatPRDDraftCmd.Flags().String("content-file", "", "Structured UTF-8 draft JSON inside the current task working directory")
	chatPRDPublishCmd.Flags().String("draft", "", "Server-issued draft ID")
	chatPRDPublishCmd.Flags().Int("version", 0, "Exact server-issued draft version")
	chatPRDPublishCmd.Flags().String("confirmation-message", "", "The original requester's real confirmation message ID in this topic")
	chatPRDCmd.AddCommand(chatPRDGetCmd, chatPRDTemplateCmd, chatPRDDraftCmd, chatPRDPublishCmd)
	chatCmd.AddCommand(chatPRDCmd)
}

func runChatPRDDraft(cmd *cobra.Command, _ []string) error {
	source, _ := cmd.Flags().GetString("source-message")
	content, ok, err := resolveTextFlag(cmd, "content")
	if err != nil {
		return err
	}
	if strings.TrimSpace(source) == "" || !ok {
		return fmt.Errorf("--source-message and --content-file are required")
	}
	if !utf8.ValidString(content) || !json.Valid([]byte(content)) || !strings.HasPrefix(strings.TrimSpace(content), "{") {
		return fmt.Errorf("--content-file must contain one UTF-8 JSON draft object")
	}
	return postChatPRD(cmd, "/api/chat/prd/draft", map[string]any{
		"source_message_id": source, "content": json.RawMessage(content),
	})
}

func runChatPRDPublish(cmd *cobra.Command, _ []string) error {
	draft, _ := cmd.Flags().GetString("draft")
	version, _ := cmd.Flags().GetInt("version")
	confirmation, _ := cmd.Flags().GetString("confirmation-message")
	if draft == "" || version < 1 || confirmation == "" {
		return fmt.Errorf("--draft, positive --version and --confirmation-message are required")
	}
	return postChatPRD(cmd, "/api/chat/prd/publish", map[string]any{
		"draft_id": draft, "version": version, "confirmation_message_id": confirmation,
	})
}

func postChatPRD(cmd *cobra.Command, path string, body any) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	var response any
	if err := client.PostJSON(ctx, path, body, &response); err != nil {
		return fmt.Errorf("PRD request failed (use multica chat prd get to inspect durable state): %w", err)
	}
	return cli.PrintJSON(os.Stdout, response)
}

package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
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
	Use:   "draft --source-message <root-message-id> --generation-task <task-id>",
	Short: "Recover the delegated PRD JSON as a local draft; never create a document",
	Args:  cobra.NoArgs,
	RunE:  runChatPRDDraft,
}

var chatPRDPublishCmd = &cobra.Command{
	Use:   "publish --draft <id> --version <n> --confirmation-message <message-id>",
	Short: "Verify a real initiator confirmation and publish the stored snapshot",
	Args:  cobra.NoArgs,
	RunE:  runChatPRDPublish,
}

var chatPRDDelegateCmd = &cobra.Command{
	Use:   "delegate --source-message <root-message-id> --brief-file <path>",
	Short: "Ask the server-configured 小码 agent to generate a draft only",
	Args:  cobra.NoArgs,
	RunE:  runChatPRDDelegate,
}

var chatPRDGenerationCmd = &cobra.Command{
	Use:   "generation",
	Short: "Recover this topic's delegated draft generation and its result",
	Args:  cobra.NoArgs,
	RunE:  runChatPRDGeneration,
}

func init() {
	chatPRDDraftCmd.Flags().String("source-message", "", "The original human PRD request's topic-root Feishu message ID")
	chatPRDDraftCmd.Flags().String("generation-task", "", "The completed draft generation task returned by delegate")
	chatPRDDelegateCmd.Flags().String("source-message", "", "The original human PRD request's topic-root Feishu message ID")
	chatPRDDelegateCmd.Flags().String("brief-file", "", "UTF-8 requirements brief inside the working directory")
	chatPRDDelegateCmd.Flags().Bool("retry", false, "Explicitly retry a failed, cancelled or invalid generation; never duplicate active or valid completed work")
	chatPRDGenerationCmd.Flags().String("task", "", "Exact generation task ID; omit to recover the latest in this topic")
	chatPRDGenerationCmd.Flags().Duration("wait", 0, "Wait for a terminal result, at most 10m (e.g. --wait 5m)")
	chatPRDPublishCmd.Flags().String("draft", "", "Server-issued draft ID")
	chatPRDPublishCmd.Flags().Int("version", 0, "Exact server-issued draft version")
	chatPRDPublishCmd.Flags().String("confirmation-message", "", "The original requester's real confirmation message ID in this topic")
	chatPRDCmd.AddCommand(chatPRDGetCmd, chatPRDDelegateCmd, chatPRDGenerationCmd, chatPRDDraftCmd, chatPRDPublishCmd)
	chatCmd.AddCommand(chatPRDCmd)
}

func runChatPRDDraft(cmd *cobra.Command, _ []string) error {
	source, _ := cmd.Flags().GetString("source-message")
	generation, _ := cmd.Flags().GetString("generation-task")
	if strings.TrimSpace(source) == "" || generation == "" {
		return fmt.Errorf("--source-message and --generation-task are required")
	}
	return postChatPRD(cmd, "/api/chat/prd/draft", map[string]any{
		"source_message_id": source, "generation_task_id": generation,
	})
}

func runChatPRDDelegate(cmd *cobra.Command, _ []string) error {
	source, _ := cmd.Flags().GetString("source-message")
	retry, _ := cmd.Flags().GetBool("retry")
	brief, ok, err := resolveTextFlag(cmd, "brief")
	if err != nil {
		return err
	}
	if strings.TrimSpace(source) == "" || !ok || strings.TrimSpace(brief) == "" {
		return fmt.Errorf("--source-message and --brief-file are required")
	}
	if !utf8.ValidString(brief) {
		return fmt.Errorf("--brief-file must contain valid UTF-8 text")
	}
	return postChatPRD(cmd, "/api/chat/prd/delegate", map[string]any{
		"source_message_id": source, "brief": brief, "retry": retry,
	})
}

func runChatPRDGeneration(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	taskID, _ := cmd.Flags().GetString("task")
	waitFor, _ := cmd.Flags().GetDuration("wait")
	if waitFor < 0 || waitFor > 10*time.Minute {
		return fmt.Errorf("--wait must be between 0 and 10m")
	}
	deadline := time.Now().Add(waitFor)
	for {
		path := "/api/chat/prd/generation"
		if taskID != "" {
			path += "?task_id=" + url.QueryEscape(taskID)
		}
		ctx, cancel := cli.APIContext(cmd.Context())
		var response map[string]any
		err := client.GetJSON(ctx, path, &response)
		cancel()
		if err != nil {
			return fmt.Errorf("read PRD generation: %w", err)
		}
		status, _ := response["status"].(string)
		terminal := status == "completed" || status == "failed" || status == "cancelled" || status == "invalid"
		if terminal || waitFor == 0 || !time.Now().Before(deadline) {
			if err := cli.PrintJSON(os.Stdout, response); err != nil {
				return err
			}
			if terminal && status != "completed" {
				return fmt.Errorf("PRD generation %s; do not self-generate or publish a substitute", status)
			}
			return nil
		}
		// Pin the first recovered generation so a concurrent new request never
		// changes which result a waiting coordinator receives.
		if taskID == "" {
			taskID, _ = response["task_id"].(string)
		}
		timer := time.NewTimer(min(2*time.Second, time.Until(deadline)))
		select {
		case <-cmd.Context().Done():
			timer.Stop()
			return cmd.Context().Err()
		case <-timer.C:
		}
	}
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

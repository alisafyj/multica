/**
 * My Issues board — web's kanban laid out for a phone.
 *
 * Web renders every status column side by side in one horizontally
 * scrolling row (`packages/views/issues/components/board-view.tsx:612`).
 * At 375pt that would show about one and a half columns, so mobile gives
 * each status a full-width page and pages between them.
 *
 * Container choice walked the iOS-native > RNR > discuss waterfall in
 * apps/mobile/CLAUDE.md and stopped at step 1: RN's own `FlatList` with
 * `horizontal` + `pagingEnabled` is UIScrollView paging, so the snap
 * physics are the platform's. No pager dependency was added. Cards inside
 * a column use FlashList, matching `chat-message-list.tsx`.
 *
 * What this deliberately does NOT do: drag a card to another column. Web
 * moves issues with `@dnd-kit` (`board-view.tsx` `handleDragEnd`), but a
 * horizontal card drag is the same gesture as a page turn. Status changes
 * keep going through the card → detail → status picker path that already
 * exists on mobile, so both clients still funnel into the same mutation.
 *
 * Parity points, all inherited from `buildBoardColumns`:
 *   - Columns and their order come from `BOARD_STATUSES`, `cancelled`
 *     excluded — the same list web's board groups by.
 *   - Per-column counts are the count of issues in that column, from the
 *     same already-filtered array the list view renders, so a status shows
 *     the same N in both mobile views and on web.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  FlatList,
  Pressable,
  ScrollView,
  View,
  useWindowDimensions,
  type LayoutChangeEvent,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
} from "react-native";
import { FlashList } from "@shopify/flash-list";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { Issue, IssueStatus, Project } from "@multica/core/types";
import { Text } from "@/components/ui/text";
import { CardPressable } from "@/components/ui/card";
import { ActorAvatar } from "@/components/ui/actor-avatar";
import { PriorityIcon } from "@/components/ui/priority-icon";
import { ProjectIcon } from "@/components/ui/project-icon";
import { StatusIcon } from "@/components/ui/status-icon";
import { findProject, projectListOptions } from "@/data/queries/projects";
import { useWorkspaceStore } from "@/data/workspace-store";
import { buildBoardColumns, type BoardColumn } from "@/lib/board-columns";
import { descriptionPreview } from "@/lib/description-preview";
import { issueStatusLabel } from "@/lib/issue-status";
import { timeAgo } from "@/lib/time-ago";

interface Props {
  /** Already status/priority-filtered — same array the list view renders. */
  issues: Issue[];
  statusFilters: IssueStatus[];
  onPressIssue: (issue: Issue) => void;
  refreshing: boolean;
  onRefresh: () => void;
}

export function IssueBoard({
  issues,
  statusFilters,
  onPressIssue,
  refreshing,
  onRefresh,
}: Props) {
  const { width } = useWindowDimensions();
  const pagerRef = useRef<FlatList<BoardColumn>>(null);
  const [activeIndex, setActiveIndex] = useState(0);

  const columns = useMemo(
    () => buildBoardColumns(issues, statusFilters),
    [issues, statusFilters],
  );

  // One project query for the whole board instead of one per card — mirrors
  // web, which threads a `projectMap` down from the surface controller.
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const { data: projects = [] } = useQuery(projectListOptions(wsId));

  // Open on the first status that actually has something in it. Backlog is
  // column 0 and is usually empty, so without this the board's first frame is
  // an empty page — the exact "not much to look at" problem it exists to fix.
  // Fires once, on the first render that has data, so it can never yank the
  // page out from under a swipe.
  const didPickInitialPage = useRef(false);
  useEffect(() => {
    if (didPickInitialPage.current) return;
    const firstPopulated = columns.findIndex((c) => c.issues.length > 0);
    if (firstPopulated < 0) return;
    didPickInitialPage.current = true;
    if (firstPopulated === 0) return;
    setActiveIndex(firstPopulated);
    pagerRef.current?.scrollToOffset({
      offset: firstPopulated * width,
      animated: false,
    });
  }, [columns, width]);

  // Changing the status filter can shrink the column set out from under the
  // current page. Clamp before the pager renders a page that no longer
  // exists, otherwise it strands on a blank screen with no way back.
  useEffect(() => {
    if (activeIndex <= columns.length - 1) return;
    const clamped = Math.max(0, columns.length - 1);
    setActiveIndex(clamped);
    pagerRef.current?.scrollToOffset({
      offset: clamped * width,
      animated: false,
    });
  }, [activeIndex, columns.length, width]);

  const goToIndex = useCallback(
    (index: number) => {
      setActiveIndex(index);
      pagerRef.current?.scrollToOffset({
        offset: index * width,
        animated: true,
      });
    },
    [width],
  );

  // Derived from `onScroll`, NOT `onMomentumScrollEnd`. On Android a slow
  // drag-and-release has no fling velocity, so the scroll ends through the
  // paging snap without ever entering a momentum phase and the momentum
  // callback never fires — the page moves while the strip stays behind,
  // showing no selected tab at all. Reading every scroll frame also makes the
  // strip track the finger mid-swipe instead of jumping at the end.
  const onScroll = useCallback(
    (event: NativeSyntheticEvent<NativeScrollEvent>) => {
      const next = Math.round(event.nativeEvent.contentOffset.x / width);
      setActiveIndex((current) => (current === next ? current : next));
    },
    [width],
  );

  return (
    <View className="flex-1">
      <StatusStrip
        columns={columns}
        activeIndex={activeIndex}
        onSelect={goToIndex}
      />
      <FlatList
        ref={pagerRef}
        data={columns}
        keyExtractor={(column) => column.status}
        horizontal
        pagingEnabled
        // Explicit flex, not `flex-1`: the pager must own the leftover height
        // so each page stretches to it.
        style={{ flex: 1 }}
        showsHorizontalScrollIndicator={false}
        onScroll={onScroll}
        scrollEventThrottle={16}
        // Every page is exactly one screen wide, so the pager can place any
        // page without measuring — this is what makes the clamp above and
        // the strip's scrollToOffset land on an exact page boundary.
        getItemLayout={(_, index) => ({
          length: width,
          offset: width * index,
          index,
        })}
        renderItem={({ item }) => (
          <BoardColumnPage
            column={item}
            width={width}
            projects={projects}
            onPressIssue={onPressIssue}
            refreshing={refreshing}
            onRefresh={onRefresh}
          />
        )}
      />
    </View>
  );
}

/**
 * Tappable status tabs above the pager. Exists because the board keeps
 * empty columns (see `buildBoardColumns`) — without a way to jump, reaching
 * "Blocked" past four empty statuses is four swipes.
 */
function StatusStrip({
  columns,
  activeIndex,
  onSelect,
}: {
  columns: BoardColumn[];
  activeIndex: number;
  onSelect: (index: number) => void;
}) {
  const { t } = useTranslation("issues");
  const scrollRef = useRef<ScrollView>(null);
  const tabLayouts = useRef<Record<number, { x: number; width: number }>>({});
  const stripWidth = useRef(0);

  // Six statuses don't fit on a 375pt screen, so swiping to a tab that's off
  // to the right has to bring it into view or the strip stops reflecting
  // where you are. Centre it when there's room to.
  useEffect(() => {
    const tab = tabLayouts.current[activeIndex];
    if (!tab || stripWidth.current === 0) return;
    scrollRef.current?.scrollTo({
      x: Math.max(0, tab.x - (stripWidth.current - tab.width) / 2),
      animated: true,
    });
  }, [activeIndex]);

  return (
    <ScrollView
      ref={scrollRef}
      horizontal
      showsHorizontalScrollIndicator={false}
      onLayout={(e: LayoutChangeEvent) => {
        stripWidth.current = e.nativeEvent.layout.width;
      }}
      // A ScrollView in a flex column takes the leftover height, and the
      // default `align-items: stretch` then makes each pill as tall as the
      // strip. `flexGrow: 0` keeps the strip at content height; `center`
      // keeps the pills at their own.
      style={{ flexGrow: 0 }}
      contentContainerStyle={{ alignItems: "center" }}
      contentContainerClassName="flex-row gap-1 px-4 pb-2"
    >
      {columns.map((column, index) => {
        const active = index === activeIndex;
        return (
          <Pressable
            key={column.status}
            onPress={() => onSelect(index)}
            onLayout={(e: LayoutChangeEvent) => {
              const { x, width } = e.nativeEvent.layout;
              tabLayouts.current[index] = { x, width };
            }}
            accessibilityRole="tab"
            accessibilityState={{ selected: active }}
            className={`flex-row items-center gap-1.5 rounded-full px-3 py-1.5 ${
              active ? "bg-accent" : "active:bg-secondary"
            }`}
          >
            <StatusIcon status={column.status} size={13} />
            {/* Active state must survive hover/press: it carries weight and
                text colour, not just the background (UI Rules, root CLAUDE.md). */}
            <Text
              numberOfLines={1}
              className={
                active
                  ? "text-xs font-medium text-accent-foreground"
                  : "text-xs text-muted-foreground"
              }
            >
              {issueStatusLabel(t, column.status)}
            </Text>
            <Text
              className={
                active
                  ? "text-xs text-accent-foreground/70"
                  : "text-xs text-muted-foreground/60"
              }
            >
              {column.issues.length}
            </Text>
          </Pressable>
        );
      })}
    </ScrollView>
  );
}

function BoardColumnPage({
  column,
  width,
  projects,
  onPressIssue,
  refreshing,
  onRefresh,
}: {
  column: BoardColumn;
  width: number;
  projects: Project[];
  onPressIssue: (issue: Issue) => void;
  refreshing: boolean;
  onRefresh: () => void;
}) {
  const { t } = useTranslation("issues");
  // `width` is set as a style, never via a `flex-*` class: `flex: 1` implies
  // `flexBasis: 0`, which in this row-direction list overrides the explicit
  // width and desynchronises page N from offset N * width — the pager then
  // lands on the wrong status.
  if (column.issues.length === 0) {
    return (
      <View
        style={{ width }}
        className="h-full items-center justify-center px-8"
      >
        <StatusIcon status={column.status} size={22} />
        <Text className="pt-3 text-sm text-muted-foreground text-center">
          {/* Web's board.empty_column is a bare "No issues"; mobile names the
              status because the board is a swipe pager showing one column at
              a time, so the surrounding context web relies on isn't there. */}
          {t("my_issues.board.empty_column", {
            status: issueStatusLabel(t, column.status),
          })}
        </Text>
      </View>
    );
  }

  return (
    <View style={{ width }} className="h-full">
      <FlashList
        data={column.issues}
        keyExtractor={(issue) => issue.id}
        renderItem={({ item }) => (
          <BoardCard
            issue={item}
            project={findProject(projects, item.project_id)}
            onPress={() => onPressIssue(item)}
          />
        )}
        ItemSeparatorComponent={() => <View className="h-2" />}
        contentContainerClassName="px-4 pb-6"
        refreshing={refreshing}
        onRefresh={onRefresh}
      />
    </View>
  );
}

/**
 * Card contents mirror web's `BoardCardContent` minus the affordances that
 * need a pointer: identifier + priority, title, description preview,
 * project, assignee, updated-at. Web gates each of these behind a Display
 * option; mobile has no display-options store, so it shows the set web
 * enables by default.
 */
function BoardCard({
  issue,
  project,
  onPress,
}: {
  issue: Issue;
  project: Project | undefined;
  onPress: () => void;
}) {
  const preview = issue.description ? descriptionPreview(issue.description) : "";

  return (
    <CardPressable onPress={onPress} className="gap-2 p-3">
      <View className="flex-row items-center gap-2">
        <PriorityIcon priority={issue.priority} size={13} />
        <Text className="text-xs text-muted-foreground">
          {issue.identifier}
        </Text>
      </View>

      <Text className="text-sm text-foreground" numberOfLines={2}>
        {issue.title}
      </Text>

      {preview ? (
        <Text className="text-xs text-muted-foreground" numberOfLines={1}>
          {preview}
        </Text>
      ) : null}

      {project ? (
        <View className="flex-row items-center gap-1 self-start rounded-md border border-border px-1.5 py-0.5">
          <ProjectIcon icon={project.icon} size="sm" />
          <Text className="text-xs text-muted-foreground" numberOfLines={1}>
            {project.title}
          </Text>
        </View>
      ) : null}

      <View className="flex-row items-center gap-2">
        {issue.assignee_type && issue.assignee_id ? (
          <ActorAvatar
            type={issue.assignee_type}
            id={issue.assignee_id}
            size={18}
            showPresence
          />
        ) : null}
        <Text className="flex-1 text-xs text-muted-foreground/70" numberOfLines={1}>
          {timeAgo(issue.updated_at)}
        </Text>
      </View>
    </CardPressable>
  );
}

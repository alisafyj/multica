"use client";

import { useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  designRepositoryCatalogueOptions,
  projectDesignAssetListOptions,
  repositoryDesignAssetListOptions,
} from "@multica/core/designs/queries";
import { designKeys } from "@multica/core/designs/keys";
import type { DesignAssetListItem } from "@multica/core/designs";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { DesignMvpAssociationDialog } from "./design-mvp-association-dialog";
import { DesignMvpViewSwitcher, type DesignMvpViewMode } from "./design-mvp-view-switcher";

export interface DesignMvpRepository {
  id: string;
  projectId: string;
  projectTitle: string;
  label: string;
  repositoryUrl: string;
  defaultBranchHint: string;
}

const repositoryLabel = (repository: DesignMvpRepository) =>
  `${repository.projectTitle} · ${repository.label} · ${repository.repositoryUrl}`;

function DesignMvpCard({
  item,
  onAssociate,
}: {
  item: DesignAssetListItem;
  onAssociate: (item: DesignAssetListItem) => void;
}) {
  return (
    <article className="flex flex-col gap-3 rounded-lg border bg-background p-3">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <h4 className="truncate text-body font-medium">{item.title}</h4>
          <p className="text-caption text-muted-foreground">{item.sourceLabel}</p>
        </div>
        <Button
          size="xs"
          variant="outline"
          aria-label={`${item.projectResourceId ? "更换仓库" : "关联仓库"}：${item.title}`}
          onClick={() => onAssociate(item)}
        >
          {item.projectResourceId ? "更换仓库" : "关联仓库"}
        </Button>
      </div>
      <p className="text-caption text-muted-foreground">
        {item.hasSavedVersion ? "已有保存版本" : "暂无保存版本"} · {item.hasDraftVersion ? "有草稿" : "无草稿"}
      </p>
    </article>
  );
}

function DesignMvpPanel({
  title,
  items,
  loading,
  onAssociate,
}: {
  title: string;
  items: DesignAssetListItem[];
  loading: boolean;
  onAssociate: (item: DesignAssetListItem) => void;
}) {
  return (
    <section className="space-y-3" aria-label={title}>
      <h3 className="text-label font-medium text-foreground">{title}</h3>
      {loading ? (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} className="h-24" />)}
        </div>
      ) : items.length === 0 ? (
        <p className="rounded-lg border bg-muted/20 p-4 text-caption text-muted-foreground">暂无内容。</p>
      ) : (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {items.map((item) => <DesignMvpCard key={`${item.kind}:${item.id}`} item={item} onAssociate={onAssociate} />)}
        </div>
      )}
    </section>
  );
}

export function DesignMvpWorkspace() {
  const wsId = useWorkspaceId();
  const queryClient = useQueryClient();
  const [mode, setMode] = useState<DesignMvpViewMode>("project");
  const [projectId, setProjectId] = useState("");
  const [repositoryId, setRepositoryId] = useState("");
  const [associationItem, setAssociationItem] = useState<DesignAssetListItem | null>(null);

  const { data: projects = [] } = useQuery({
    queryKey: ["projects", wsId, "design-mvp"],
    queryFn: () => api.listProjects(),
    select: (data) => data.projects,
  });
  const { data: repositories = [], isLoading: repositoriesLoading } = useQuery(
    designRepositoryCatalogueOptions(wsId),
  );
  const selectedRepository = repositories.find((repository) => repository.id === repositoryId);
  const projectAssets = useQuery(projectDesignAssetListOptions(wsId, mode === "project" ? projectId : ""));
  const repositoryAssets = useQuery(repositoryDesignAssetListOptions(
    wsId,
    selectedRepository?.projectId ?? "",
    selectedRepository?.id ?? "",
  ));

  const items = mode === "project" ? projectAssets.data ?? [] : repositoryAssets.data ?? [];
  const savedItems = items.filter((item) => item.hasSavedVersion);
  const draftItems = items.filter((item) => item.hasDraftVersion);
  const loading = mode === "project" ? projectAssets.isLoading : repositoryAssets.isLoading;
  const associatedProjectIdRef = useRef<string | null>(null);
  const associationMutation = useMutation({
    mutationFn: async (targetRepositoryId: string) => {
      if (!associationItem) throw new Error("association item missing");
      associatedProjectIdRef.current = associationItem.projectId;
      await api.setDesignAssetRepositoryAssociation({
        project_id: associationItem.projectId,
        project_resource_id: targetRepositoryId,
        items: [{ kind: associationItem.kind === "figma_file" ? "design_file" : "design_document", id: associationItem.id }],
      });
    },
    onSuccess: async () => {
      const associatedProjectId = associatedProjectIdRef.current;
      await queryClient.invalidateQueries({ queryKey: designKeys.files(wsId) });
      await queryClient.invalidateQueries({ queryKey: designKeys.documents(wsId, projectId) });
      if (associatedProjectId) {
        await queryClient.invalidateQueries({
          queryKey: designKeys.assetsByProject(wsId, associatedProjectId),
        });
      }
      if (selectedRepository) {
        await queryClient.invalidateQueries({
          queryKey: designKeys.assetsByRepository(wsId, selectedRepository.projectId, selectedRepository.id),
        });
      }
      setAssociationItem(null);
    },
  });

  const selectableRepositories = useMemo(
    () => mode === "repository" ? repositories : repositories.filter((repository) => repository.projectId === projectId),
    [mode, projectId, repositories],
  );
  const header = mode === "project"
    ? projects.find((project) => project.id === projectId)?.title ?? "选择一个项目"
    : selectedRepository ? repositoryLabel(selectedRepository) : "选择一个仓库";

  return (
    <div className="flex min-h-0 flex-col">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b bg-muted/20 px-4 py-3">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <DesignMvpViewSwitcher mode={mode} onModeChange={setMode} />
          <select
            aria-label={mode === "project" ? "选择项目" : "选择仓库"}
            value={mode === "project" ? projectId : repositoryId}
            onChange={(event) => {
              const next = event.target.value;
              if (!next) return;
              if (mode === "project") setProjectId(next);
              else setRepositoryId(next);
            }}
            className="h-8 max-w-md rounded-lg border bg-background px-2 text-body"
          >
            <option value="">{mode === "project" ? "选择项目" : "选择仓库"}</option>
            {mode === "project"
              ? projects.map((project) => <option key={project.id} value={project.id}>{project.title}</option>)
              : repositories.map((repository) => <option key={repository.id} value={repository.id}>{repositoryLabel(repository)}</option>)}
          </select>
        </div>
        <p className="min-w-0 truncate text-caption text-muted-foreground">{header}</p>
      </div>
      <div className="min-h-0 space-y-6 overflow-auto p-4">
        <DesignMvpPanel title="设计稿" items={savedItems} loading={loading} onAssociate={setAssociationItem} />
        <DesignMvpPanel title="设计草稿" items={draftItems} loading={loading} onAssociate={setAssociationItem} />
      </div>
      <DesignMvpAssociationDialog
        open={associationItem !== null}
        item={associationItem ? {
          id: associationItem.id,
          kind: associationItem.kind === "figma_file" ? "design_file" : "design_document",
          projectId: associationItem.projectId,
          projectResourceId: associationItem.projectResourceId,
          title: associationItem.title,
          sourceLabel: associationItem.sourceLabel,
        } : null}
        repositories={selectableRepositories}
        pending={associationMutation.isPending}
        error={associationMutation.error}
        onClose={() => { setAssociationItem(null); associationMutation.reset(); }}
        onConfirm={async (targetRepositoryId) => { await associationMutation.mutateAsync(targetRepositoryId); }}
      />
      {mode === "repository" && repositoriesLoading ? <span className="sr-only">正在加载仓库目录</span> : null}
    </div>
  );
}

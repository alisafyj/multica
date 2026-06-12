export type DesignSourceType = "upload" | "ai_generated" | "template" | "import";
export type DesignRevisionStatus = "draft" | "valid" | "invalid";
export type DesignAssetKind = "frame_preview" | "frame_thumbnail" | "image" | "slice" | "thumbnail" | "source" | "other";
export type DesignTemplateSlotType = "text" | "number" | "boolean" | "image" | "color" | "enum" | "list" | "object";
export type DesignDraftStatus = "draft" | "generated" | "validated" | "failed" | "archived";
export type DesignRestoreTaskStatus = "queued" | "running" | "completed" | "failed" | "cancelled";
export type DesignRestoreTargetKind = "component" | "file" | "symbol" | "route" | "unknown";
export type DesignRestoreTaskPurpose = "frontend_restore" | "ui_generation" | "template_annotation";
export type DesignRestoreTaskItemSource = "frame" | "selected_layers" | "selection_bounds" | "template" | "draft";
export type DesignProjectRulesSource = "project_rules" | "gallery_specs_legacy" | "inline" | "none";
export type GalleryNativeVersion = "1.0";

export type GalleryLayerId = string;
export type GalleryFrameId = string;
export type GalleryAssetId = string;
export type GallerySlotKey = string;
export type GalleryModuleKey = string;
export type GalleryStateKey = string;

export interface DesignFileMeta {
  id?: string;
  title: string;
  description?: string | null;
  sourceType: DesignSourceType;
  createdAt?: string;
  updatedAt?: string;
}

export interface DesignFrame {
  id: GalleryFrameId;
  sourceNodeId?: string;
  name: string;
  rootLayerId: GalleryLayerId;
  width: number;
  height: number;
  x?: number;
  y?: number;
  previewAssetId?: GalleryAssetId;
  thumbnailAssetId?: GalleryAssetId;
  thumbnailDataUrl?: string;
  thumbnailUrl?: string;
  board?: { x?: number; y?: number; order?: number };
}

export type DesignLayerType = "frame" | "group" | "text" | "image" | "shape" | "component" | "instance" | "vector" | "slice" | "table" | "form" | "custom";

export interface DesignColorValue {
  r: number;
  g: number;
  b: number;
  a: number;
  hex?: string;
  css?: string;
}

export interface DesignTextLayerData {
  text?: string;
  characters?: string;
  fontFamily?: string;
  fontStyle?: string;
  fontSize?: number;
  fontWeight?: string | number;
  lineHeight?: number | "AUTO";
  letterSpacing?: number;
  textAlignHorizontal?: "left" | "center" | "right" | "justified";
  textAlignVertical?: "top" | "center" | "bottom";
  color?: DesignColorValue;
}

export interface DesignLayer {
  id: GalleryLayerId;
  sourceNodeId?: string;
  frameId: GalleryFrameId;
  parentId?: GalleryLayerId;
  children?: GalleryLayerId[];
  name: string;
  type: DesignLayerType;
  visible: boolean;
  x: number;
  y: number;
  width: number;
  height: number;
  rotation?: number;
  opacity?: number;
  text?: DesignTextLayerData;
  image?: { assetId: GalleryAssetId; alt?: string };
  shape?: { shapeType?: "rectangle" | "ellipse" | "line" | "custom" };
  exportable?: Array<Record<string, unknown>>;
  semantic?: {
    role?: "page" | "header" | "filterBar" | "table" | "pagination" | "form" | "formField" | "button" | "card" | "emptyState" | "custom";
    moduleKey?: GalleryModuleKey;
    stateKey?: GalleryStateKey;
    slotKey?: GallerySlotKey;
    fieldKey?: string;
    actionKey?: string;
    entityKey?: string;
  };
  style?: Record<string, unknown>;
  source?: Record<string, unknown>;
}

export interface DesignAssetRef {
  id: GalleryAssetId;
  kind: DesignAssetKind;
  url: string;
  contentType?: string;
  width?: number;
  height?: number;
  sizeBytes?: number;
  sourceNodeId?: string;
  frameId?: GalleryFrameId;
  metadata?: Record<string, unknown>;
}

export interface GalleryNativeJson {
  version: GalleryNativeVersion;
  file: DesignFileMeta;
  frames: DesignFrame[];
  layers: Record<GalleryLayerId, DesignLayer>;
  assets: Record<GalleryAssetId, DesignAssetRef>;
  tokens?: Record<string, unknown>;
  slots?: Record<GallerySlotKey, { slotKey: GallerySlotKey; layerIds: GalleryLayerId[]; value?: unknown }>;
  modules?: Record<GalleryModuleKey, { moduleKey: GalleryModuleKey; layerIds: GalleryLayerId[]; entityKey?: string }>;
  states?: Record<GalleryStateKey, { stateKey: GalleryStateKey; layerIds: GalleryLayerId[]; stateType?: string }>;
  componentBindings?: Record<GalleryLayerId, { componentKey: string; target?: string; props?: Record<string, unknown> }>;
  restoreHints?: Record<string, unknown>;
  source?: Record<string, unknown>;
}

export interface DesignFile {
  id: string;
  workspace_id: string;
  project_id?: string | null;
  folder_id?: string | null;
  title: string;
  description: string | null;
  source_type: DesignSourceType;
  source_ref: Record<string, unknown>;
  thumbnail_url?: string | null;
  current_revision_id: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface DesignFolder {
  id: string;
  workspace_id: string;
  project_id: string;
  parent_id: string | null;
  name: string;
  position: number;
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface DesignRevision {
  id: string;
  file_id: string;
  workspace_id: string;
  revision_number: number;
  status: DesignRevisionStatus;
  native_json: GalleryNativeJson;
  validation_errors: string[];
  created_by: string | null;
  created_at: string;
}

export type DesignRevisionMetadata = Omit<DesignRevision, "native_json">;

export interface DesignAsset {
  id: string;
  file_id: string;
  revision_id: string | null;
  workspace_id: string;
  asset_key: string;
  kind: DesignAssetKind;
  url: string;
  content_type: string | null;
  size_bytes: number | null;
  metadata: Record<string, unknown>;
  created_by: string | null;
  created_at: string;
}

export interface DesignTemplate {
  id: string;
  workspace_id: string | null;
  key: string;
  name: string;
  description: string | null;
  category: string;
  native_json: GalleryNativeJson;
  slot_schema: Record<string, unknown>;
  metadata: Record<string, unknown>;
  is_system: boolean;
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface DesignCatalogTemplate {
  id: string;
  workspace_id: string;
  library_id: string;
  key: string;
  name: string;
  description?: string | null;
  category: string;
  current_revision_id?: string | null;
  design_revision_id?: string | null;
  template_revision_number?: number | null;
  slot_schema?: Record<string, unknown>;
  design_file_id?: string | null;
  design_file_title?: string | null;
  metadata: Record<string, unknown>;
  created_by?: string | null;
  created_at: string;
  updated_at: string;
}

export interface PublishDesignTemplateRequest {
  revision_id?: string;
  library_key?: string;
  library_name?: string;
  template_key?: string;
  name?: string;
  description?: string | null;
  category?: string;
  slot_schema?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
}

export interface DesignTemplateSlot {
  id: string;
  template_id: string;
  slot_key: string;
  label: string;
  slot_type: DesignTemplateSlotType;
  required: boolean;
  default_value: unknown;
  constraints: Record<string, unknown>;
  description: string | null;
  position: number;
  created_at: string;
}

export interface RequirementCore {
  version: "1.0";
  title: string;
  summary?: string;
  pageType: "saas.filter-table-pagination" | "saas.form-page" | "saas.detail-page";
  tabKey?: string;
  businessGoal?: string;
  targetUsers?: string[];
  entity: { key: string; label: string; description?: string };
  modules?: string[];
  fields?: Array<{ key: string; label: string; type?: string; required?: boolean }>;
  filters?: Array<{ key: string; label: string; type?: string; required?: boolean }>;
  tableColumns?: Array<{ key: string; label: string; fieldKey?: string; width?: number }>;
  formFields?: Array<{ key: string; label: string; fieldKey?: string; type?: string; required?: boolean }>;
  sections?: Array<{ key: string; title: string; fieldKeys?: string[] }>;
  actions?: Array<{ key: string; label: string; intent?: string }>;
  states?: string[];
  constraints?: string[];
  sourceRefs?: Array<{ sourceId?: string; title?: string; url?: string }>;
}

export type TemplateSlotValues = Record<string, unknown>;

export interface GalleryJsonPatchOperation {
  op: "add" | "replace" | "remove";
  path: string;
  value?: unknown;
}

export interface DesignDraft {
  id: string;
  workspace_id: string;
  template_id: string | null;
  catalog_template_id?: string | null;
  template_revision_id?: string | null;
  file_id: string | null;
  revision_id: string | null;
  generated_file_id?: string | null;
  generated_revision_id?: string | null;
  issue_id: string | null;
  title: string;
  requirement_core: RequirementCore;
  slot_values: Record<string, unknown>;
  patch: unknown[];
  status: DesignDraftStatus;
  validation_errors: string[];
  created_by: string | null;
  created_at: string;
  updated_at: string;
  materialized_at?: string | null;
}

export interface CreateDesignDraftRequest {
  catalog_template_id: string;
  template_revision_id?: string;
  issue_id?: string;
  title?: string;
  requirement_core?: Partial<RequirementCore> | Record<string, unknown>;
  slot_values?: TemplateSlotValues;
  patch?: GalleryJsonPatchOperation[];
}

export interface CreateDesignDraftAgentTaskRequest {
  agent_id: string;
  catalog_template_id: string;
  template_revision_id?: string;
  title?: string;
  prompt?: string;
  requirement_core?: Partial<RequirementCore> | Record<string, unknown>;
}

export interface CreateDesignDraftAgentTaskResponse {
  task_id: string;
  status: string;
}

export interface DesignRestoreTask {
  id: string;
  workspace_id: string;
  file_id: string;
  revision_id: string;
  issue_id: string | null;
  agent_task_id: string | null;
  status: DesignRestoreTaskStatus;
  input: Record<string, unknown>;
  result: Record<string, unknown>;
  error: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface ListDesignRestoreTasksResponse {
  tasks: DesignRestoreTask[];
}

export interface DesignRestoreMapping {
  id: string;
  restore_task_id: string;
  workspace_id: string;
  layer_id: GalleryLayerId;
  target_path: string;
  target_kind: DesignRestoreTargetKind;
  confidence: number;
  metadata: Record<string, unknown>;
  created_at: string;
}

export interface DesignSelectionBounds {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface DesignSelectionInput {
  layerIds?: GalleryLayerId[];
  selectionBounds?: DesignSelectionBounds;
  includeIntersectingLayers?: boolean;
}

export interface DesignProjectRulesContext {
  projectId?: string;
  source: DesignProjectRulesSource;
  version?: string;
  updatedAt?: string;
  rules?: unknown;
  techStack?: Record<string, unknown>;
  componentCatalog?: Record<string, unknown>;
  pagesIndex?: Record<string, unknown>;
  designTokens?: Record<string, unknown>;
  restoreChecklist?: Record<string, unknown>;
  generationChecklist?: Record<string, unknown>;
}

export interface DesignFrameSummary {
  id: GalleryFrameId;
  name: string;
  width: number;
  height: number;
  previewAssetId?: GalleryAssetId;
  thumbnailAssetId?: GalleryAssetId;
  layerCount?: number;
}

export interface DesignUsageSummary {
  total?: number;
  byKind?: Record<string, number>;
  items?: Array<Record<string, unknown>>;
}

export interface DesignContext {
  designFileId: string;
  revisionId: string;
  name: string;
  sourceType: DesignSourceType;
  frames: DesignFrameSummary[];
  assetsSummary?: DesignUsageSummary;
  colorsSummary?: DesignUsageSummary;
  textSummary?: DesignUsageSummary;
  annotationsSummary?: DesignUsageSummary;
  nativeJsonAvailable: true;
}

export interface DesignExportableContextItem {
  layerId: GalleryLayerId;
  assetId?: GalleryAssetId;
  url?: string;
  format?: string;
  metadata?: Record<string, unknown>;
}

export interface DesignColorUsage {
  layerId?: GalleryLayerId;
  color: DesignColorValue;
  property?: string;
  tokenKey?: string;
}

export interface DesignTextUsage {
  layerId: GalleryLayerId;
  text?: string;
  fontFamily?: string;
  fontSize?: number;
  fontWeight?: string | number;
}

export interface DesignAnnotation {
  id: string;
  layerId?: GalleryLayerId;
  frameId?: GalleryFrameId;
  kind?: string;
  text?: string;
  metadata?: Record<string, unknown>;
}

export interface DesignFrameContext {
  designFileId: string;
  revisionId: string;
  frame: DesignFrame;
  rootLayerId: GalleryLayerId;
  layers: Record<GalleryLayerId, DesignLayer>;
  assets: Record<GalleryAssetId, DesignAssetRef>;
  exportables?: DesignExportableContextItem[];
  colors?: DesignColorUsage[];
  text?: DesignTextUsage[];
  annotations?: DesignAnnotation[];
  editPatch?: GalleryJsonPatchOperation[];
}

export interface DesignSelectionContextWarning {
  code: string;
  message: string;
  layerId?: GalleryLayerId;
}

export interface DesignSelectionContext {
  designFileId: string;
  revisionId: string;
  frameId: GalleryFrameId;
  input: DesignSelectionInput;
  explicitLayerIds: GalleryLayerId[];
  resolvedLayerIds: GalleryLayerId[];
  layers: Record<GalleryLayerId, DesignLayer>;
  assets: Record<GalleryAssetId, DesignAssetRef>;
  exportables?: DesignExportableContextItem[];
  colors?: DesignColorUsage[];
  text?: DesignTextUsage[];
  bounds?: DesignSelectionBounds;
  warnings?: DesignSelectionContextWarning[];
}

export interface DesignRestoreTaskItemInput {
  itemId?: string;
  order: number;
  designFileId: string;
  revisionId?: string;
  frameId: GalleryFrameId;
  frameName?: string;
  source: DesignRestoreTaskItemSource;
  layerIds?: GalleryLayerId[];
  selectionBounds?: DesignSelectionBounds;
  moduleKey?: GalleryModuleKey;
  stateKey?: GalleryStateKey;
  slotKey?: GallerySlotKey;
  note?: string;
}

export interface DesignRestoreTaskInputV1 {
  version: "1.0";
  projectId?: string;
  folderId?: string;
  sourceIssueId?: string;
  targetRoute?: string;
  targetFiles?: string[];
  purpose: DesignRestoreTaskPurpose;
  items: DesignRestoreTaskItemInput[];
}

export interface CreateDesignRestoreTaskRequest {
  file_id: string;
  revision_id?: string;
  issue_id?: string;
  input: DesignRestoreTaskInputV1;
}

export interface DesignLayerLightweightEditRequest {
  revision_id?: string;
  text?: string;
  semantic?: Partial<Record<"role" | "moduleKey" | "stateKey" | "slotKey", string>>;
}

export interface DesignRestoreTaskItemContextResponse {
  task: DesignRestoreTask;
  item: DesignRestoreTaskItemInput;
  context: DesignFrameContext | DesignSelectionContext;
}

export interface DispatchDesignRestoreTaskRequest {
  agent_id: string;
  issue_id?: string;
  prompt?: string;
}

export interface DispatchDesignRestoreTaskResponse {
  task: DesignRestoreTask;
  agent_task_id: string;
}

export type DesignAgentContextSource =
  | { kind: "design_file"; designFileId: string; revisionId?: string }
  | { kind: "frame"; designFileId: string; frameId: GalleryFrameId; revisionId?: string }
  | { kind: "selection"; designFileId: string; frameId: GalleryFrameId; selection: DesignSelectionInput; revisionId?: string }
  | { kind: "restore_task"; restoreTaskId: string }
  | { kind: "design_draft"; designDraftId: string };

export interface DesignAgentContext {
  workspaceId: string;
  projectId?: string;
  folderId?: string;
  source: DesignAgentContextSource;
  design: DesignContext | DesignFrameContext | DesignSelectionContext | DesignRestoreTaskInputV1;
  projectRules?: DesignProjectRulesContext;
  requirement?: RequirementCore;
  constraints?: Record<string, unknown>;
  provenance?: Record<string, unknown>;
}

export interface DesignFileDetailResponse {
  file: DesignFile;
  current_revision: DesignRevision | null;
}

export interface DesignDraftMaterializeResponse {
  draft: DesignDraft;
  design_file: DesignFileDetailResponse;
}

export interface ListDesignFilesResponse {
  design_files: DesignFile[];
  total: number;
}

export interface ListDesignFoldersResponse {
  folders: DesignFolder[];
  total: number;
}

export interface CreateDesignFolderRequest {
  project_id: string;
  name: string;
}

export interface ListDesignRevisionsResponse {
  revisions: DesignRevisionMetadata[];
  total: number;
}

export interface ListDesignTemplatesResponse {
  templates: DesignCatalogTemplate[];
  total: number;
}

export interface ListDesignDraftsResponse {
  drafts: DesignDraft[];
  total: number;
}

export interface CreateDesignFileRequest {
  title: string;
  description?: string | null;
  project_id?: string | null;
  folder_id?: string | null;
  source_type?: DesignSourceType;
  source_ref?: Record<string, unknown>;
  native_json: GalleryNativeJson;
}

export interface FigmaImportConnection {
  code: string;
  expires_at: string;
}

export interface FigmaPluginAuthSession {
  session_id: string;
  user_code: string;
  authorize_url: string;
  expires_at: string;
}

export interface FigmaPluginAuthStatus {
  status: "pending" | "approved" | "expired" | "consumed" | "denied";
  token?: string;
  workspace_id?: string;
  expires_at?: string;
}

import { z } from "zod";

export const GalleryNativeVersionSchema = z.literal("1.0");

const JsonRecordSchema = z.record(z.string(), z.unknown());

export const DesignSourceTypeSchema = z.enum(["upload", "ai_generated", "template", "import"]);

export const RequirementPageTypeSchema = z.enum([
  "saas.filter-table-pagination",
  "saas.form-page",
  "saas.detail-page",
]);

const KeyLabelSchema = z.object({
  key: z.string().min(1),
  label: z.string().min(1),
}).loose();

export const RequirementCoreSchema = z.object({
  version: GalleryNativeVersionSchema,
  title: z.string().min(1),
  summary: z.string().optional(),
  pageType: RequirementPageTypeSchema,
  tabKey: z.string().optional(),
  businessGoal: z.string().optional(),
  targetUsers: z.array(z.string().min(1)).optional(),
  entity: KeyLabelSchema,
  fields: z.array(KeyLabelSchema).default([]),
  filters: z.array(KeyLabelSchema).optional(),
  tableColumns: z.array(KeyLabelSchema).optional(),
  formFields: z.array(KeyLabelSchema).optional(),
  sections: z.array(KeyLabelSchema).optional(),
  actions: z.array(KeyLabelSchema).optional(),
  states: z.array(z.string().min(1)).optional(),
  constraints: z.array(z.string().min(1)).optional(),
  sourceRefs: z.array(JsonRecordSchema).optional(),
}).loose();

export const SlotValuesSchema = z.record(z.string().min(1), z.unknown());

export const JsonPatchOperationSchema = z.object({
  op: z.enum(["add", "replace", "remove"]),
  path: z.string().min(1).startsWith("/"),
  value: z.unknown().optional(),
}).loose();

export const JsonPatchOperationsSchema = z.array(JsonPatchOperationSchema);

const FORBIDDEN_PATCH_PATH_SEGMENTS = new Set(["x", "y", "width", "height", "children"]);

export const DesignFrameSchema = z.object({
  id: z.string().min(1),
  sourceNodeId: z.string().optional(),
  name: z.string(),
  rootLayerId: z.string().min(1),
  width: z.number().nonnegative(),
  height: z.number().nonnegative(),
  x: z.number().optional(),
  y: z.number().optional(),
  previewAssetId: z.string().min(1).optional(),
  thumbnailAssetId: z.string().min(1).optional(),
  thumbnailDataUrl: z.string().optional(),
  thumbnailUrl: z.string().optional(),
  board: z.object({ x: z.number().optional(), y: z.number().optional(), order: z.number().optional() }).loose().optional(),
}).loose();

export const DesignAssetRefSchema = z.object({
  id: z.string().min(1),
  kind: z.enum(["frame_preview", "frame_thumbnail", "image", "slice", "thumbnail", "source", "other"]),
  url: z.string().min(1),
  contentType: z.string().optional(),
  width: z.number().nonnegative().optional(),
  height: z.number().nonnegative().optional(),
  sizeBytes: z.number().nonnegative().optional(),
  sourceNodeId: z.string().optional(),
  frameId: z.string().optional(),
  metadata: JsonRecordSchema.optional(),
}).loose();

const ColorValueSchema = z.object({
  r: z.number(),
  g: z.number(),
  b: z.number(),
  a: z.number(),
  hex: z.string().optional(),
  css: z.string().optional(),
}).loose();

const TextLayerDataSchema = z.object({
  text: z.string().optional(),
  characters: z.string().optional(),
  fontFamily: z.string().optional(),
  fontStyle: z.string().optional(),
  fontSize: z.number().optional(),
  fontWeight: z.union([z.string(), z.number()]).optional(),
  lineHeight: z.union([z.number(), z.literal("AUTO")]).optional(),
  letterSpacing: z.number().optional(),
  textAlignHorizontal: z.enum(["left", "center", "right", "justified"]).optional(),
  textAlignVertical: z.enum(["top", "center", "bottom"]).optional(),
  color: ColorValueSchema.optional(),
}).loose();

export const DesignLayerSchema = z.object({
  id: z.string().min(1),
  sourceNodeId: z.string().optional(),
  frameId: z.string().min(1),
  parentId: z.string().min(1).optional(),
  children: z.array(z.string().min(1)).optional(),
  name: z.string(),
  type: z.string().min(1),
  visible: z.boolean(),
  x: z.number(),
  y: z.number(),
  width: z.number().nonnegative(),
  height: z.number().nonnegative(),
  rotation: z.number().optional(),
  opacity: z.number().optional(),
  text: TextLayerDataSchema.optional(),
  image: z.object({ assetId: z.string().min(1), alt: z.string().optional() }).loose().optional(),
  shape: z.object({ shapeType: z.string().optional() }).loose().optional(),
  exportable: z.array(JsonRecordSchema).optional(),
  semantic: JsonRecordSchema.optional(),
  style: JsonRecordSchema.optional(),
  source: JsonRecordSchema.optional(),
}).loose();

export const GalleryNativeJsonSchema = z.object({
  version: GalleryNativeVersionSchema,
  file: z.object({
    id: z.string().optional(),
    title: z.string().min(1),
    description: z.string().nullable().optional(),
    sourceType: DesignSourceTypeSchema,
    createdAt: z.string().optional(),
    updatedAt: z.string().optional(),
  }).loose(),
  frames: z.array(DesignFrameSchema).min(1),
  layers: z.record(z.string(), DesignLayerSchema),
  assets: z.record(z.string(), DesignAssetRefSchema).default({}),
  tokens: JsonRecordSchema.optional(),
  slots: z.record(z.string(), z.object({
    slotKey: z.string().min(1),
    layerIds: z.array(z.string().min(1)),
    value: z.unknown().optional(),
  }).loose()).optional(),
  modules: z.record(z.string(), z.object({
    moduleKey: z.string().min(1),
    layerIds: z.array(z.string().min(1)),
    entityKey: z.string().optional(),
  }).loose()).optional(),
  states: z.record(z.string(), z.object({
    stateKey: z.string().min(1),
    layerIds: z.array(z.string().min(1)),
    stateType: z.string().optional(),
  }).loose()).optional(),
  componentBindings: z.record(z.string(), z.object({
    componentKey: z.string().min(1),
    target: z.string().optional(),
    props: JsonRecordSchema.optional(),
  }).loose()).optional(),
  restoreHints: JsonRecordSchema.optional(),
  source: JsonRecordSchema.optional(),
}).loose();

export type GalleryNativeJsonInput = z.infer<typeof GalleryNativeJsonSchema>;
export type RequirementCoreInput = z.infer<typeof RequirementCoreSchema>;
export type JsonPatchOperationInput = z.infer<typeof JsonPatchOperationSchema>;

export interface GalleryNativeValidationResult {
  valid: boolean;
  errors: string[];
}

export function validateGalleryNativeJson(value: unknown): GalleryNativeValidationResult {
  const parsed = GalleryNativeJsonSchema.safeParse(value);
  if (!parsed.success) {
    return { valid: false, errors: parsed.error.issues.map((issue) => issue.message) };
  }

  return validateGalleryNativeReferences(parsed.data);
}

export function validateRequirementCore(value: unknown): GalleryNativeValidationResult {
  const parsed = RequirementCoreSchema.safeParse(value);
  if (!parsed.success) {
    return { valid: false, errors: parsed.error.issues.map((issue) => issue.message) };
  }
  return { valid: true, errors: [] };
}

export function validateSlotValues(value: unknown): GalleryNativeValidationResult {
  const parsed = SlotValuesSchema.safeParse(value);
  if (!parsed.success) {
    return { valid: false, errors: parsed.error.issues.map((issue) => issue.message) };
  }
  return { valid: true, errors: [] };
}

export function validateJsonPatchOperations(value: unknown): GalleryNativeValidationResult {
  const parsed = JsonPatchOperationsSchema.safeParse(value);
  if (!parsed.success) {
    return { valid: false, errors: parsed.error.issues.map((issue) => issue.message) };
  }

  const errors: string[] = [];
  for (const operation of parsed.data) {
    const segments = operation.path.split("/").filter(Boolean);
    if (segments.some((segment) => FORBIDDEN_PATCH_PATH_SEGMENTS.has(segment))) {
      errors.push(`Patch path ${operation.path} changes layout or tree structure and is not allowed in MVP`);
    }
  }
  return { valid: errors.length === 0, errors };
}

export function validateGalleryNativeReferences(value: GalleryNativeJsonInput): GalleryNativeValidationResult {
  const errors: string[] = [];
  const frameIds = new Set(value.frames.map((frame) => frame.id));
  const layerIds = new Set(Object.keys(value.layers));
  const assetIds = new Set(Object.keys(value.assets));

  for (const frame of value.frames) {
    if (!layerIds.has(frame.rootLayerId)) errors.push(`Frame ${frame.id} references missing root layer ${frame.rootLayerId}`);
    if (frame.previewAssetId && !assetIds.has(frame.previewAssetId)) errors.push(`Frame ${frame.id} references missing preview asset ${frame.previewAssetId}`);
    if (frame.thumbnailAssetId && !assetIds.has(frame.thumbnailAssetId)) errors.push(`Frame ${frame.id} references missing thumbnail asset ${frame.thumbnailAssetId}`);
  }

  for (const [id, layer] of Object.entries(value.layers)) {
    if (layer.id !== id) errors.push(`Layer map key ${id} does not match layer.id ${layer.id}`);
    if (!frameIds.has(layer.frameId)) errors.push(`Layer ${id} references missing frame ${layer.frameId}`);
    if (layer.parentId && !layerIds.has(layer.parentId)) errors.push(`Layer ${id} references missing parent ${layer.parentId}`);
    for (const childId of layer.children ?? []) {
      const child = value.layers[childId];
      if (!child) {
        errors.push(`Layer ${id} references missing child ${childId}`);
      } else if (child.parentId !== id) {
        errors.push(`Layer ${id} child ${childId} has parent ${child.parentId ?? "<none>"}`);
      }
    }
    if (layer.image?.assetId && !assetIds.has(layer.image.assetId)) {
      errors.push(`Layer ${id} references missing asset ${layer.image.assetId}`);
    }
  }

  for (const [slotKey, slot] of Object.entries(value.slots ?? {})) {
    for (const layerId of slot.layerIds) {
      if (!layerIds.has(layerId)) errors.push(`Slot ${slotKey} references missing layer ${layerId}`);
    }
  }

  for (const layerId of Object.keys(value.componentBindings ?? {})) {
    if (!layerIds.has(layerId)) errors.push(`Component binding references missing layer ${layerId}`);
  }

  return { valid: errors.length === 0, errors };
}

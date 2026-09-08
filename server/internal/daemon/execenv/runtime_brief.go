package execenv

// RenderRuntimeBrief returns the platform workflow without modifying the
// checkout. Managed primary repositories deliver it in the task message so
// repository-owned instructions cannot accidentally acquire a committed brief.
func RenderRuntimeBrief(provider string, ctx TaskContextForEnv) string {
	return buildMetaSkillContent(provider, ctx)
}

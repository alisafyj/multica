/**
 * Project selection shared by every testing list surface.
 *
 * The selected project is a persisted preference, so it outlives the project it
 * names. Resolving it against the live list keeps the picker and the query
 * pointed at the same project: without this, a deleted project leaves the
 * picker rendering the first live option (a `<select>` with no matching
 * `value`) while every query still carries the dead id, and the list reads as
 * permanently empty with nothing on screen explaining why.
 */
export function resolveSelectedProjectId(
  projects: ReadonlyArray<{ id: string }>,
  persistedId: string | null,
): string {
  if (persistedId !== null && projects.some((project) => project.id === persistedId)) {
    return persistedId;
  }
  return projects[0]?.id ?? "";
}

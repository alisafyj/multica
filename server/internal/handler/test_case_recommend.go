package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Change-based regression selection: which cases claim the files a change
// touched. A case claims files through the `path_globs` of its repo bindings
// (test_case_repo), so the recommendation is a pure function of the bindings
// and the changed paths — no history, no model. The caller (a CI job, the
// CLI on a branch, the case library) brings the paths.

const maxRecommendPaths = 2000

type RecommendTestCasesRequest struct {
	ProjectID string   `json:"project_id"`
	Paths     []string `json:"paths"`
	// Repo narrows the claims to one binding alias when the paths come from
	// a single repository of a multi-repo project.
	Repo string `json:"repo,omitempty"`
}

type TestCaseRecommendationMatch struct {
	Alias string   `json:"alias"`
	Role  string   `json:"role"`
	Glob  string   `json:"glob"`
	Paths []string `json:"paths"`
}

type TestCaseRecommendation struct {
	TestCase TestCaseResponse              `json:"test_case"`
	Matches  []TestCaseRecommendationMatch `json:"matches"`
	// PathCount is the number of distinct changed paths the case claims; the
	// list is ranked by it, then by case number.
	PathCount int `json:"path_count"`
}

type RecommendTestCasesResponse struct {
	Cases          []TestCaseRecommendation `json:"cases"`
	UnmatchedPaths []string                 `json:"unmatched_paths"`
	Total          int                      `json:"total"`
}

// RecommendTestCases handles POST /api/test-cases/recommend.
func (h *Handler) RecommendTestCases(w http.ResponseWriter, r *http.Request) {
	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return
	}
	var req RecommendTestCasesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	projectUUID, ok := parseUUIDOrBadRequest(w, req.ProjectID, "project_id")
	if !ok {
		return
	}
	paths := normalizeChangedPaths(req.Paths)
	if len(paths) == 0 {
		writeError(w, http.StatusBadRequest, "paths is required")
		return
	}
	if len(paths) > maxRecommendPaths {
		writeError(w, http.StatusBadRequest, "too many paths")
		return
	}

	bindings, err := h.Queries.ListTestCaseReposForProject(r.Context(), db.ListTestCaseReposForProjectParams{
		ProjectID:   projectUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		slog.Error("list test case repos for project failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to load test case bindings")
		return
	}
	claims := recommendTestCases(bindings, paths, strings.TrimSpace(req.Repo))

	resp := RecommendTestCasesResponse{Cases: []TestCaseRecommendation{}, UnmatchedPaths: claims.unmatched}
	if len(claims.byCase) > 0 {
		testCases, err := h.Queries.ListTestCases(r.Context(), db.ListTestCasesParams{WorkspaceID: wsUUID, ProjectID: projectUUID})
		if err != nil {
			slog.Error("list test cases for recommendation failed", append(logger.RequestAttrs(r), "error", err)...)
			writeError(w, http.StatusInternalServerError, "failed to list test cases")
			return
		}
		reposByCase := map[string][]db.TestCaseRepo{}
		for _, b := range bindings {
			reposByCase[uuidToString(b.TestCaseID)] = append(reposByCase[uuidToString(b.TestCaseID)], bindingRepo(b))
		}
		for _, tc := range testCases {
			claim, ok := claims.byCase[uuidToString(tc.ID)]
			if !ok {
				continue
			}
			resp.Cases = append(resp.Cases, TestCaseRecommendation{
				TestCase:  testCaseToResponse(tc, reposByCase[uuidToString(tc.ID)]),
				Matches:   claim.matches,
				PathCount: len(claim.paths),
			})
		}
		sort.SliceStable(resp.Cases, func(i, j int) bool {
			if resp.Cases[i].PathCount != resp.Cases[j].PathCount {
				return resp.Cases[i].PathCount > resp.Cases[j].PathCount
			}
			return resp.Cases[i].TestCase.CaseNumber < resp.Cases[j].TestCase.CaseNumber
		})
	}
	resp.Total = len(resp.Cases)
	writeJSON(w, http.StatusOK, resp)
}

func bindingRepo(b db.ListTestCaseReposForProjectRow) db.TestCaseRepo {
	return db.TestCaseRepo{
		TestCaseID:        b.TestCaseID,
		WorkspaceID:       b.WorkspaceID,
		ProjectResourceID: b.ProjectResourceID,
		Alias:             b.Alias,
		Role:              b.Role,
		PathGlobs:         b.PathGlobs,
		CreatedAt:         b.CreatedAt,
	}
}

// normalizeChangedPaths trims, strips a leading "./" or "/", drops empties and
// duplicates, and keeps the caller's order otherwise.
func normalizeChangedPaths(raw []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "./")
		p = strings.TrimLeft(p, "/")
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

type testCaseClaim struct {
	matches []TestCaseRecommendationMatch
	paths   map[string]bool
}

type testCaseClaims struct {
	byCase    map[string]*testCaseClaim
	unmatched []string
}

// recommendTestCases matches every changed path against every binding glob.
// A binding without globs claims nothing: "this case touches that repo" is
// not "this case covers every file in it".
func recommendTestCases(bindings []db.ListTestCaseReposForProjectRow, paths []string, repo string) testCaseClaims {
	claims := testCaseClaims{byCase: map[string]*testCaseClaim{}}
	claimed := map[string]bool{}
	for _, b := range bindings {
		if repo != "" && b.Alias != repo {
			continue
		}
		var globs []string
		if len(b.PathGlobs) > 0 {
			if err := json.Unmarshal(b.PathGlobs, &globs); err != nil {
				slog.Warn("test case repo path_globs failed to decode", "test_case_id", uuidToString(b.TestCaseID), "error", err)
				continue
			}
		}
		for _, glob := range globs {
			var hit []string
			for _, p := range paths {
				if matchPathGlob(glob, p) {
					hit = append(hit, p)
				}
			}
			if len(hit) == 0 {
				continue
			}
			key := uuidToString(b.TestCaseID)
			claim := claims.byCase[key]
			if claim == nil {
				claim = &testCaseClaim{paths: map[string]bool{}}
				claims.byCase[key] = claim
			}
			claim.matches = append(claim.matches, TestCaseRecommendationMatch{Alias: b.Alias, Role: b.Role, Glob: glob, Paths: hit})
			for _, p := range hit {
				claim.paths[p] = true
				claimed[p] = true
			}
		}
	}
	claims.unmatched = []string{}
	for _, p := range paths {
		if !claimed[p] {
			claims.unmatched = append(claims.unmatched, p)
		}
	}
	return claims
}

var (
	globRegexpMu    sync.Mutex
	globRegexpCache = map[string]*regexp.Regexp{}
)

// matchPathGlob reports whether a repo-relative path matches one binding
// glob, with the semantics the bindings were written in (gitignore /
// doublestar): `**` spans directories, `*` and `?` stop at a slash, `{a,b}`
// alternates, `[...]` is a character class, a pattern without a slash matches
// at any depth (`*.go` claims every Go file), a trailing `/` or a plain
// directory name claims everything under it.
func matchPathGlob(pattern, path string) bool {
	pattern = strings.TrimSpace(pattern)
	pattern = strings.TrimPrefix(pattern, "./")
	pattern = strings.TrimLeft(pattern, "/")
	if pattern == "" {
		return false
	}
	if !strings.ContainsAny(pattern, "*?[{") {
		dir := strings.TrimSuffix(pattern, "/")
		return path == dir || strings.HasPrefix(path, dir+"/")
	}
	re := compilePathGlob(pattern)
	if re == nil {
		return false
	}
	return re.MatchString(path)
}

func compilePathGlob(pattern string) *regexp.Regexp {
	globRegexpMu.Lock()
	defer globRegexpMu.Unlock()
	if re, ok := globRegexpCache[pattern]; ok {
		return re
	}
	src := pattern
	if strings.HasSuffix(src, "/") {
		src += "**"
	}
	if !strings.Contains(src, "/") {
		src = "**/" + src
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch c {
		case '*':
			if i+1 < len(src) && src[i+1] == '*' {
				if i+2 < len(src) && src[i+2] == '/' {
					b.WriteString("(?:.*/)?")
					i += 2
				} else {
					b.WriteString(".*")
					i++
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '{':
			end := strings.IndexByte(src[i:], '}')
			if end < 0 {
				b.WriteString(regexp.QuoteMeta("{"))
				continue
			}
			alts := strings.Split(src[i+1:i+end], ",")
			for j := range alts {
				alts[j] = regexp.QuoteMeta(strings.TrimSpace(alts[j]))
			}
			b.WriteString("(?:" + strings.Join(alts, "|") + ")")
			i += end
		case '[':
			end := strings.IndexByte(src[i:], ']')
			if end < 0 {
				b.WriteString(regexp.QuoteMeta("["))
				continue
			}
			class := src[i+1 : i+end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteString("[" + strings.ReplaceAll(class, `\`, `\\`) + "]")
			i += end
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		slog.Warn("test case path glob does not compile", "glob", pattern, "error", err)
		globRegexpCache[pattern] = nil
		return nil
	}
	if len(globRegexpCache) > 4096 {
		globRegexpCache = map[string]*regexp.Regexp{}
	}
	globRegexpCache[pattern] = re
	return re
}

---
name: multica-running-tests
description: "Use when an agent is assigned to execute a Multica test run — discovering capabilities, recording results, uploading evidence, and opening defects. Not for writing, reviewing, or generating test cases."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Running Tests

This skill covers what a Multica test run is, how to drive it from the CLI,
and what the platform enforces. Every claim is pinned in
`references/running-tests-source-map.md`; when behavior differs from this
document, the source map is where to re-check it.

## 1. Discover capabilities first, always

Before touching any test case result, run:

    multica test run get <run-id> --output json
    multica test capability list --run <run-id> --output json

`run get` returns the run's title, status, and the frozen case list.
`capability list` returns the `capability_binding` the platform resolved at
dispatch time. That binding looks like:

```json
{
  "run_id": "<run-id>",
  "capability_binding": {
    "daemon_id": "<daemon-id>",
    "runtime_id": "<runtime-id>",
    "resolved": {
      "android": "pixel-9-key",
      "browser": "chrome-desktop-key"
    }
  }
}
```

**Only the `capability_key` values in `resolved` are valid for this run.**
The platform freezes the binding at dispatch time — it is not re-evaluated at
runtime. Do not use any capability the binding did not return.

## 2. Never probe the host for devices or browsers

Do not run `adb devices`, `xcrun simctl list`, `google-chrome --version`,
`which chromium`, or any other tool that looks for capabilities on the host.
The platform already resolved which device or browser is bound to this run. If
the capability is not in the `resolved` map, that kind is unavailable for this
run — use `blocked` as the result, not `failed`.

Probing the host finds ambient devices that are NOT bound to your run, produces
false positives, and violates the capability-isolation contract that lets
multiple runs share one daemon safely.

## 3. `blocked` is not `failed`

| Result | Meaning |
| --- | --- |
| `passed` | Case executed; acceptance criterion met. |
| `failed` | Case executed; criterion not met. |
| `blocked` | Case could not be executed (missing device, missing credential, environment not ready). |
| `skipped` | Case excluded from this run (not applicable). |

Set `blocked` when the required capability is absent or the prerequisite
environment is not available. Never set `failed` when the test could not run.

## 4. Execute the frozen snapshot

A run executes the frozen case snapshot captured at dispatch, not the live case
record. The snapshot includes the `steps` array and the `required_capabilities`
list. Treat the snapshot as immutable; do not look up the live case mid-run.

## 5. Record results as you go

After each test case completes, record the result immediately — do not batch at
the end.

### Set result

    multica test result set <run-case-id> --result passed|failed|blocked|skipped [--note "…"] [--step-results <json>]

`--step-results` is an optional JSON array of per-step outcomes:

```json
[{"index": 1, "result": "passed"}, {"index": 2, "result": "failed", "note": "Button not found"}]
```

`--note` maps to the `notes` field — use it for a short failure summary or
blocking reason.

### Upload evidence

Upload evidence for every `failed` or `blocked` result:

    multica test evidence add <run-case-id> --file ./path/to/screenshot.png --kind screenshot

`--kind` values: `screenshot`, `video`, `log`, `other`.

Evidence upload uses multipart form with fields `file`, `test_run_case_id`, and
`kind`. The agent's task token authenticates the request automatically.

### Open a defect

When a case `failed` and the failure represents a product defect:

    multica test defect open <run-case-id> --title "Short reproduction title" [--note "…"]

This creates a linked issue in the workspace and attaches it to the test run
case. One defect per reproduction scenario; do not open a defect for `blocked`
results.

## 6. Start a pending run

If the run is in `pending` status, start it before recording results:

    multica test run start <run-id>

This transitions the run from `pending` to `running`. The agent normally
receives a run that has already been dispatched to `running` status; call
`run start` only if status is `pending`.

## 7. Test plans (informational)

Test plans group cases into a named release scope. You can read them but you
do not modify plans during a run:

    multica test plan list --output json
    multica test plan get <plan-id> --output json

## Hard rules (never violate)

- **Capability boundary**: only use capability keys from `capability list`. Do
  not use `adb`, `xcrun simctl`, `which chromium`, or any host probe.
- **Blocked ≠ failed**: if you cannot run a case, set `blocked` with a note
  explaining why. `failed` means the test ran but the product did not behave
  correctly.
- **Record immediately**: set the result before moving to the next case. Do not
  accumulate results and flush them at the end.
- **Evidence on failure**: upload at least one screenshot or log for every
  `failed` or `blocked` case where the environment permits it.
- **Frozen snapshot is authoritative**: do not re-read the live test case
  record mid-run. The run has the snapshot it was dispatched with.
- **One defect per scenario**: do not open duplicate defect issues for the
  same reproduction path.

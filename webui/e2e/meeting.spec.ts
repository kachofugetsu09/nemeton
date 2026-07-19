import { expect, test, type Page } from "@playwright/test"

const participants = [
  { id: "p1", seat: "designer-1", role: "designer", provider: "codex", model: "gpt-5.4-mini", provider_options: { reasoning_effort: "medium" }, status: "completed" },
  { id: "p2", seat: "designer-2", role: "designer", provider: "opencode", model: "opencode-go/deepseek-v4-pro", provider_options: { variant: "high" }, status: "running" },
  { id: "pr", seat: "recorder", role: "recorder", provider: "opencode", model: "opencode-go/deepseek-v4-pro", provider_options: { variant: "high" }, status: "waiting" },
]

async function installMeeting(page: Page, id: string, snapshot: object, bodies: Record<string, string>) {
  await page.route(`**/v1/meetings/${id}`, (route) => route.fulfill({ json: { meeting: snapshot } }))
  await page.route(`**/v1/meetings/${id}/contents/*`, (route) => {
    const contentID = route.request().url().split("/").at(-1) || ""
    return route.fulfill({ json: { content: { body: bodies[contentID] } } })
  })
  await page.addInitScript((meetingID) => localStorage.setItem("nemeton.meeting", meetingID), id)
}

test("provider can be dragged into the roster without mobile overflow", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.route("**/v1/projects/open", (route) => route.fulfill({
    json: { project: { project: { id: "project" }, binding: { display_path: "/tmp/project", canonical_root: "/tmp/project" } } },
  }))
  await page.goto("/")
  await page.getByLabel("Repository 绝对路径").fill("/tmp/project")
  await page.getByRole("button", { name: "打开项目" }).click()
  await page.getByRole("button", { name: /参与者与模型/ }).click()

  const source = page.locator(".provider-card").first()
  const target = page.locator(".participant-dropzone")
  const from = await source.boundingBox()
  const to = await target.boundingBox()
  expect(from).not.toBeNull()
  expect(to).not.toBeNull()
  await page.mouse.move(from!.x + from!.width / 2, from!.y + from!.height / 2)
  await page.mouse.down()
  await page.mouse.move(from!.x + from!.width / 2 + 20, from!.y + from!.height / 2 + 20, { steps: 10 })
  await page.mouse.move(to!.x + to!.width / 2, to!.y + to!.height / 2, { steps: 20 })
  await expect(target).toHaveAttribute("data-over", "true")
  await page.mouse.up()
  await expect(page.locator(".participant-row")).toHaveCount(5)

  await page.setViewportSize({ width: 390, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
})

test("meeting keeps natural-language discussion central and proposals separate", async ({ page }) => {
  const contents = [
    { id: "proposal", participant_id: "p1", kind: "proposal", cycle: 1, round: 0, content_digest: "d1", refs: [] },
    { id: "position", participant_id: "p2", kind: "position", cycle: 1, round: 1, content_digest: "d2", refs: [] },
  ]
  const snapshot = {
    meeting: { id: "room", project_id: "project", title: "恢复所有权", brief: "明确断线恢复的唯一事实来源。", status: "deliberating", cycle: 1, current_round: 1, result_digest: "", protocol_version: 2 },
    participants,
    contents,
    candidates: [],
    stream_version: 12,
  }
  await installMeeting(page, "room", snapshot, {
    proposal: JSON.stringify({ summary: "Daemon 持有唯一 committed sequence", candidate_items: [{ statement: "浏览器只提交观察游标。" }] }),
    position: JSON.stringify({ rationale: "@designer-1 同意。客户端不维护独立领域 reducer。" }),
  })
  await page.goto("/")

  await expect(page.getByLabel("会议讨论")).toContainText("@designer-1 同意")
  await expect(page.locator(".proposal-panel-live")).toContainText("Daemon 持有唯一 committed sequence")
  await expect(page.getByLabel("会议讨论")).not.toContainText("Daemon 持有唯一 committed sequence")
  await expect(page.getByText("Verifier", { exact: true })).toHaveCount(0)
})

test("result review is option-first and keeps prose optional", async ({ page }) => {
  const resultID = "result"
  const snapshot = {
    meeting: { id: "review", project_id: "project", title: "恢复所有权", brief: "明确断线恢复的唯一事实来源。", status: "awaiting_user_review", cycle: 1, current_round: 2, result_digest: "digest", result_content_id: resultID, protocol_version: 2 },
    participants,
    contents: [{ id: resultID, participant_id: "pr", kind: "meeting_result", cycle: 1, round: 2, content_digest: "digest", refs: [] }],
    candidates: [
      { id: "c1", kind: "invariant", statement: "Committed event 一旦发布即不可变。", source_refs: [resultID], status: "proposed", design_disposition: "pending", context_disposition: "none", rationale: "稳定 replay。" },
      { id: "c2", kind: "acceptance", statement: "Daemon restart 后不重复事件。", source_refs: [resultID], status: "proposed", design_disposition: "pending", context_disposition: "none", rationale: "可执行验收。" },
    ],
    stream_version: 24,
  }
  await installMeeting(page, "review", snapshot, {
    [resultID]: JSON.stringify({ synthesis: "Event log 是唯一事实，projection 可重建，浏览器只持有观察游标。" }),
  })
  await page.goto("/")

  await expect(page.locator(".candidate-review")).toHaveCount(2)
  await expect(page.getByText("接受并长期保留")).toHaveCount(2)
  await expect(page.getByText("接受但仅本方案")).toHaveCount(2)
  await expect(page.getByPlaceholder(/由 Recorder 自己判断/)).toHaveValue("")
  await expect(page.getByRole("button", { name: "批准完整方案" })).toBeEnabled()
  await page.getByText("反对", { exact: true }).first().click()
  await expect(page.getByRole("button", { name: "批准完整方案" })).toBeDisabled()
  await expect(page.getByRole("button", { name: "继续处理" })).toBeEnabled()
})

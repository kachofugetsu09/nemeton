import { useCallback, useEffect, useMemo, useState } from "react"
import { ChevronDown, CircleAlert, MessagesSquare, Plus, Radio } from "lucide-react"
import { ParticipantBuilder } from "@/features/meetings/participant-builder"
import { MeetingRoomLive } from "@/features/meetings/meeting-room-live"
import { ResultReview, type ReviewChoice } from "@/features/meetings/result-review"
import {
  createMeeting,
  fetchHandoff,
  hydrateContents,
  inspectMeeting,
  openProject,
  reviewMeeting,
  startMeeting,
  watchMeeting,
  type Handoff,
  type MeetingSnapshot,
  type ParticipantInput,
  type ProjectSnapshot,
} from "@/lib/api"

const initialParticipants: ParticipantInput[] = [
  { id: crypto.randomUUID(), seat: "designer-1", role: "designer", provider: "codex", model: "gpt-5.4-mini", provider_options: { reasoning_effort: "medium" } },
  { id: crypto.randomUUID(), seat: "designer-2", role: "designer", provider: "opencode", model: "opencode-go/deepseek-v4-pro", provider_options: { variant: "high" } },
  { id: crypto.randomUUID(), seat: "designer-3", role: "designer", provider: "codex", model: "gpt-5.4-mini", provider_options: { reasoning_effort: "medium" } },
  { id: crypto.randomUUID(), seat: "recorder", role: "recorder", provider: "opencode", model: "opencode-go/deepseek-v4-pro", provider_options: { variant: "high" } },
]

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    draft: "草稿", preparing: "准备工作区", sealed_proposals: "形成提案", revealed: "提案已公开",
    deliberating: "讨论中", recording: "Recorder 整理中", awaiting_user_review: "等待你的审阅",
    recorder_reviewing: "Recorder 判断中", reconvening: "准备下一轮", concluded: "已批准", failed: "失败",
  }
  return labels[status] || status
}

export default function App() {
  const [project, setProject] = useState<ProjectSnapshot>()
  const [snapshot, setSnapshot] = useState<MeetingSnapshot>()
  const [handoff, setHandoff] = useState<Handoff>()
  const [repoPath, setRepoPath] = useState(localStorage.getItem("nemeton.repo") || "")
  const [title, setTitle] = useState("")
  const [brief, setBrief] = useState("")
  const [participants, setParticipants] = useState(initialParticipants)
  const [configOpen, setConfigOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  const refresh = useCallback(async (meetingId: string) => {
    const next = await hydrateContents(await inspectMeeting(meetingId))
    setSnapshot(next)
    localStorage.setItem("nemeton.meeting", meetingId)
    if (next.meeting.status === "concluded") setHandoff(await fetchHandoff(meetingId))
  }, [])

  useEffect(() => {
    const meetingId = localStorage.getItem("nemeton.meeting")
    if (meetingId) refresh(meetingId).catch(() => localStorage.removeItem("nemeton.meeting"))
  }, [refresh])

  useEffect(() => {
    if (!snapshot || snapshot.meeting.status === "concluded" || snapshot.meeting.status === "failed") return
    const socket = watchMeeting(snapshot.meeting.id, snapshot.stream_version, () => {
      refresh(snapshot.meeting.id).catch((cause: unknown) => setError(cause instanceof Error ? cause.message : String(cause)))
    })
    return () => socket.close()
  }, [snapshot, refresh])

  const resultBody = useMemo(() => snapshot?.contents.find((item) => item.id === snapshot.meeting.result_content_id)?.body, [snapshot])

  async function connectProject() {
    setBusy(true); setError("")
    try {
      const opened = await openProject(repoPath.trim())
      setProject(opened)
      localStorage.setItem("nemeton.repo", repoPath.trim())
    } catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)) }
    finally { setBusy(false) }
  }

  async function launchMeeting() {
    if (!project) return
    const recorders = participants.filter((item) => item.role === "recorder")
    if (recorders.length !== 1) { setError("参与者必须有且只有一个 Recorder。") ; return }
    setBusy(true); setError("")
    try {
      const created = await createMeeting(project.project.id, title.trim(), brief.trim(), participants)
      await startMeeting(created.meeting.id)
      await refresh(created.meeting.id)
    } catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)) }
    finally { setBusy(false) }
  }

  async function submitReview(action: "approve" | "continue", choices: Record<string, ReviewChoice>, comment: string) {
    if (!snapshot) return
    setBusy(true); setError("")
    try {
      const items = Object.entries(choices).map(([candidate_id, choice]) => ({
        candidate_id,
        design_disposition: choice === "persist" || choice === "result_only" ? "accepted" : choice === "reject" ? "rejected" : "deferred",
        context_disposition: choice === "persist" ? "persist" : choice === "result_only" ? "result_only" : "none",
      }))
      await reviewMeeting(snapshot.meeting.id, action, items, comment)
      await refresh(snapshot.meeting.id)
    } catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)) }
    finally { setBusy(false) }
  }

  if (snapshot && (snapshot.meeting.status === "awaiting_user_review" || snapshot.meeting.status === "concluded")) {
    return <ResultReview busy={busy} handoff={handoff} key={snapshot.meeting.result_content_id || snapshot.meeting.id} onBack={() => { setSnapshot(undefined); setHandoff(undefined); localStorage.removeItem("nemeton.meeting") }} onSubmit={submitReview} resultBody={resultBody} snapshot={snapshot} />
  }

  return (
    <div className="app-shell">
      <header className="app-bar">
        <div className="wordmark">Nemeton</div>
        <div className="section-label"><MessagesSquare aria-hidden="true" />会议</div>
        <div className="app-bar-actions">
          {snapshot ? <span className="live-status"><Radio />{statusLabel(snapshot.meeting.status)}</span> : <span className="local-note">Local · {location.host}</span>}
          {snapshot && <button className="new-meeting-button" onClick={() => { setSnapshot(undefined); localStorage.removeItem("nemeton.meeting") }} type="button"><Plus />新会议</button>}
        </div>
      </header>

      {error && <div className="error-banner"><CircleAlert /><strong>错误</strong><span>{error}</span><button onClick={() => setError("")} type="button">关闭</button></div>}

      {!snapshot && !project && (
        <main className="start-screen">
          <section className="start-card">
            <h1>从一个真实项目问题开始</h1>
            <p>Nemeton 冻结当前 Git Reality，让你选择的 Agent 在隔离工作区里讨论，再由 Recorder 把结果带回给你审阅。</p>
            <label className="large-field"><span>Repository 绝对路径</span><input autoFocus onChange={(event) => setRepoPath(event.target.value)} placeholder="/mnt/data/coding/akasic-agent" value={repoPath} /></label>
            <button className="primary-button" disabled={!repoPath.trim() || busy} onClick={connectProject} type="button">打开项目</button>
          </section>
        </main>
      )}

      {!snapshot && project && (
        <main className="compose-screen">
          <section className="compose-card">
            <div className="compose-heading"><h1>你希望这次会议解决什么？</h1><p>{project.binding.canonical_root}</p></div>
            <label className="large-field"><span>会议标题</span><input autoFocus onChange={(event) => setTitle(event.target.value)} placeholder="例如：确定断线恢复的状态所有权" value={title} /></label>
            <label className="large-field"><span>原始问题</span><textarea onChange={(event) => setBrief(event.target.value)} placeholder="可以只给一个模糊方向。Agent 会读取项目、收集证据并提出设计。" value={brief} /></label>
            <div className="participant-disclosure">
              <button aria-expanded={configOpen} onClick={() => setConfigOpen(!configOpen)} type="button">
                <span><strong>参与者与模型</strong><small>{participants.length} 位 · {participants.filter((item) => item.role === "recorder").length} Recorder</small></span>
                <ChevronDown data-open={configOpen || undefined} />
              </button>
              {configOpen && <ParticipantBuilder onChange={setParticipants} value={participants} />}
            </div>
            <div className="compose-actions"><span>会议只产出设计、项目语义与 Handoff，不修改目标仓库。</span><button className="primary-button" disabled={!title.trim() || !brief.trim() || busy} onClick={launchMeeting} type="button">召开会议</button></div>
          </section>
        </main>
      )}

      {snapshot && (
        <main className="meeting-shell">
          <header className="meeting-header-live">
            <div><span>Cycle {snapshot.meeting.cycle}</span><h1>{snapshot.meeting.title}</h1></div>
            <small>Committed sequence {snapshot.stream_version}</small>
          </header>
          <MeetingRoomLive snapshot={snapshot} />
        </main>
      )}
    </div>
  )
}

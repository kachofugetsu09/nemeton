import { useMemo } from "react"
import { Circle, LoaderCircle } from "lucide-react"
import type { MeetingContent, MeetingSnapshot } from "@/lib/api"

function parse(body?: string): Record<string, unknown> {
  if (!body) return {}
  try { return JSON.parse(body) as Record<string, unknown> } catch { return { text: body } }
}

function participantName(snapshot: MeetingSnapshot, id?: string) {
  const participant = snapshot.participants.find((item) => item.id === id)
  if (!participant) return "You"
  return participant.seat === "recorder" ? "Recorder" : participant.seat
}

function naturalText(content: MeetingContent) {
  const body = parse(content.body)
  if (content.kind === "position") return String(body.rationale || body.canonical_statement || body.text || "")
  if (content.kind === "recorder_opening") return String(body.opening || body.response || body.text || "")
  if (content.kind === "recorder_answer") return String(body.response || body.text || "")
  if (content.kind === "review_comment" || content.kind === "human_input") return String(body.text || content.body || "")
  return ""
}

export function MeetingRoomLive({ snapshot }: { snapshot: MeetingSnapshot }) {
  const proposals = useMemo(() => snapshot.contents.filter((item) => item.kind === "proposal"), [snapshot.contents])
  const discussion = useMemo(() => snapshot.contents.filter((item) =>
    ["position", "recorder_opening", "recorder_answer", "review_comment"].includes(item.kind),
  ), [snapshot.contents])

  return (
    <div className="meeting-grid">
      <aside className="roster-panel">
        <div className="panel-heading"><span>参与者</span><small>{snapshot.participants.length}</small></div>
        <div className="roster-list">
          {snapshot.participants.map((participant) => (
            <div className="roster-person" key={participant.id}>
              <span className="avatar">{participant.seat.slice(0, 1).toUpperCase()}</span>
              <span><strong>{participantName(snapshot, participant.id)}</strong><small>{participant.provider} · {participant.model}</small></span>
              {participant.status === "running" ? <LoaderCircle className="spin" /> : <Circle />}
            </div>
          ))}
        </div>
      </aside>

      <section className="discussion-panel" aria-label="会议讨论">
        <div className="meeting-context">
          <span>原始问题</span>
          <p>{snapshot.meeting.brief}</p>
        </div>
        <div className="conversation">
          {discussion.length === 0 && (
            <div className="empty-conversation">
              <LoaderCircle className="spin" />
              <strong>会议正在展开</strong>
              <p>Agent 的自然语言讨论会出现在这里。Proposal 保持在右侧，不打断对话。</p>
            </div>
          )}
          {discussion.map((content) => {
            const speaker = participantName(snapshot, content.participant_id)
            return (
              <article className="message" key={content.id}>
                <div className="message-meta"><strong>@{speaker}</strong><span>Cycle {content.cycle} · Round {content.round}</span></div>
                <p>{naturalText(content)}</p>
              </article>
            )
          })}
        </div>
      </section>

      <aside className="proposal-panel-live">
        <div className="panel-heading"><span>Proposals</span><small>{proposals.length}</small></div>
        <div className="proposal-list">
          {proposals.length === 0 && <p className="quiet-empty">等待 Design Agent 提交。</p>}
          {proposals.map((content) => {
            const body = parse(content.body)
            const candidates = Array.isArray(body.candidate_items) ? body.candidate_items as Array<{ statement?: string }> : []
            return (
              <article className="proposal-card-live" key={content.id}>
                <div className="proposal-author">@{participantName(snapshot, content.participant_id)}</div>
                <h3>{String(body.summary || "Proposal")}</h3>
                {candidates.map((candidate, index) => <p key={index}>{candidate.statement}</p>)}
              </article>
            )
          })}
        </div>
      </aside>
    </div>
  )
}

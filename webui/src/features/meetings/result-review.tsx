import { useMemo, useState } from "react"
import { ArrowLeft, Check, Download, MessageSquareMore } from "lucide-react"
import type { Handoff, MeetingSnapshot, SemanticCandidate } from "@/lib/api"

export type ReviewChoice = "persist" | "result_only" | "reject" | "defer"

const choices: Array<{ id: ReviewChoice; label: string }> = [
  { id: "persist", label: "接受并长期保留" },
  { id: "result_only", label: "接受但仅本方案" },
  { id: "reject", label: "反对" },
  { id: "defer", label: "稍后" },
]

function resultSynthesis(body?: string) {
  if (!body) return ""
  try {
    const value = JSON.parse(body) as { synthesis?: string }
    return value.synthesis || body
  } catch { return body }
}

export function ResultReview({ snapshot, resultBody, busy, handoff, onSubmit, onBack }: {
  snapshot: MeetingSnapshot
  resultBody?: string
  busy: boolean
  handoff?: Handoff
  onSubmit: (action: "approve" | "continue", choices: Record<string, ReviewChoice>, comment: string) => Promise<void>
  onBack: () => void
}) {
  const candidates = useMemo(() => snapshot.candidates.filter((item) =>
    item.source_refs.includes(snapshot.meeting.result_content_id || ""),
  ), [snapshot])
  const [selected, setSelected] = useState<Record<string, ReviewChoice>>(() =>
    Object.fromEntries(candidates.map((candidate) => [candidate.id,
      candidate.context_disposition === "persist" ? "persist" :
      candidate.design_disposition === "rejected" ? "reject" :
      candidate.design_disposition === "deferred" ? "defer" : "result_only",
    ])),
  )
  const [comment, setComment] = useState("")
  const complete = candidates.every((candidate) => selected[candidate.id])
  const approvable = complete && candidates.every((candidate) =>
    selected[candidate.id] === "persist" || selected[candidate.id] === "result_only",
  )

  if (handoff) {
    return (
      <section className="handoff-view">
        <div className="review-toolbar"><button className="text-button" onClick={onBack} type="button"><ArrowLeft />返回会议</button></div>
        <div className="handoff-document">
          <div className="completion-mark"><Check /></div>
          <span className="document-label">Coding Design Handoff</span>
          <h1>{handoff.title}</h1>
          <p className="handoff-brief">{handoff.original_brief}</p>
          <pre>{resultSynthesis(handoff.result)}</pre>
          <h2>后续会议必须遵守</h2>
          {handoff.approved_context.length === 0 ? <p>本次没有新增长期项目语义。</p> : (
            <ul>{handoff.approved_context.map((item) => <li key={item.id}>{item.statement}</li>)}</ul>
          )}
          <button className="primary-button" onClick={() => navigator.clipboard.writeText(handoff.markdown)} type="button"><Download />复制 Markdown Handoff</button>
        </div>
      </section>
    )
  }

  return (
    <section className="review-view">
      <div className="review-toolbar">
        <button className="text-button" onClick={onBack} type="button"><ArrowLeft />返回会议</button>
        <span>Result Review · Cycle {snapshot.meeting.cycle}</span>
      </div>
      <div className="review-layout">
        <article className="result-document">
          <span className="document-label">Recorder synthesis</span>
          <h1>{snapshot.meeting.title}</h1>
          <p className="result-copy">{resultSynthesis(resultBody)}</p>
        </article>
        <aside className="review-controls">
          <div><h2>语义处置</h2><p>逐项决定是否接受，以及是否成为未来会议的固定上下文。</p></div>
          <div className="candidate-review-list">
            {candidates.map((candidate: SemanticCandidate, index) => (
              <fieldset className="candidate-review" key={candidate.id}>
                <legend><span>{String(index + 1).padStart(2, "0")}</span>{candidate.statement}</legend>
                <div className="choice-grid">
                  {choices.map((choice) => (
                    <label data-selected={selected[candidate.id] === choice.id || undefined} key={choice.id}>
                      <input
                        checked={selected[candidate.id] === choice.id}
                        name={candidate.id}
                        onChange={() => setSelected({ ...selected, [candidate.id]: choice.id })}
                        type="radio"
                      />
                      {choice.label}
                    </label>
                  ))}
                </div>
              </fieldset>
            ))}
          </div>
          <label className="comment-field"><span><MessageSquareMore />补充给 Recorder（可选）</span>
            <textarea onChange={(event) => setComment(event.target.value)} placeholder="指出疑问或不满意之处；由 Recorder 自己判断回答、修订还是重开会议。" value={comment} />
          </label>
          <div className="review-actions">
            <button className="secondary-button" disabled={!complete || busy} onClick={() => onSubmit("continue", selected, comment)} type="button">继续处理</button>
            <button className="primary-button" disabled={!approvable || busy} onClick={() => onSubmit("approve", selected, comment)} type="button">批准完整方案</button>
          </div>
        </aside>
      </div>
    </section>
  )
}

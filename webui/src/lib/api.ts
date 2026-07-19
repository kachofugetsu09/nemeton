export interface ParticipantInput {
  id: string
  seat: string
  role: "designer" | "recorder"
  provider: "codex" | "opencode"
  model: string
  provider_options: Record<string, string>
}

export interface MeetingParticipant {
  id: string
  seat: string
  role: string
  provider: string
  model: string
  provider_options: Record<string, string>
  status: string
  session_id?: string
}

export interface MeetingContent {
  id: string
  participant_id?: string
  kind: string
  cycle: number
  round: number
  content_digest: string
  refs: string[]
  body?: string
}

export interface SemanticCandidate {
  id: string
  kind: string
  statement: string
  source_refs: string[]
  status: string
  design_disposition: string
  context_disposition: string
  rationale: string
}

export interface MeetingSnapshot {
  meeting: {
    id: string
    project_id: string
    title: string
    brief: string
    status: string
    cycle: number
    current_round: number
    result_digest: string
    result_content_id?: string
    approved_result_digest?: string
    protocol_version: number
  }
  participants: MeetingParticipant[]
  contents: MeetingContent[]
  candidates: SemanticCandidate[]
  stream_version: number
}

export interface ProjectSnapshot {
  project: { id: string }
  binding: { display_path: string; canonical_root: string }
}

export interface Handoff {
  schema: string
  meeting_id: string
  title: string
  original_brief: string
  result_content_id: string
  result_digest: string
  result: string
  approved_context: SemanticCandidate[]
  markdown: string
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: init?.body ? { "Content-Type": "application/json", ...init.headers } : init?.headers,
  })
  if (!response.ok) {
    const problem = await response.json().catch(() => ({ detail: response.statusText })) as { detail?: string }
    throw new Error(problem.detail || `Request failed: ${response.status}`)
  }
  return response.json() as Promise<T>
}

export async function openProject(path: string) {
  const response = await request<{ project: ProjectSnapshot }>("/v1/projects/open", {
    method: "POST",
    body: JSON.stringify({ path }),
  })
  return response.project
}

export async function createMeeting(projectId: string, title: string, brief: string, participants: ParticipantInput[]) {
  const response = await request<{ meeting: MeetingSnapshot }>(`/v1/projects/${projectId}/meetings`, {
    method: "POST",
    body: JSON.stringify({
      kind: "change",
      title,
      brief,
      participants: participants.map(({ id: _id, ...participant }) => participant),
    }),
  })
  return response.meeting
}

export async function startMeeting(meetingId: string) {
  const response = await request<{ meeting: MeetingSnapshot }>(`/v1/meetings/${meetingId}/start`, {
    method: "POST",
    body: "{}",
  })
  return response.meeting
}

export async function inspectMeeting(meetingId: string) {
  const response = await request<{ meeting: MeetingSnapshot }>(`/v1/meetings/${meetingId}`)
  return response.meeting
}

export async function hydrateContents(snapshot: MeetingSnapshot) {
  const contents = await Promise.all(snapshot.contents.map(async (content) => {
    const response = await request<{ content: { body: string } }>(
      `/v1/meetings/${snapshot.meeting.id}/contents/${content.id}`,
    )
    return { ...content, body: response.content.body }
  }))
  return { ...snapshot, contents }
}

export async function reviewMeeting(
  meetingId: string,
  resultAction: "approve" | "continue",
  items: Array<{ candidate_id: string; design_disposition: string; context_disposition: string }>,
  comment: string,
) {
  const response = await request<{ meeting: MeetingSnapshot }>(`/v1/meetings/${meetingId}/reviews`, {
    method: "POST",
    body: JSON.stringify({ result_action: resultAction, items, comment }),
  })
  return response.meeting
}

export async function fetchHandoff(meetingId: string) {
  const response = await request<{ handoff: Omit<Handoff, "markdown">; markdown: string }>(`/v1/meetings/${meetingId}/handoff`)
  return { ...response.handoff, markdown: response.markdown }
}

export function watchMeeting(meetingId: string, after: number, onCommitted: () => void) {
  const protocol = location.protocol === "https:" ? "wss:" : "ws:"
  const socket = new WebSocket(`${protocol}//${location.host}/v1/meetings/${meetingId}/ws?after_sequence=${after}`)
  let pendingRefresh = 0
  socket.addEventListener("message", (event) => {
    const message = JSON.parse(String(event.data)) as { type: string }
    if (message.type !== "domain_event" || pendingRefresh) return
    pendingRefresh = window.setTimeout(() => {
      pendingRefresh = 0
      onCommitted()
    }, 40)
  })
  return { close() {
    if (pendingRefresh) window.clearTimeout(pendingRefresh)
    socket.close()
  } }
}

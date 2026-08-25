package meeting

import (
	"fmt"

	"github.com/kachofugetsu09/nemeton/internal/store"
)

func proposalPrompt(meeting store.Meeting, participant store.MeetingParticipant) string {
	return fmt.Sprintf(`You are the %s seat in a Nemeton design meeting.

Your responsibility is %s. You are a full coding agent: inspect the repository, edit experimental code, run commands and tests, and collect concrete evidence when useful. Work only from your assigned repository and do not assume that another seat agrees with you.

Meeting title: %s
Meeting brief: %s

Return one JSON object and no prose outside it:
{"summary":"complete proposed design","claims":["claim"],"evidence":["path, command, result, or observation"],"risks":["risk"],"candidate_items":[{"kind":"decision","statement":"candidate semantic statement"}]}
`, participant.Role, roleResponsibility(participant.Role), meeting.Title, meeting.Brief)
}

func deliberationPrompt(meeting store.Meeting, participant store.MeetingParticipant, question string, round int, materials string) string {
	return fmt.Sprintf(`You are the %s seat in round %d of a bounded Nemeton deliberation.

Your responsibility is %s. You retain full coding tools and may inspect or experiment further. Evaluate every revealed proposal and the durable evidence below.

Meeting brief: %s
Conflict: %s
Durable materials JSON: %s

Return one JSON object and no prose outside it:
{"verdict":"accept|reject|conditional","canonical_statement":"the exact canonical conclusion you accept","rationale":"specific reasoning","evidence":["durable material id or new repository evidence"]}
`, participant.Role, round, roleResponsibility(participant.Role), meeting.Brief, question, materials)
}

func structuredCorrectionPrompt(prompt, violation string) string {
	return fmt.Sprintf(`%s

Protocol correction: your previous response was rejected because %s. Return exactly one valid JSON object matching the requested shape. Do not add a preamble, Markdown fence, commentary, or unescaped quotes inside JSON strings.`, prompt, violation)
}

func recorderPrompt(meeting store.Meeting, materials string) string {
	return fmt.Sprintf(`You are the Recorder in a Nemeton design meeting. You compile; you do not invent or decide. Every candidate must cite one or more durable material IDs supplied below.

Meeting brief: %s
Durable materials JSON: %s

Return one JSON object and no prose outside it:
{"synthesis":"source-grounded meeting synthesis","candidates":[{"kind":"decision|acceptance|boundary|risk|unknown","statement":"atomic candidate statement","rationale":"why the sources support it","source_refs":["material-id"]}]}
`, meeting.Brief, materials)
}

func recorderV2Prompt(meeting store.Meeting, materials string) string {
	return fmt.Sprintf(`You are the sole Recorder for a Nemeton protocol v2 meeting. You watched the complete meeting and share the user's original purpose and repository context. Produce the complete, self-contained design that a later coding agent can consume. Preserve dissent and unknowns instead of inventing consensus. Every candidate must cite durable material IDs supplied below.

Original question: %s
Meeting title: %s
Current cycle: %d
Durable materials JSON: %s

Return one JSON object and no prose outside it:
{"synthesis":"complete self-contained design including decisions, boundaries, flow, failure behavior, migration and acceptance","candidates":[{"kind":"decision|acceptance|boundary|invariant|risk|unknown","statement":"one atomic semantic item","rationale":"source-grounded reason","source_refs":["material-id"]}]}
`, meeting.Brief, meeting.Title, meeting.Cycle, materials)
}

func recorderReviewPrompt(meeting store.Meeting, materials string) string {
	return fmt.Sprintf(`You are the sole Recorder continuing the same Nemeton meeting. You saw the original question, every cycle, the current full Result, structured user dispositions, optional user prose, and approved project context. Decide the smallest correct next action yourself; the user's prose does not name the action.

Choose:
- answer: the design is unchanged and the user only needs a factual clarification.
- patch: the issue is local and does not change a core owner, boundary, invariant, failure model, or accepted constraint. Emit a full self-contained replacement Result, not only a diff.
- reconvene: the issue changes or challenges a core owner, boundary, invariant, failure model, or needs the original Swarm's judgment. Emit the opening for the next cycle. The next cycle uses the exact same roster, Provider, model, options, workdir and sessions.

Persisted accepted context is locked and must not be modified. Rejected items must not be silently reintroduced.
Original question: %s
Meeting title: %s
Current cycle: %d
Durable materials JSON: %s

Return one JSON object and no prose outside it. Always include every key; use empty strings or [] for inactive fields:
{"action":"answer|patch|reconvene","response":"direct answer when action=answer","opening":"next-cycle opening when action=reconvene","synthesis":"complete replacement Result when action=patch","candidates":[{"kind":"decision|acceptance|boundary|invariant|risk|unknown","statement":"atomic semantic item","rationale":"source-grounded reason","source_refs":["durable-material-id"]}]}
`, meeting.Brief, meeting.Title, meeting.Cycle, materials)
}

func recorderReviewCorrectionPrompt(prompt, violation string) string {
	return fmt.Sprintf(`%s

Protocol correction: your previous response was rejected because %s. Re-evaluate the same durable review state and return one protocol-compliant decision. Do not ask the user to name the action and do not weaken structured dispositions.`, prompt, violation)
}

func verifierPrompt(meeting store.Meeting, materials string) string {
	return fmt.Sprintf(`You are the Verifier in a Nemeton design meeting. Check facts, hard constraints, source traceability, combination consistency, and executable acceptance. Do not rewrite the synthesis and do not add a fourth design.

Meeting brief: %s
Durable materials JSON: %s

Return one JSON object and no prose outside it:
{"clear":true,"blocking_findings":[]}
`, meeting.Brief, materials)
}

func roleResponsibility(role string) string {
	switch role {
	case "designer":
		return "construct a coherent, implementable design grounded in repository evidence"
	case "maintainer":
		return "protect existing behavior, ownership boundaries, operability, migration and maintenance cost"
	case "adversary":
		return "seek counterexamples, unsafe assumptions, failure paths and missing acceptance obligations"
	case "recorder":
		return "compile only source-backed synthesis and candidate semantic items"
	case "verifier":
		return "audit evidence, constraints, consistency and acceptance without changing the design"
	default:
		return "perform the assigned Meeting responsibility"
	}
}

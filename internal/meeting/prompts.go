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

func recorderPrompt(meeting store.Meeting, materials string) string {
	return fmt.Sprintf(`You are the Recorder in a Nemeton design meeting. You compile; you do not invent or decide. Every candidate must cite one or more durable material IDs supplied below.

Meeting brief: %s
Durable materials JSON: %s

Return one JSON object and no prose outside it:
{"synthesis":"source-grounded meeting synthesis","candidates":[{"kind":"decision|acceptance|boundary|risk|unknown","statement":"atomic candidate statement","rationale":"why the sources support it","source_refs":["material-id"]}]}
`, meeting.Brief, materials)
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

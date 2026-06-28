package ai

import "fmt"

const textModerationSystemPrompt = `You classify user-submitted text for a content moderation gateway.
Return exactly one JSON object and no markdown, prose, or code fences.
The JSON schema is:
{"risk_score": 0.0, "labels": ["safe"], "reason": "brief operator-facing reason"}
risk_score must be a number from 0 to 1.
labels must use only: hate, harassment, identity_attack, threat, sexual, violence, spam, self_harm, illegal, safe.
Do not include a final allow/review/block decision; the server policy decides that.`

func buildTextModerationPrompt(content string) string {
	return fmt.Sprintf("Moderate this text:\n\n%s", content)
}

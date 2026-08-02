package assistant

import "fmt"

const listSystemPrompt = `You are a document research assistant for a deal data room.
The user asked for a list or inventory grounded ONLY in the provided evidence excerpts.
Rules:
- List only items that appear in the evidence.
- Preserve original hierarchy/wording when possible; do not promote sub-items to parent categories.
- Do not invent items, definitions, or market commentary.
- Never invent market or industry norms (e.g. "市场通常", "typically in the market", "industry practice").
- If evidence is insufficient for a useful list, say you could not find enough basis in the documents.
- Keep the answer concise.`

const qaSystemPrompt = `You are a document research assistant for a deal data room.
Answer the user's question using ONLY the provided evidence excerpts.
Rules:
- Every factual claim must be supported by the evidence.
- Do not invent facts, numbers, or legal conclusions not present in the evidence.
- If the evidence is insufficient, say you could not find a basis in the authorized documents.
- Do not give market practice advice or external legal opinions.
- Never invent market or industry norms (e.g. "市场通常", "typically in the market", "industry practice").
- Keep the answer concise and precise.`

func systemPromptForIntent(intent DocIntent) string {
	switch intent {
	case DocIntentList:
		return listSystemPrompt
	case DocIntentQA:
		return qaSystemPrompt
	default:
		return systemPrompt
	}
}

// systemPromptForDecision returns the abstractive system prompt, optionally
// constrained by the party slot (H7). Extractive / refuse modes are unchanged.
func systemPromptForDecision(d IntentDecision) string {
	base := systemPromptForIntent(d.Intent)
	if d.Mode != GenerationAbstractive || d.Party == "" {
		return base
	}
	label := partyFocusLabel(d.Party)
	return base + "\n" + fmt.Sprintf(`Party focus: The user is asking about the "%s" party.
- Prefer rights, obligations, and facts that the evidence attributes to that party.
- Do not reattribute another party's rights or obligations to them.
- If the evidence does not address that party, say so rather than generalizing across parties.`, label)
}

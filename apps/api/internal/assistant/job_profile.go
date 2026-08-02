package assistant

// JobProfile is the exported per-intent knob table (P1).
type JobProfile struct {
	Mode          GenerationMode
	TopK          int
	MaxEvidence   int
	PreferLiteral bool
	SkipLLMFilter bool
}

// ProfileFor returns the stable JobProfile for a DocIntent.
func ProfileFor(intent DocIntent) JobProfile {
	switch intent {
	case DocIntentLocate:
		return JobProfile{Mode: GenerationExtractive, TopK: 8, MaxEvidence: 1, PreferLiteral: true, SkipLLMFilter: true}
	case DocIntentTopic:
		return JobProfile{Mode: GenerationExtractive, TopK: 8, MaxEvidence: 3, PreferLiteral: false, SkipLLMFilter: true}
	case DocIntentList:
		return JobProfile{Mode: GenerationAbstractive, TopK: 8, MaxEvidence: 5, PreferLiteral: false, SkipLLMFilter: true}
	case DocIntentQA:
		return JobProfile{Mode: GenerationAbstractive, TopK: 8, MaxEvidence: 5, PreferLiteral: false, SkipLLMFilter: false}
	case DocIntentRefuseEarly:
		return JobProfile{Mode: GenerationRefuse, TopK: 0, MaxEvidence: 0, PreferLiteral: false, SkipLLMFilter: true}
	default:
		return JobProfile{Mode: GenerationAbstractive, TopK: 8, MaxEvidence: 5, PreferLiteral: false, SkipLLMFilter: false}
	}
}

// profileFor keeps the unexported alias used by the router path.
func profileFor(intent DocIntent) JobProfile {
	return ProfileFor(intent)
}

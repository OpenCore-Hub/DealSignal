package assistant

// Registry maps DocIntent → JobProfile for the Ask Docs clue engine (P1 export).
// Vertical Pack plugins are out of scope until P2.
type Registry struct {
	profiles map[DocIntent]JobProfile
}

// DefaultRegistry returns the built-in intent → profile table.
func DefaultRegistry() *Registry {
	intents := []DocIntent{
		DocIntentLocate,
		DocIntentTopic,
		DocIntentList,
		DocIntentQA,
		DocIntentRefuseEarly,
	}
	profiles := make(map[DocIntent]JobProfile, len(intents))
	for _, intent := range intents {
		profiles[intent] = ProfileFor(intent)
	}
	return &Registry{profiles: profiles}
}

// Profile returns the JobProfile for intent, falling back to ProfileFor for unknown keys.
func (r *Registry) Profile(intent DocIntent) JobProfile {
	if r == nil {
		return ProfileFor(intent)
	}
	if p, ok := r.profiles[intent]; ok {
		return p
	}
	return ProfileFor(intent)
}

// Intents returns the registered primary DocIntent keys in stable order.
func (r *Registry) Intents() []DocIntent {
	if r == nil {
		return nil
	}
	out := make([]DocIntent, 0, len(r.profiles))
	for _, intent := range []DocIntent{
		DocIntentLocate,
		DocIntentTopic,
		DocIntentList,
		DocIntentQA,
		DocIntentRefuseEarly,
	} {
		if _, ok := r.profiles[intent]; ok {
			out = append(out, intent)
		}
	}
	return out
}

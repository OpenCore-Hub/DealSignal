package suggestions

import "testing"

func TestQuestionIntentCreatesSignal(t *testing.T) {
	for _, intent := range []string{"pricing", "objection", "timeline", "implementation", "feature_request"} {
		if !questionIntentCreatesSignal(intent) {
			t.Errorf("expected intent %q to create signal", intent)
		}
	}
	for _, intent := range []string{"support", "general", "security"} {
		if questionIntentCreatesSignal(intent) {
			t.Errorf("expected intent %q not to create signal", intent)
		}
	}
}

func TestQuestionSignalType(t *testing.T) {
	for _, intent := range []string{"pricing", "objection", "timeline"} {
		if questionSignalType(intent) != "hot_signal" {
			t.Errorf("expected intent %q to map to hot_signal", intent)
		}
	}
	for _, intent := range []string{"implementation", "feature_request"} {
		if questionSignalType(intent) != "follow_up" {
			t.Errorf("expected intent %q to map to follow_up", intent)
		}
	}
}

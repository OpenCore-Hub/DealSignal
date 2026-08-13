package link

import (
	"context"
	"errors"
	"testing"
)

func TestExpiryReminderExpirePastDueOnce(t *testing.T) {
	t.Parallel()
	var got int64
	r := &ExpiryReminder{
		expirePastDue: func(context.Context) (int64, error) {
			got = 4
			return 4, nil
		},
	}
	r.expirePastDueOnce(context.Background())
	if got != 4 {
		t.Fatalf("expirePastDueOnce must invoke hook, got=%d", got)
	}

	r = &ExpiryReminder{}
	r.expirePastDueOnce(context.Background()) // nil hook no-ops

	r = &ExpiryReminder{
		expirePastDue: func(context.Context) (int64, error) {
			return 0, errors.New("db down")
		},
	}
	r.expirePastDueOnce(context.Background()) // logs and returns; must not panic
}

func TestExpiryReminderRunOnceNilNotifierSafe(t *testing.T) {
	t.Parallel()
	var got int64
	r := &ExpiryReminder{
		// queries nil: RunOnce must still invoke past-due before list reminders.
		expirePastDue: func(context.Context) (int64, error) {
			got = 2
			return 2, nil
		},
	}
	r.RunOnce(context.Background())
	if got != 2 {
		t.Fatalf("RunOnce must call past-due hook, got=%d", got)
	}
}

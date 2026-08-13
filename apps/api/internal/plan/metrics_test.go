package plan

import (
	"testing"
)

func TestRecordQuotaDenialFromErr(t *testing.T) {
	t.Parallel()

	before := TestingDenialCount(CodeLimitStorage)
	RecordQuotaDenialFromErr(ErrLimitStorage)
	after := TestingDenialCount(CodeLimitStorage)
	if after < before+1 {
		t.Fatalf("expected storage denial counter to increase, before=%v after=%v", before, after)
	}

	beforeRooms := TestingDenialCount(CodeLimitRooms)
	RecordQuotaDenial(CodeLimitRooms)
	if TestingDenialCount(CodeLimitRooms) < beforeRooms+1 {
		t.Fatal("expected rooms denial counter to increase")
	}

	beforeOther := TestingDenialCount(CodeFeatureNDA)
	RecordQuotaDenialFromErr(nil)
	RecordQuotaDenialFromErr(errTest("not a plan error"))
	RecordQuotaDenial("")
	if TestingDenialCount(CodeFeatureNDA) != beforeOther {
		t.Fatal("non-plan errors must not bump feature counters")
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }

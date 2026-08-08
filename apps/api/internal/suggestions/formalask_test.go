package suggestions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateFormalAskSuggestionNoopWithoutTurn(t *testing.T) {
	svc := NewService(nil, nil, nil)
	require.NoError(t, svc.CreateFormalAskSuggestion(context.Background(), CreateFormalAskSuggestionInput{
		WorkspaceID: "11111111-1111-1111-1111-111111111111",
		LinkID:      "22222222-2222-2222-2222-222222222222",
		Question:    "Any disclosure?",
	}))
}

func TestResolveFormalAskSuggestionNoopWithoutTurn(t *testing.T) {
	svc := NewService(nil, nil, nil)
	require.NoError(t, svc.ResolveFormalAskSuggestion(context.Background(), "11111111-1111-1111-1111-111111111111", ""))
}

func TestFormalAskLocalizedCopy(t *testing.T) {
	en := newLocalizedStrings("en")
	require.Contains(t, en.formalAskAction, "Formal")
	zh := newLocalizedStrings("zh-CN")
	require.Contains(t, zh.formalAskAction, "正式")
	require.Equal(t, "正式问答待审", zh.subtypeTitles[SubtypeFormalAsk])
}

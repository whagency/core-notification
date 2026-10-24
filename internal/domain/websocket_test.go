package domain

import "testing"

func TestWsMessageTypeString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		typ  WsMessageType
		want string
	}{
		{name: "successful deposit", typ: WsMessageTypeSuccessfulDeposit, want: "sd"},
		{name: "required verification", typ: WsMessageTypeRequiredVerificationDeposit, want: "rvd"},
		{name: "successful withdrawal", typ: WsMessageTypeSuccessfulWithdrawal, want: "sw"},
		{name: "successful incoming transfer", typ: WsMessageTypeSuccessfulIncomingTransfer, want: "st"},
		{name: "unknown", typ: WsMessageType(99), want: "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.typ.String(); got != tc.want {
				t.Fatalf("String = %q; want %q", got, tc.want)
			}
		})
	}
}

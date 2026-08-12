//go:build linux

package linux

import (
	"context"
	"errors"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Tokens the fakes below hand around. They are opaque to the policy: what each
// test asserts is which one was presented and which one was kept.
const (
	storedToken  = "stored-token"
	revokedToken = "revoked-token"
	nextToken    = "next-token"
	freshToken   = "fresh-token"
)

// Failures the scripted portal answers with. Static so the no-retry test can
// assert the caller sees the very error the second attempt produced.
var (
	errNotRestorable = errors.New("session could not be restored")
	errPortalRefused = errors.New("portal refused")
	errStoreReadOnly = errors.New("read-only state directory")
)

// fakeTokenStore is a restoreTokenStore that records what the policy did to it.
type fakeTokenStore struct {
	token   string
	saved   []string
	cleared int
	saveErr error
}

func (f *fakeTokenStore) load() string { return f.token }

func (f *fakeTokenStore) save(token string) error {
	if f.saveErr != nil {
		return f.saveErr
	}

	f.token = token
	f.saved = append(f.saved, token)

	return nil
}

func (f *fakeTokenStore) clear() error {
	f.token = ""
	f.cleared++

	return nil
}

// recordingOpener replays a scripted sequence of portal answers and records the
// restore token each attempt presented. A test that expects one attempt scripts
// one answer, so an unexpected extra attempt fails loudly rather than reading
// past the end of the script.
type recordingOpener struct {
	t         *testing.T
	answers   []openerAnswer
	presented []string
}

type openerAnswer struct {
	grant portalGrant
	err   error
}

func (r *recordingOpener) open(_ context.Context, restoreToken string) (portalGrant, error) {
	r.t.Helper()

	if len(r.presented) >= len(r.answers) {
		r.t.Fatalf(
			"portal opened %d times, script has %d answers",
			len(r.presented)+1,
			len(r.answers),
		)
	}

	answer := r.answers[len(r.presented)]
	r.presented = append(r.presented, restoreToken)

	return answer.grant, answer.err
}

// grantWithToken builds a portal answer carrying a fresh restore token.
func grantWithToken(token string) openerAnswer {
	return openerAnswer{grant: portalGrant{eisFD: 7, restoreToken: token}}
}

// TestEstablishPortalGrant_RestoresTheStoredGrantWithoutPrompting is the whole
// point of the ticket: a restart presents the token it stored last time and the
// portal restores the session, so the user sees no consent dialog.
func TestEstablishPortalGrant_RestoresTheStoredGrantWithoutPrompting(t *testing.T) {
	store := &fakeTokenStore{token: storedToken}
	opener := &recordingOpener{t: t, answers: []openerAnswer{grantWithToken(nextToken)}}

	grant, err := establishPortalGrant(context.Background(), store, opener.open)
	if err != nil {
		t.Fatalf("establishPortalGrant() error = %v", err)
	}

	if grant.eisFD != 7 {
		t.Errorf("grant.eisFD = %d, want 7", grant.eisFD)
	}

	if len(opener.presented) != 1 || opener.presented[0] != storedToken {
		t.Errorf("portal attempts = %q, want one attempt presenting the stored token",
			opener.presented)
	}

	if store.cleared != 0 {
		t.Errorf("stored token cleared %d times, want 0", store.cleared)
	}
}

// TestEstablishPortalGrant_StoresTheTokenTheGrantReturned pins the half that
// makes the restore repeatable. A restore token is invalidated by the use that
// consumes it, so a session that keeps the token it presented would restore
// exactly once and prompt forever after.
func TestEstablishPortalGrant_StoresTheTokenTheGrantReturned(t *testing.T) {
	store := &fakeTokenStore{token: storedToken}
	opener := &recordingOpener{t: t, answers: []openerAnswer{grantWithToken(nextToken)}}

	_, err := establishPortalGrant(context.Background(), store, opener.open)
	if err != nil {
		t.Fatalf("establishPortalGrant() error = %v", err)
	}

	if len(store.saved) != 1 || store.saved[0] != nextToken {
		t.Errorf("tokens saved = %q, want the token the grant returned", store.saved)
	}
}

// TestEstablishPortalGrant_ClearsTheStoredTokenWhenTheGrantIsNotPersistent
// covers a portal that grants the session but declines to persist it — the
// token that was just presented is spent, so keeping it would have the next
// start replay a credential that cannot work.
func TestEstablishPortalGrant_ClearsTheStoredTokenWhenTheGrantIsNotPersistent(t *testing.T) {
	store := &fakeTokenStore{token: storedToken}
	opener := &recordingOpener{t: t, answers: []openerAnswer{grantWithToken("")}}

	_, err := establishPortalGrant(context.Background(), store, opener.open)
	if err != nil {
		t.Fatalf("establishPortalGrant() error = %v", err)
	}

	if store.cleared != 1 {
		t.Errorf("stored token cleared %d times, want 1", store.cleared)
	}

	if len(store.saved) != 0 {
		t.Errorf("tokens saved = %q, want none", store.saved)
	}
}

// TestEstablishPortalGrant_PromptsOnceWhenTheStoredTokenIsRefused is the
// revoked-or-expired path: exactly two attempts, the second presenting nothing
// so the portal prompts, and the dead token dropped rather than retried.
func TestEstablishPortalGrant_PromptsOnceWhenTheStoredTokenIsRefused(t *testing.T) {
	store := &fakeTokenStore{token: revokedToken}
	opener := &recordingOpener{t: t, answers: []openerAnswer{
		{err: errNotRestorable},
		grantWithToken(freshToken),
	}}

	grant, err := establishPortalGrant(context.Background(), store, opener.open)
	if err != nil {
		t.Fatalf("establishPortalGrant() error = %v", err)
	}

	if grant.eisFD != 7 {
		t.Errorf("grant.eisFD = %d, want 7", grant.eisFD)
	}

	want := []string{revokedToken, ""}
	if len(opener.presented) != len(want) ||
		opener.presented[0] != want[0] || opener.presented[1] != want[1] {
		t.Errorf("portal attempts = %q, want %q", opener.presented, want)
	}

	if store.cleared != 1 {
		t.Errorf("stored token cleared %d times, want 1", store.cleared)
	}

	if len(store.saved) != 1 || store.saved[0] != freshToken {
		t.Errorf("tokens saved = %q, want the token the fresh grant returned", store.saved)
	}
}

// TestEstablishPortalGrant_DoesNotRetryAfterTheFreshPromptFails is the
// no-retry-loop half. The script carries two answers, so a third attempt fails
// the test rather than being tolerated.
func TestEstablishPortalGrant_DoesNotRetryAfterTheFreshPromptFails(t *testing.T) {
	store := &fakeTokenStore{token: revokedToken}
	opener := &recordingOpener{t: t, answers: []openerAnswer{
		{err: errNotRestorable},
		{err: errPortalRefused},
	}}

	_, err := establishPortalGrant(context.Background(), store, opener.open)
	if !errors.Is(err, errPortalRefused) {
		t.Fatalf("establishPortalGrant() error = %v, want %v", err, errPortalRefused)
	}

	if len(opener.presented) != 2 {
		t.Errorf("portal attempts = %d, want 2", len(opener.presented))
	}
}

// TestEstablishPortalGrant_DoesNotPromptAgainWhenTheUserCanceled keeps a
// refusal the user made themselves from being answered with a second dialog.
// A canceled request says nothing about the stored token, so it is kept.
func TestEstablishPortalGrant_DoesNotPromptAgainWhenTheUserCanceled(t *testing.T) {
	store := &fakeTokenStore{token: storedToken}
	opener := &recordingOpener{t: t, answers: []openerAnswer{
		{err: errPortalRequestCanceled},
	}}

	_, err := establishPortalGrant(context.Background(), store, opener.open)
	if !errors.Is(err, errPortalRequestCanceled) {
		t.Fatalf("establishPortalGrant() error = %v, want cancelation", err)
	}

	if len(opener.presented) != 1 {
		t.Errorf("portal attempts = %d, want 1", len(opener.presented))
	}

	if store.cleared != 0 {
		t.Errorf("stored token cleared %d times, want 0", store.cleared)
	}
}

// TestEstablishPortalGrant_KeepsTheStoredTokenWhenNothingReachedThePortal is
// the other half of "prompt once": a bus that could not be reached and a
// handshake that ran out of time are not the token's fault, so throwing it away
// would spend a working grant — and buy a consent prompt on the next start —
// for a failure that had nothing to do with it.
func TestEstablishPortalGrant_KeepsTheStoredTokenWhenNothingReachedThePortal(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "no session bus", err: derrors.New(derrors.CodeNotSupported, "no session bus")},
		{name: "handshake timed out", err: derrors.New(derrors.CodeTimeout, "no answer in time")},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			store := &fakeTokenStore{token: storedToken}
			opener := &recordingOpener{t: t, answers: []openerAnswer{{err: testCase.err}}}

			_, err := establishPortalGrant(context.Background(), store, opener.open)
			if err == nil {
				t.Fatal("establishPortalGrant() error = nil, want the portal's failure")
			}

			if len(opener.presented) != 1 {
				t.Errorf("portal attempts = %d, want 1", len(opener.presented))
			}

			if store.cleared != 0 {
				t.Errorf("stored token cleared %d times, want 0", store.cleared)
			}
		})
	}
}

// TestPromptingFallbackAllowed_RefusesToReAskAUserWhoSaidNo guards the second
// place a dialog could be raised: the KDE input path falls back to the
// liboeffis handshake when its own portal attempt fails, and that handshake
// always prompts. Falling back after a cancelation would ask the user the same
// question twice in one start.
func TestPromptingFallbackAllowed_RefusesToReAskAUserWhoSaidNo(t *testing.T) {
	if promptingFallbackAllowed(errPortalRequestCanceled) {
		t.Error("a canceled consent request allows a prompting fallback, want refused")
	}

	if promptingFallbackAllowed(errPortalGrantYieldedNoDevices) {
		t.Error("a device-less grant allows a prompting fallback, want refused")
	}

	if !promptingFallbackAllowed(errPortalRefused) {
		t.Error("a portal failure refuses a prompting fallback, want allowed")
	}
}

// TestEstablishPortalGrant_PromptsWhenNothingIsStored covers the first run:
// one attempt, presenting no token, and the token it comes back with stored.
func TestEstablishPortalGrant_PromptsWhenNothingIsStored(t *testing.T) {
	store := &fakeTokenStore{}
	opener := &recordingOpener{t: t, answers: []openerAnswer{grantWithToken("first-token")}}

	_, err := establishPortalGrant(context.Background(), store, opener.open)
	if err != nil {
		t.Fatalf("establishPortalGrant() error = %v", err)
	}

	if len(opener.presented) != 1 || opener.presented[0] != "" {
		t.Errorf("portal attempts = %q, want one attempt presenting no token",
			opener.presented)
	}

	if len(store.saved) != 1 || store.saved[0] != "first-token" {
		t.Errorf("tokens saved = %q, want the token the grant returned", store.saved)
	}
}

// TestEstablishPortalGrant_KeepsTheGrantWhenTheTokenCannotBeStored pins the
// degradation: an unwritable state directory costs the user a prompt on the
// next start, which is strictly better than refusing the session they just
// approved.
func TestEstablishPortalGrant_KeepsTheGrantWhenTheTokenCannotBeStored(t *testing.T) {
	store := &fakeTokenStore{saveErr: errStoreReadOnly}
	opener := &recordingOpener{t: t, answers: []openerAnswer{grantWithToken(nextToken)}}

	grant, err := establishPortalGrant(context.Background(), store, opener.open)
	if err != nil {
		t.Fatalf("establishPortalGrant() error = %v", err)
	}

	if grant.eisFD != 7 {
		t.Errorf("grant.eisFD = %d, want 7", grant.eisFD)
	}
}

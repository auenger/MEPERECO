package main

import (
	"context"
	"errors"
	"testing"

	"github.com/auenger/MEPERECO/core/internal/events"
	"github.com/auenger/MEPERECO/core/internal/util"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/auenger/MEPERECO/core/pkg/protocol"
	"github.com/jackc/pgx/v5/pgtype"
)

type chatNotificationFixture struct {
	session      db.ChatSession
	participants []db.ListChatSessionParticipantsRow
	removed      pgtype.UUID
}

func (f *chatNotificationFixture) GetChatSession(context.Context, pgtype.UUID) (db.ChatSession, error) {
	return f.session, nil
}
func (f *chatNotificationFixture) ListChatSessionParticipants(context.Context, pgtype.UUID) ([]db.ListChatSessionParticipantsRow, error) {
	return f.participants, nil
}
func (f *chatNotificationFixture) GetMemberByUserAndWorkspace(_ context.Context, p db.GetMemberByUserAndWorkspaceParams) (db.Member, error) {
	if p.UserID == f.removed {
		return db.Member{}, errors.New("not a member")
	}
	return db.Member{}, nil
}

func TestChatNotificationsRecipientAndMention(t *testing.T) {
	workspace := util.MustParseUUID("10000000-0000-0000-0000-000000000000")
	session := util.MustParseUUID("20000000-0000-0000-0000-000000000000")
	sender := util.MustParseUUID("30000000-0000-0000-0000-000000000000")
	mentioned := util.MustParseUUID("40000000-0000-0000-0000-000000000000")
	other := util.MustParseUUID("50000000-0000-0000-0000-000000000000")
	removed := util.MustParseUUID("60000000-0000-0000-0000-000000000000")
	agent := util.MustParseUUID("70000000-0000-0000-0000-000000000000")
	f := &chatNotificationFixture{session: db.ChatSession{ID: session, WorkspaceID: workspace, CreatorID: sender, Kind: "group", Status: "active"}, removed: removed}
	for _, id := range []pgtype.UUID{sender, mentioned, other, removed} {
		f.participants = append(f.participants, db.ListChatSessionParticipantsRow{ParticipantType: "member", ParticipantID: id})
	}
	f.participants = append(f.participants, db.ListChatSessionParticipantsRow{ParticipantType: "agent", ParticipantID: agent})
	bus := events.New()
	registerChatNotificationListeners(bus, f)
	got := map[string]protocol.ChatNotificationPayload{}
	bus.Subscribe(protocol.EventChatNotification, func(e events.Event) { p := e.Payload.(protocol.ChatNotificationPayload); got[p.RecipientID] = p })
	message := protocol.ChatMessagePayload{ChatSessionID: util.UUIDToString(session), MessageID: "message-1", Role: "user", SenderUserID: util.UUIDToString(sender), Content: "[@someone](mention://member/" + util.UUIDToString(mentioned) + ")"}
	publish := func() {
		bus.Publish(events.Event{Type: protocol.EventChatMessage, WorkspaceID: util.UUIDToString(workspace), Payload: message})
	}
	publish()
	if len(got) != 2 || !got[util.UUIDToString(mentioned)].Mentioned || got[util.UUIDToString(other)].Mentioned {
		t.Fatalf("unexpected recipients or mention flags: %#v", got)
	}
	got = map[string]protocol.ChatNotificationPayload{}
	message.Content = "[@all](mention://all/all)"
	publish()
	if len(got) != 2 || !got[util.UUIDToString(mentioned)].Mentioned || !got[util.UUIDToString(other)].Mentioned {
		t.Fatalf("unexpected @all notification: %#v", got)
	}
	got = map[string]protocol.ChatNotificationPayload{}
	f.session.Status = "archived"
	publish()
	if len(got) != 0 {
		t.Fatal("archived session must not notify")
	}
	f.session.Status, f.session.Kind = "active", "direct"
	publish()
	if len(got) != 0 {
		t.Fatal("outgoing direct messages must not notify")
	}
	bus.Publish(events.Event{Type: protocol.EventChatDone, WorkspaceID: util.UUIDToString(workspace), Payload: protocol.ChatDonePayload{ChatSessionID: util.UUIDToString(session), MessageID: "reply"}})
	if len(got) != 1 || got[util.UUIDToString(sender)].MessageID != "reply" {
		t.Fatalf("direct reply should notify only owner: %#v", got)
	}
}

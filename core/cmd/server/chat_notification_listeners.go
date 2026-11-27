package main

import (
	"context"
	"strings"

	"github.com/auenger/MEPERECO/core/internal/events"
	"github.com/auenger/MEPERECO/core/internal/util"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/auenger/MEPERECO/core/pkg/protocol"
	"github.com/jackc/pgx/v5/pgtype"
)

// The personal event is an invalidation/notification hint, not a second inbox
// item. Carry IDs only; the client must fetch the accessible session list.
type chatNotificationQueries interface {
	GetChatSession(context.Context, pgtype.UUID) (db.ChatSession, error)
	ListChatSessionParticipants(context.Context, pgtype.UUID) ([]db.ListChatSessionParticipantsRow, error)
	GetMemberByUserAndWorkspace(context.Context, db.GetMemberByUserAndWorkspaceParams) (db.Member, error)
}

func registerChatNotificationListeners(bus *events.Bus, queries chatNotificationQueries) {
	notify := func(e events.Event, sessionID, messageID, senderUserID, content, createdAt string) {
		if messageID == "" || sessionID == "" {
			return
		}
		id, err := util.ParseUUID(sessionID)
		if err != nil {
			return
		}
		ctx := context.Background()
		session, err := queries.GetChatSession(ctx, id)
		if err != nil || session.Status != "active" || util.UUIDToString(session.WorkspaceID) != e.WorkspaceID {
			return
		}
		var recipients []pgtype.UUID
		if session.Kind == "group" {
			participants, err := queries.ListChatSessionParticipants(ctx, id)
			if err != nil {
				return
			}
			for _, p := range participants {
				if p.ParticipantType == "member" {
					recipients = append(recipients, p.ParticipantID)
				}
			}
		} else {
			// Direct-chat user messages are outgoing, not a notification.
			if senderUserID != "" {
				return
			}
			recipients = append(recipients, session.CreatorID)
		}
		mentions := util.ParseMentions(content)
		for _, recipient := range recipients {
			userID := util.UUIDToString(recipient)
			if userID == "" || userID == senderUserID {
				continue
			}
			// Membership may have changed between message creation and delivery.
			if _, err := queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: recipient, WorkspaceID: session.WorkspaceID}); err != nil {
				continue
			}
			bus.Publish(events.Event{Type: protocol.EventChatNotification, WorkspaceID: e.WorkspaceID,
				Payload: protocol.ChatNotificationPayload{WorkspaceID: e.WorkspaceID, ChatSessionID: sessionID,
					MessageID: messageID, RecipientID: userID, CreatedAt: createdAt,
					Mentioned: session.Kind == "group" && chatMentionsMember(mentions, userID)}})
		}
	}
	bus.Subscribe(protocol.EventChatMessage, func(e events.Event) {
		if p, ok := e.Payload.(protocol.ChatMessagePayload); ok && p.Role == "user" && p.SenderUserID != "" {
			notify(e, p.ChatSessionID, p.MessageID, p.SenderUserID, p.Content, p.CreatedAt)
		}
	})
	bus.Subscribe(protocol.EventChatDone, func(e events.Event) {
		if p, ok := e.Payload.(protocol.ChatDonePayload); ok {
			notify(e, p.ChatSessionID, p.MessageID, "", p.Content, p.CreatedAt)
		}
	})
}

func chatMentionsMember(mentions []util.Mention, userID string) bool {
	for _, mention := range mentions {
		if (mention.Type == "member" && strings.EqualFold(mention.ID, userID)) || (mention.Type == "all" && mention.ID == "all") {
			return true
		}
	}
	return false
}

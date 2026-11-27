package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/auenger/MEPERECO/core/internal/util"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"log/slog"
	"strings"

	"github.com/auenger/MEPERECO/core/internal/events"
	"github.com/auenger/MEPERECO/core/internal/handler"
	"github.com/auenger/MEPERECO/core/internal/realtime"
	"github.com/auenger/MEPERECO/core/pkg/protocol"
)

// registerListeners wires up event bus listeners for WS broadcasting.
// Personal events (inbox, invites) are sent only to the target user via
// SendToUser. All other events are broadcast to the workspace room.
//
// The broadcaster parameter is intentionally typed as the realtime.Broadcaster
// interface (not *realtime.Hub) so that this layer can later be swapped out
// for a Redis-backed relay or a feature-flagged dual-write implementation
// without touching any of the event listeners below. This is Phase 0 of the
// horizontal-scaling plan tracked in SILI-1138.
func registerListeners(bus *events.Bus, b realtime.Broadcaster, q *db.Queries) {
	// Personal events should NOT be broadcast to the whole workspace.
	personalEvents := map[string]bool{
		protocol.EventInboxNew:             true,
		protocol.EventProjectAccessChanged: true,
		protocol.EventChatNotification:     true,
		protocol.EventChatSessionRead:      true,
		protocol.EventInboxRead:            true,
		protocol.EventInboxArchived:        true,
		protocol.EventInboxUnarchived:      true,
		protocol.EventInboxBatchRead:       true,
		protocol.EventInboxBatchArchived:   true,
		protocol.EventInvitationCreated:    true,
		protocol.EventInvitationRevoked:    true,
	}

	// Helper: marshal event and send to a specific user.
	sendToRecipient := func(b realtime.Broadcaster, e events.Event, recipientID string) {
		if recipientID == "" {
			return
		}
		if e.Type == protocol.EventInboxNew || e.Type == protocol.EventChatNotification || e.Type == protocol.EventChatSessionRead {
			ws, err := util.ParseUUID(e.WorkspaceID)
			if err != nil {
				return
			}
			user, err := util.ParseUUID(recipientID)
			if err != nil {
				return
			}
			scope := resolveProjectEventScope(context.Background(), q, e, ws)
			if scope.resolved && !projectEventAllowed(context.Background(), q, scope, ws, user) {
				return
			}
			if !scope.resolved && e.Type != protocol.EventInboxNew {
				return
			}
		}
		data, err := json.Marshal(map[string]any{"type": e.Type, "payload": e.Payload, "actor_id": e.ActorID, "actor_type": e.ActorType})
		if err != nil {
			return
		}
		realtime.M.RecordEvent(e.Type)
		if e.Type == protocol.EventProjectAccessChanged {
			b.BroadcastToScope(realtime.ScopeWorkspaceUser, e.WorkspaceID+"/"+recipientID, data)
		} else {
			b.SendToUser(recipientID, data)
		}
	}
	bus.Subscribe(protocol.EventProjectAccessChanged, func(e events.Event) {
		if p, ok := e.Payload.(map[string]any); ok {
			uid, _ := p["recipient_id"].(string)
			sendToRecipient(b, e, uid)
		}
	})

	// inbox:new — extract recipient from nested item
	bus.Subscribe(protocol.EventInboxNew, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		item, ok := payload["item"].(map[string]any)
		if !ok {
			return
		}
		recipientID, _ := item["recipient_id"].(string)
		sendToRecipient(b, e, recipientID)
	})
	bus.Subscribe(protocol.EventChatNotification, func(e events.Event) {
		if payload, ok := e.Payload.(protocol.ChatNotificationPayload); ok {
			sendToRecipient(b, e, payload.RecipientID)
		}
	})
	bus.Subscribe(protocol.EventChatSessionRead, func(e events.Event) {
		if payload, ok := e.Payload.(protocol.ChatSessionReadPayload); ok {
			sendToRecipient(b, e, payload.RecipientID)
		}
	})

	// inbox:read, inbox:archived, inbox:unarchived, inbox:batch-read,
	// inbox:batch-archived — extract recipient from top-level payload
	for _, eventType := range []string{
		protocol.EventInboxRead, protocol.EventInboxArchived, protocol.EventInboxUnarchived,
		protocol.EventInboxBatchRead, protocol.EventInboxBatchArchived,
	} {
		bus.Subscribe(eventType, func(e events.Event) {
			payload, ok := e.Payload.(map[string]any)
			if !ok {
				return
			}
			recipientID, _ := payload["recipient_id"].(string)
			sendToRecipient(b, e, recipientID)
		})
	}

	// invitation:created — send to the invitee so they see the invitation in real time.
	bus.Subscribe(protocol.EventInvitationCreated, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		inv, ok := payload["invitation"].(handler.InvitationResponse)
		if !ok {
			// Fallback for map encoding.
			if invMap, ok := payload["invitation"].(map[string]any); ok {
				if uid, _ := invMap["invitee_user_id"].(*string); uid != nil && *uid != "" {
					data, err := json.Marshal(map[string]any{"type": e.Type, "payload": e.Payload, "actor_id": e.ActorID, "actor_type": e.ActorType})
					if err != nil {
						return
					}
					realtime.M.RecordEvent(e.Type)
					b.SendToUser(*uid, data)
				}
			}
			return
		}
		if inv.InviteeUserID != nil && *inv.InviteeUserID != "" {
			data, err := json.Marshal(map[string]any{"type": e.Type, "payload": e.Payload, "actor_id": e.ActorID, "actor_type": e.ActorType})
			if err != nil {
				return
			}
			realtime.M.RecordEvent(e.Type)
			b.SendToUser(*inv.InviteeUserID, data)
		}
	})

	// invitation:revoked — send to the invitee so their pending list updates.
	bus.Subscribe(protocol.EventInvitationRevoked, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		uid, _ := payload["invitee_user_id"].(*string)
		if uid != nil && *uid != "" {
			sendToRecipient(b, e, *uid)
		}
	})

	// member:added — also send to the invited user so they discover the new workspace.
	// Pass excludeWorkspace so clients already in the target room (reached via
	// BroadcastToWorkspace in SubscribeAll) don't receive the event twice.
	bus.Subscribe(protocol.EventMemberAdded, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		var userID string
		switch m := payload["member"].(type) {
		case handler.MemberWithUserResponse:
			userID = m.UserID
		case map[string]any:
			userID, _ = m["user_id"].(string)
		default:
			slog.Warn("member:added: unexpected member payload type", "type", fmt.Sprintf("%T", payload["member"]))
		}
		if userID == "" {
			return
		}
		data, err := json.Marshal(map[string]any{"type": e.Type, "payload": e.Payload, "actor_id": e.ActorID, "actor_type": e.ActorType})
		if err != nil {
			return
		}
		realtime.M.RecordEvent(e.Type)
		b.SendToUser(userID, data, e.WorkspaceID)
	})

	// SubscribeAll handles workspace-broadcast for non-personal events.
	bus.SubscribeAll(func(e events.Event) {
		// Skip personal events — they are handled by type-specific listeners above.
		if personalEvents[e.Type] {
			return
		}

		msg := map[string]any{
			"type":       e.Type,
			"payload":    e.Payload,
			"actor_id":   e.ActorID,
			"actor_type": e.ActorType,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			slog.Error("failed to marshal event", "event_type", e.Type, "error", err)
			return
		}

		// Resource events carrying project data must not fan out to the
		// full workspace. Resolve scope at delivery time, including revocations.
		if e.WorkspaceID != "" && projectSensitiveEvent(e.Type) {
			ctx := context.Background()
			ws, err := util.ParseUUID(e.WorkspaceID)
			if err != nil {
				return
			}
			scope := resolveProjectEventScope(ctx, q, e, ws)
			if !scope.resolved {
				// Deleted rows no longer have lineage. Invalidate without IDs,
				// titles, transcripts, or other confidential fields.
				b.BroadcastToWorkspace(e.WorkspaceID, []byte(`{"type":"project:resources_changed","payload":{}}`))
				return
			}
			recipients, err := projectEventAudience(ctx, q, scope, ws)
			if err != nil {
				return
			}
			for _, uid := range recipients {
				b.BroadcastToScope(realtime.ScopeWorkspaceUser, e.WorkspaceID+"/"+util.UUIDToString(uid), data)
			}

			return
		}

		if e.WorkspaceID != "" {
			realtime.M.RecordEvent(e.Type)
			b.BroadcastToWorkspace(e.WorkspaceID, data)
		} else if strings.HasPrefix(e.Type, "daemon:") {
			realtime.M.RecordEvent(e.Type)
			b.Broadcast(data)
		}
		// Otherwise drop — no global broadcast for non-daemon events without a workspace.
	})
}

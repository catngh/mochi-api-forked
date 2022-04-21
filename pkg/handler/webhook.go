package handler

import (
	"encoding/json"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/defipod/mochi/pkg/model"
	"github.com/defipod/mochi/pkg/request"
	"github.com/defipod/mochi/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) HandleDiscordWebhook(c *gin.Context) {
	var req request.HandleDiscordWebhookRequest
	if err := req.Bind(c); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch req.Event {
	case request.GUILD_MEMBER_ADD:
		h.handleGuildMemberAdd(c, req.Data)
	}
}

func (h *Handler) handleGuildMemberAdd(c *gin.Context, data json.RawMessage) {
	var member discordgo.Member
	byteData, err := data.MarshalJSON()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := discordgo.Unmarshal(byteData, &member); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.handleInviteTracker(c, &member)
}

func (h *Handler) handleInviteTracker(c *gin.Context, invitee *discordgo.Member) {
	var response response.HandleInviteHistoryResponse

	inviter, isVanity, err := h.entities.FindInviter(invitee.GuildID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.IsVanity = isVanity

	if inviter != nil {
		if err := h.entities.CreateUser(request.CreateUserRequest{
			ID:       inviter.User.ID,
			Username: inviter.User.Username,
			Nickname: inviter.Nick,
			JoinDate: inviter.JoinedAt,
			GuildID:  inviter.GuildID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		response.InviterID = inviter.User.ID
		if inviter.User.Bot {
			response.IsBot = true
		}
	}
	if invitee != nil {
		if err := h.entities.CreateUser(request.CreateUserRequest{
			ID:        invitee.User.ID,
			Username:  invitee.User.Username,
			Nickname:  invitee.Nick,
			JoinDate:  invitee.JoinedAt,
			GuildID:   invitee.GuildID,
			InvitedBy: invitee.User.ID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		response.InviteeID = invitee.User.ID
	}

	inviteType := model.INVITE_TYPE_NORMAL
	if inviter == nil {
		inviteType = model.INVITE_TYPE_LEFT
	}

	// TODO: Can't find age of user now
	// if time.Now().Unix()-invit < 60*60*24*3 {
	// 	inviteType = model.INVITE_TYPE_FAKE
	// }

	if err := h.entities.CreateInviteHistory(request.CreateInviteHistoryRequest{
		GuildID: invitee.GuildID,
		Inviter: inviter.User.ID,
		Invitee: invitee.User.ID,
		Type:    inviteType,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalInvites, err := h.entities.CountInviteHistoriesByGuildUser(inviter.GuildID, inviter.User.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response.InvitesAmount = int(totalInvites)
	c.JSON(http.StatusOK, gin.H{
		"data": response,
	})
}

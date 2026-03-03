// whatsapp_service.go — WhatsApp Business API integration.
//
// All order lifecycle notifications are sent through this service.
// If WHATSAPP_PHONE_ID is not set the service gracefully logs instead
// of sending, making local development easier.

package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosFactory-backend/internal/config"
)

type WhatsAppService struct {
	phoneID string
	token   string
	baseURL string
	enabled bool
}

func NewWhatsAppService(cfg *config.Config) *WhatsAppService {
	return &WhatsAppService{
		phoneID: cfg.WhatsAppPhoneID,
		token:   cfg.WhatsAppToken,
		baseURL: "https://graph.facebook.com/v18.0",
		enabled: cfg.WhatsAppPhoneID != "" && cfg.WhatsAppToken != "",
	}
}

// whatsappPayload is the JSON body for the Meta API.
type whatsappPayload struct {
	MessagingProduct string `json:"messaging_product"`
	To               string `json:"to"`
	Type             string `json:"type"`
	Text             struct {
		Body string `json:"body"`
	} `json:"text"`
}

func (s *WhatsAppService) send(phone, message string) {
	if !s.enabled {
		log.Info().
			Str("to", maskPhone(phone)).
			Int("message_len", len(message)).
			Msg("[WhatsApp DISABLED] Would send message")
		return
	}

	// Clean phone number
	to := strings.ReplaceAll(phone, "+", "")
	to = strings.ReplaceAll(to, " ", "")
	to = strings.ReplaceAll(to, "-", "")

	payload := whatsappPayload{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "text",
	}
	payload.Text.Body = message

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/%s/messages", s.baseURL, s.phoneID)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Error().Err(err).Msg("WhatsApp: failed to create request")
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("WhatsApp: failed to send message")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Error().
			Int("status", resp.StatusCode).
			Str("to", maskPhone(to)).
			Msg("WhatsApp: unexpected status")
	}
}

func maskPhone(phone string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)

	if len(digits) <= 4 {
		return "****"
	}

	return strings.Repeat("*", len(digits)-4) + digits[len(digits)-4:]
}

// SendOrderCreated notifies customer their order was received.
func (s *WhatsAppService) SendOrderCreated(phone, orderNumber string, total float64) {
	msg := fmt.Sprintf(
		"🧊 *Orden Recibida*\n\n"+
			"¡Gracias por tu pedido!\n\n"+
			"*Número de orden:* %s\n"+
			"*Total:* $%.2f\n\n"+
			"Estamos revisando tu orden y te confirmaremos pronto. 🍦",
		orderNumber, total,
	)
	s.send(phone, msg)
}

// SendOrderApproved notifies customer their order was confirmed.
func (s *WhatsAppService) SendOrderApproved(phone, orderNumber string) {
	msg := fmt.Sprintf(
		"✅ *Orden Confirmada*\n\n"+
			"Tu orden *%s* ha sido confirmada y está en preparación.\n\n"+
			"Te avisaremos cuando esté lista. 🎉",
		orderNumber,
	)
	s.send(phone, msg)
}

// SendOrderRejected notifies customer their order was not accepted.
func (s *WhatsAppService) SendOrderRejected(phone, orderNumber, reason string) {
	msg := fmt.Sprintf(
		"❌ *Orden No Procesada*\n\n"+
			"Lamentamos informarte que tu orden *%s* no pudo ser procesada.\n\n"+
			"*Motivo:* %s\n\n"+
			"Por favor contáctanos para más información.",
		orderNumber, reason,
	)
	s.send(phone, msg)
}

// SendOrderReady notifies customer their order is ready for pickup.
func (s *WhatsAppService) SendOrderReady(phone, orderNumber string) {
	msg := fmt.Sprintf(
		"🎉 *¡Tu Orden Está Lista!*\n\n"+
			"Tu orden *%s* está lista para recoger.\n\n"+
			"¡Te esperamos! 🍦",
		orderNumber,
	)
	s.send(phone, msg)
}

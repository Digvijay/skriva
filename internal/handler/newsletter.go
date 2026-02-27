// newsletter.go — newsletter CRUD, subscriber management, email sending, scheduling.
package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Digvijay/skriva/internal/config"
	"github.com/Digvijay/skriva/internal/store"
)

// HandleNewsletterSubscribe handles email subscription.
func (h *BlogHandler) HandleNewsletterSubscribe(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		http.Error(w, "Invalid token. Please reload and try again.", http.StatusForbidden)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	if email == "" || !strings.Contains(email, "@") || len(email) > 254 {
		http.Error(w, "Invalid email address", http.StatusBadRequest)
		return
	}

	// Generate unique token
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	if err := h.store.AddSubscriber(r.Context(), email, token); err != nil {
		h.logger.Error("adding subscriber", "error", err)
	}

	// Send confirmation email if SMTP is configured, otherwise auto-confirm
	smtpCfg := h.cfg.GetSMTP()
	site := h.cfg.GetSite()
	if smtpCfg.Enabled && smtpCfg.Host != "" {
		confirmURL := site.BaseURL + "/api/unsubscribe?token=" + token + "&action=confirm"
		subject := "Confirm your subscription to " + site.Title
		body := "Hi!\n\nPlease confirm your subscription to " + site.Title + " by visiting:\n" + confirmURL + "\n\nIf you didn't subscribe, ignore this email.\n"
		if err := h.sendEmail(smtpCfg, email, subject, body); err != nil {
			h.logger.Error("sending confirmation email", "to", email, "error", err)
			// Still auto-confirm as fallback
			_ = h.store.ConfirmSubscriber(r.Context(), token)
		}
	} else {
		// No SMTP — auto-confirm
		_ = h.store.ConfirmSubscriber(r.Context(), token)
	}

	h.logger.Info("newsletter subscription", "email", email)

	// Redirect back with success — validate Referer is same-origin to prevent open redirect
	redirect := "/"
	if referer := r.Header.Get("Referer"); referer != "" {
		site := h.cfg.GetSite()
		if site.BaseURL != "" && strings.HasPrefix(referer, site.BaseURL) {
			redirect = referer
		} else if strings.HasPrefix(referer, "/") && !strings.HasPrefix(referer, "//") {
			redirect = referer
		}
	}
	http.Redirect(w, r, redirect+"#subscribed", http.StatusSeeOther)
}

// HandleNewsletterUnsubscribe handles unsubscribe and confirmation via token.
func (h *BlogHandler) HandleNewsletterUnsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	// Handle email confirmation (double-opt-in) when action=confirm is present
	action := r.URL.Query().Get("action")
	if action == "confirm" {
		if err := h.store.ConfirmSubscriber(r.Context(), token); err != nil {
			http.Error(w, "Invalid or expired confirmation link", http.StatusBadRequest)
			return
		}
		site := h.cfg.GetSite()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Confirmed</title></head><body style="font-family:sans-serif;text-align:center;padding:3rem"><h1>Subscription Confirmed</h1><p>You are now subscribed to %s.</p><a href="/">← Back to blog</a></body></html>`, html.EscapeString(site.Title))
		return
	}

	_ = h.store.RemoveSubscriber(r.Context(), token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Unsubscribed</title></head><body style="font-family:sans-serif;text-align:center;padding:3rem"><h1>Unsubscribed</h1><p>You have been removed from the mailing list.</p><a href="/">← Back to blog</a></body></html>`)
}

// HandleAdminNewsletter serves the newsletter editor/scheduler page.
func (h *BlogHandler) HandleAdminNewsletter(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	h.serveAdminHTML(w, adminNewsletterHTML)
}

// HandleAdminAPINewsletters lists all newsletters.
func (h *BlogHandler) HandleAdminAPINewsletters(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	newsletters, err := h.store.AllNewsletters(r.Context())
	if err != nil {
		h.logger.Error("listing newsletters", "error", err)
		jsonError(w, "Failed to load newsletters", http.StatusInternalServerError)
		return
	}
	if newsletters == nil {
		newsletters = []store.Newsletter{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newsletters)
}

// HandleAdminAPICreateNewsletter creates a new newsletter draft.
func (h *BlogHandler) HandleAdminAPICreateNewsletter(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Subject) == "" {
		jsonError(w, "Subject is required", http.StatusBadRequest)
		return
	}
	if len(req.Subject) > 500 {
		jsonError(w, "Subject too long (max 500 characters)", http.StatusBadRequest)
		return
	}

	newsletter, err := h.store.CreateNewsletter(r.Context(), strings.TrimSpace(req.Subject), req.Body)
	if err != nil {
		h.logger.Error("creating newsletter", "error", err)
		jsonError(w, "Failed to create newsletter", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newsletter)
}

// HandleAdminAPIUpdateNewsletter updates a newsletter draft.
func (h *BlogHandler) HandleAdminAPIUpdateNewsletter(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid newsletter ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Subject) == "" {
		jsonError(w, "Subject is required", http.StatusBadRequest)
		return
	}
	if len(req.Subject) > 500 {
		jsonError(w, "Subject too long (max 500 characters)", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateNewsletter(r.Context(), id, strings.TrimSpace(req.Subject), req.Body); err != nil {
		h.logger.Error("updating newsletter", "error", err)
		jsonError(w, "Failed to update newsletter: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleAdminAPIDeleteNewsletter deletes a newsletter.
func (h *BlogHandler) HandleAdminAPIDeleteNewsletter(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid newsletter ID", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteNewsletter(r.Context(), id); err != nil {
		h.logger.Error("deleting newsletter", "error", err)
		jsonError(w, "Failed to delete newsletter", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleAdminAPIScheduleNewsletter schedules a newsletter for future sending.
func (h *BlogHandler) HandleAdminAPIScheduleNewsletter(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid newsletter ID", http.StatusBadRequest)
		return
	}

	var req struct {
		ScheduledAt string `json:"scheduled_at"` // RFC 3339 format
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		jsonError(w, "Invalid date format (use RFC 3339, e.g. 2026-03-01T09:00:00Z)", http.StatusBadRequest)
		return
	}

	if scheduledAt.Before(time.Now()) {
		jsonError(w, "Scheduled time must be in the future", http.StatusBadRequest)
		return
	}

	if err := h.store.ScheduleNewsletter(r.Context(), id, scheduledAt); err != nil {
		h.logger.Error("scheduling newsletter", "error", err)
		jsonError(w, "Failed to schedule: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "scheduled"})
}

// HandleAdminAPIUnscheduleNewsletter moves a scheduled newsletter back to draft.
func (h *BlogHandler) HandleAdminAPIUnscheduleNewsletter(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid newsletter ID", http.StatusBadRequest)
		return
	}

	if err := h.store.UnscheduleNewsletter(r.Context(), id); err != nil {
		h.logger.Error("unscheduling newsletter", "error", err)
		jsonError(w, "Failed to unschedule: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "draft"})
}

// HandleAdminAPISendNewsletter sends a newsletter immediately to all confirmed subscribers.
func (h *BlogHandler) HandleAdminAPISendNewsletter(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid newsletter ID", http.StatusBadRequest)
		return
	}

	smtpCfg := h.cfg.GetSMTP()
	if !smtpCfg.Enabled || smtpCfg.Host == "" {
		jsonError(w, "SMTP is not configured. Set up smtp.yaml to send newsletters.", http.StatusBadRequest)
		return
	}

	// Send in background
	go h.sendNewsletter(context.Background(), id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "sending"})
}

// HandleAdminAPINewsletterPreview returns rendered HTML preview of a newsletter body.
func (h *BlogHandler) HandleAdminAPINewsletterPreview(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	rendered, err := h.engine.RenderMarkdown(req.Body)
	if err != nil {
		jsonError(w, "Failed to render preview", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"html": rendered})
}

// HandleAdminAPISubscribers lists all subscribers for admin.
func (h *BlogHandler) HandleAdminAPISubscribers(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	subs, err := h.store.AllSubscribers(r.Context())
	if err != nil {
		h.logger.Error("listing subscribers", "error", err)
		jsonError(w, "Failed to load subscribers", http.StatusInternalServerError)
		return
	}
	if subs == nil {
		subs = []store.Subscriber{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subs)
}

// HandleAdminAPIDeleteSubscriber removes a subscriber by ID.
func (h *BlogHandler) HandleAdminAPIDeleteSubscriber(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid subscriber ID", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteSubscriber(r.Context(), id); err != nil {
		h.logger.Error("deleting subscriber", "error", err)
		jsonError(w, "Failed to delete subscriber", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// sendNewsletter sends a newsletter to all confirmed subscribers.
func (h *BlogHandler) sendNewsletter(ctx context.Context, id int64) {
	newsletter, err := h.store.GetNewsletter(ctx, id)
	if err != nil {
		h.logger.Error("loading newsletter for send", "id", id, "error", err)
		return
	}

	if err := h.store.MarkNewsletterSending(ctx, id); err != nil {
		h.logger.Error("marking newsletter as sending", "id", id, "error", err)
		return
	}

	subscribers, err := h.store.ConfirmedSubscribers(ctx)
	if err != nil {
		h.logger.Error("loading subscribers for newsletter", "id", id, "error", err)
		_ = h.store.MarkNewsletterFailed(ctx, id, 0, 0)
		return
	}

	if len(subscribers) == 0 {
		h.logger.Info("no confirmed subscribers, marking as sent", "newsletter_id", id)
		_ = h.store.MarkNewsletterSent(ctx, id, 0, 0)
		return
	}

	smtpCfg := h.cfg.GetSMTP()
	site := h.cfg.GetSite()

	// SMTP preflight check — verify connection before looping through subscribers
	addr := fmt.Sprintf("%s:%d", smtpCfg.Host, smtpCfg.Port)
	testConn, err := smtp.Dial(addr)
	if err != nil {
		h.logger.Error("SMTP preflight failed — cannot connect", "addr", addr, "error", err)
		_ = h.store.MarkNewsletterFailed(ctx, id, 0, len(subscribers))
		return
	}
	_ = testConn.Close()

	sentCount := 0
	failCount := 0

	// Render markdown body to HTML
	renderedHTML, err := h.engine.RenderMarkdown(newsletter.Body)
	if err != nil {
		h.logger.Error("rendering newsletter markdown", "id", id, "error", err)
		_ = h.store.MarkNewsletterFailed(ctx, id, 0, 0)
		return
	}

	for _, sub := range subscribers {
		unsubURL := site.BaseURL + "/api/unsubscribe?token=" + sub.Token

		// Add tracking pixel and rewrite links for analytics
		trackingPixel := fmt.Sprintf(`<img src="%s/api/newsletter/open?n=%d&s=%d" width="1" height="1" style="display:none" alt="">`,
			site.BaseURL, id, sub.ID)
		trackedHTML := rewriteLinksForTracking(renderedHTML, site.BaseURL, id, sub.ID)

		htmlBody := h.buildNewsletterEmail(site, newsletter.Subject, trackedHTML+trackingPixel, unsubURL)

		if err := h.sendHTMLEmail(smtpCfg, sub.Email, newsletter.Subject, htmlBody); err != nil {
			h.logger.Error("sending newsletter email", "subscriber_id", sub.ID, "newsletter_id", id, "error", err)
			_ = h.store.RecordNewsletterSend(ctx, id, sub.ID, "failed", "send failed")
			failCount++
		} else {
			_ = h.store.RecordNewsletterSend(ctx, id, sub.ID, "sent", "")
			sentCount++
		}

		// Brief delay between emails to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	if failCount > 0 && sentCount == 0 {
		_ = h.store.MarkNewsletterFailed(ctx, id, sentCount, failCount)
		h.logger.Error("newsletter send failed completely", "id", id, "fail_count", failCount)
	} else {
		_ = h.store.MarkNewsletterSent(ctx, id, sentCount, failCount)
		h.logger.Info("newsletter sent", "id", id, "sent", sentCount, "failed", failCount)
	}
}

// sendNewsletterRetry retries sending a newsletter only to previously failed recipients.
func (h *BlogHandler) sendNewsletterRetry(ctx context.Context, id int64) (sent, failedCount int) {
	newsletter, err := h.store.GetNewsletter(ctx, id)
	if err != nil {
		h.logger.Error("loading newsletter for retry", "id", id, "error", err)
		return 0, 0
	}

	failed, err := h.store.FailedNewsletterRecipients(ctx, id)
	if err != nil || len(failed) == 0 {
		h.logger.Info("no failed recipients to retry", "newsletter_id", id)
		return 0, 0
	}

	smtpCfg := h.cfg.GetSMTP()
	site := h.cfg.GetSite()

	// SMTP preflight
	addr := fmt.Sprintf("%s:%d", smtpCfg.Host, smtpCfg.Port)
	testConn, err := smtp.Dial(addr)
	if err != nil {
		h.logger.Error("SMTP preflight failed for retry", "addr", addr, "error", err)
		return 0, len(failed)
	}
	_ = testConn.Close()

	renderedHTML, err := h.engine.RenderMarkdown(newsletter.Body)
	if err != nil {
		return 0, len(failed)
	}

	sentCount := 0
	failCount := 0

	for _, sub := range failed {
		unsubURL := site.BaseURL + "/api/unsubscribe?token=" + sub.Token
		trackingPixel := fmt.Sprintf(`<img src="%s/api/newsletter/open?n=%d&s=%d" width="1" height="1" style="display:none" alt="">`,
			site.BaseURL, id, sub.ID)
		trackedHTML := rewriteLinksForTracking(renderedHTML, site.BaseURL, id, sub.ID)
		htmlBody := h.buildNewsletterEmail(site, newsletter.Subject, trackedHTML+trackingPixel, unsubURL)

		if err := h.sendHTMLEmail(smtpCfg, sub.Email, newsletter.Subject, htmlBody); err != nil {
			_ = h.store.RecordNewsletterSend(ctx, id, sub.ID, "failed", "retry failed")
			failCount++
		} else {
			_ = h.store.RecordNewsletterSend(ctx, id, sub.ID, "sent", "")
			sentCount++
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Update newsletter counts
	nl, _ := h.store.GetNewsletter(ctx, id)
	if nl != nil {
		_ = h.store.MarkNewsletterSent(ctx, id, nl.SentCount+sentCount, nl.FailCount-sentCount)
	}

	h.logger.Info("newsletter retry complete", "id", id, "retried", len(failed), "sent", sentCount, "still_failed", failCount)
	return sentCount, failCount
}

// buildNewsletterEmail constructs an HTML email for a newsletter.
func (h *BlogHandler) buildNewsletterEmail(site config.SiteConfig, subject, renderedHTML, unsubURL string) string {
	title := html.EscapeString(site.Title)
	if title == "" {
		title = "Blog"
	}

	return `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head>
<body style="margin:0;padding:0;background:#f5f5f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;color:#24292f">
<table width="100%" cellpadding="0" cellspacing="0" style="background:#f5f5f5;padding:24px 0">
<tr><td align="center">
<table width="600" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:8px;overflow:hidden;max-width:600px;width:100%">
<tr><td style="background:#0969da;padding:24px 32px;text-align:center">
<h1 style="margin:0;color:#fff;font-size:20px;font-weight:600">` + title + `</h1>
</td></tr>
<tr><td style="padding:32px">
<h2 style="margin:0 0 16px;font-size:22px;font-weight:600;color:#24292f">` + html.EscapeString(subject) + `</h2>
<div style="font-size:16px;line-height:1.6;color:#24292f">` + renderedHTML + `</div>
</td></tr>
<tr><td style="padding:16px 32px;border-top:1px solid #e1e4e8;text-align:center;font-size:12px;color:#656d76">
<p style="margin:0">You received this because you subscribed to ` + title + `.</p>
<p style="margin:8px 0 0"><a href="` + html.EscapeString(unsubURL) + `" style="color:#656d76">Unsubscribe</a></p>
</td></tr>
</table>
</td></tr>
</table>
</body>
</html>`
}

// sendHTMLEmail sends an HTML email using SMTP.
func (h *BlogHandler) sendHTMLEmail(smtpCfg config.SMTPConfig, to, subject, htmlBody string) error {
	from := smtpCfg.From
	if from == "" {
		from = smtpCfg.Username
	}

	// SECURITY: Sanitize email fields to prevent header injection via CRLF
	to = sanitizeEmailHeader(to)
	subject = sanitizeEmailHeader(subject)
	from = sanitizeEmailHeader(from)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s",
		from, to, subject, htmlBody)

	addr := fmt.Sprintf("%s:%d", smtpCfg.Host, smtpCfg.Port)
	auth := smtp.PlainAuth("", smtpCfg.Username, smtpCfg.Password, smtpCfg.Host)

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		// SECURITY: Don't include email address in error message (PII/GDPR)
		return fmt.Errorf("sending email via SMTP: %w", err)
	}
	return nil
}

// StartNewsletterScheduler starts a background goroutine that checks for due newsletters.
func (h *BlogHandler) StartNewsletterScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.checkDueNewsletters(ctx)
			}
		}
	}()
	h.logger.Info("newsletter scheduler started")
}

func (h *BlogHandler) checkDueNewsletters(ctx context.Context) {
	due, err := h.store.DueNewsletters(ctx)
	if err != nil {
		h.logger.Error("checking due newsletters", "error", err)
		return
	}
	// Send due newsletters concurrently (each in its own goroutine)
	for i := range due {
		n := due[i]
		h.logger.Info("sending scheduled newsletter", "id", n.ID, "subject", n.Subject)
		go h.sendNewsletter(ctx, n.ID)
	}
}

// HandleNewsletterTrackOpen records an email open via a tracking pixel.
func (h *BlogHandler) HandleNewsletterTrackOpen(w http.ResponseWriter, r *http.Request) {
	nID, _ := strconv.ParseInt(r.URL.Query().Get("n"), 10, 64)
	sID, _ := strconv.ParseInt(r.URL.Query().Get("s"), 10, 64)

	if nID > 0 && sID > 0 {
		_ = h.store.RecordNewsletterEvent(r.Context(), nID, sID, "open", "")
	}

	// Return 1x1 transparent GIF
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	transparentGIF := []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00,
		0x01, 0x00, 0x80, 0x00, 0x00, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00,
		0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44, 0x01, 0x00, 0x3b}
	w.Write(transparentGIF)
}

// HandleNewsletterTrackClick records a link click and redirects.
func (h *BlogHandler) HandleNewsletterTrackClick(w http.ResponseWriter, r *http.Request) {
	nID, _ := strconv.ParseInt(r.URL.Query().Get("n"), 10, 64)
	sID, _ := strconv.ParseInt(r.URL.Query().Get("s"), 10, 64)
	targetURL := r.URL.Query().Get("url")

	if targetURL == "" {
		http.Error(w, "Missing URL", http.StatusBadRequest)
		return
	}

	// Validate URL to prevent open redirect
	parsed, err := url.Parse(targetURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// SECURITY: Require valid newsletter ID and subscriber ID to prevent
	// abuse as a generic open redirect. Both must be positive.
	if nID <= 0 || sID <= 0 {
		http.Error(w, "Invalid newsletter or subscriber ID", http.StatusBadRequest)
		return
	}

	_ = h.store.RecordNewsletterEvent(r.Context(), nID, sID, "click", targetURL)

	http.Redirect(w, r, targetURL, http.StatusTemporaryRedirect)
}

// HandleAdminAPINewsletterAnalytics returns analytics for a specific newsletter.
func (h *BlogHandler) HandleAdminAPINewsletterAnalytics(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid newsletter ID", http.StatusBadRequest)
		return
	}

	analytics, err := h.store.GetNewsletterAnalytics(r.Context(), id)
	if err != nil {
		jsonError(w, "Failed to load analytics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analytics)
}

// HandleAdminAPICreateABTest creates a B variant for an existing newsletter.
func (h *BlogHandler) HandleAdminAPICreateABTest(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	idStr := r.PathValue("id")
	parentID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid newsletter ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	variant, err := h.store.CreateNewsletterVariant(r.Context(), parentID, "B", req.Subject, req.Body)
	if err != nil {
		jsonError(w, "Failed to create variant", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(variant)
}

// HandleAdminAPIRetryNewsletter retries sending a newsletter to failed recipients only.
func (h *BlogHandler) HandleAdminAPIRetryNewsletter(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid newsletter ID", http.StatusBadRequest)
		return
	}

	smtpCfg := h.cfg.GetSMTP()
	if !smtpCfg.Enabled || smtpCfg.Host == "" {
		jsonError(w, "SMTP is not configured", http.StatusBadRequest)
		return
	}

	// Run retry in background
	go func() {
		sent, failed := h.sendNewsletterRetry(context.Background(), id)
		h.logger.Info("newsletter retry result", "newsletter_id", id, "sent", sent, "still_failed", failed)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "retrying"})
}

// rewriteLinksForTracking replaces href URLs in HTML with click-tracking redirects.
func rewriteLinksForTracking(htmlContent, baseURL string, newsletterID, subscriberID int64) string {
	// Match href="http(s)://..." in the HTML
	result := htmlContent
	offset := 0

	// Simple approach: find all href="http..." and wrap them
	for {
		idx := strings.Index(result[offset:], `href="http`)
		if idx < 0 {
			break
		}
		idx += offset
		// Find the closing quote
		start := idx + 6 // after href="
		end := strings.Index(result[start:], `"`)
		if end < 0 {
			break
		}
		end += start

		originalURL := result[start:end]
		// Don't track unsubscribe links
		if strings.Contains(originalURL, "/api/unsubscribe") {
			offset = end + 1
			continue
		}

		trackURL := fmt.Sprintf("%s/api/newsletter/click?n=%d&s=%d&url=%s",
			baseURL, newsletterID, subscriberID, url.QueryEscape(originalURL))
		result = result[:start] + trackURL + result[end:]
		offset = start + len(trackURL) + 1 // skip past the rewritten URL
	}

	return result
}

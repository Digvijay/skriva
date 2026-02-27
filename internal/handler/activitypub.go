// Package handler — ActivityPub protocol implementation for fediverse integration.
// Implements: Webfinger, Actor, Inbox (Follow/Undo/Create), Outbox, HTTP Signatures.
package handler

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Digvijay/skriva/internal/config"
	"github.com/Digvijay/skriva/internal/content"
	"github.com/Digvijay/skriva/internal/store"
)

const (
	activityStreamsContext = "https://www.w3.org/ns/activitystreams"
	securityContext        = "https://w3id.org/security/v1"
)

// --- Webfinger ---

// HandleWebfinger implements RFC 7033 Webfinger for ActivityPub actor discovery.
func (h *BlogHandler) HandleWebfinger(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	if !site.ActivityPubEnabled {
		http.NotFound(w, r)
		return
	}

	resource := r.URL.Query().Get("resource")
	if resource == "" {
		http.Error(w, "Missing resource parameter", http.StatusBadRequest)
		return
	}

	baseURL, err := url.Parse(site.BaseURL)
	if err != nil || baseURL.Host == "" {
		http.Error(w, "Blog base_url not configured", http.StatusInternalServerError)
		return
	}

	// Accept acct:user@domain or https://domain/... format
	expectedAcct := fmt.Sprintf("acct:blog@%s", baseURL.Host)
	expectedURL := site.BaseURL + "/activitypub/actor"

	if resource != expectedAcct && resource != expectedURL {
		http.NotFound(w, r)
		return
	}

	resp := map[string]interface{}{
		"subject": expectedAcct,
		"aliases": []string{expectedURL},
		"links": []map[string]string{
			{
				"rel":  "self",
				"type": "application/activity+json",
				"href": expectedURL,
			},
			{
				"rel":  "http://webfinger.net/rel/profile-page",
				"type": "text/html",
				"href": site.BaseURL,
			},
		},
	}

	w.Header().Set("Content-Type", "application/jrd+json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(resp)
}

// --- Actor ---

// HandleActivityPubActor returns the ActivityPub Actor object.
func (h *BlogHandler) HandleActivityPubActor(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	if !site.ActivityPubEnabled {
		http.NotFound(w, r)
		return
	}

	secrets := h.cfg.GetSecrets()
	actorURL := site.BaseURL + "/activitypub/actor"

	actor := map[string]interface{}{
		"@context":          []string{activityStreamsContext, securityContext},
		"id":                actorURL,
		"type":              "Person",
		"preferredUsername": "blog",
		"name":              site.Author.Name,
		"summary":           site.Tagline,
		"url":               site.BaseURL,
		"inbox":             site.BaseURL + "/activitypub/inbox",
		"outbox":            site.BaseURL + "/activitypub/outbox",
		"followers":         site.BaseURL + "/activitypub/followers",
		"endpoints": map[string]string{
			"sharedInbox": site.BaseURL + "/activitypub/inbox",
		},
	}

	if site.Author.Avatar != "" {
		avatarURL := site.BaseURL + site.Author.Avatar
		actor["icon"] = map[string]string{
			"type":      "Image",
			"mediaType": "image/jpeg",
			"url":       avatarURL,
		}
	}

	// Add public key for HTTP Signature verification
	if secrets.ActivityPubPublicKey != "" {
		actor["publicKey"] = map[string]string{
			"id":           actorURL + "#main-key",
			"owner":        actorURL,
			"publicKeyPem": secrets.ActivityPubPublicKey,
		}
	}

	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	json.NewEncoder(w).Encode(actor)
}

// --- Inbox ---

// HandleActivityPubInbox receives incoming ActivityPub activities.
func (h *BlogHandler) HandleActivityPubInbox(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	if !site.ActivityPubEnabled {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var activity map[string]interface{}
	if err := json.Unmarshal(body, &activity); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Verify HTTP Signature — reject activities without valid signatures
	if sig := r.Header.Get("Signature"); sig != "" {
		if err := h.verifyHTTPSignature(r, body); err != nil {
			h.logger.Warn("HTTP Signature verification failed, rejecting", "error", err)
			http.Error(w, "Invalid HTTP Signature", http.StatusUnauthorized)
			return
		}
	} else {
		h.logger.Warn("activitypub inbox: missing HTTP Signature header, rejecting")
		http.Error(w, "HTTP Signature required", http.StatusUnauthorized)
		return
	}

	activityType, _ := activity["type"].(string)
	h.logger.Info("activitypub inbox", "type", activityType)

	switch activityType {
	case "Follow":
		h.handleFollow(w, r, activity)
	case "Undo":
		h.handleUndo(w, r, activity)
	case "Create":
		h.handleCreate(w, r, activity)
	case "Delete":
		// Acknowledge but no action needed for most delete activities
		w.WriteHeader(http.StatusAccepted)
	case "Like", "Announce":
		// Acknowledge likes and boosts
		w.WriteHeader(http.StatusAccepted)
	default:
		h.logger.Info("unhandled activity type", "type", activityType)
		w.WriteHeader(http.StatusAccepted)
	}
}

func (h *BlogHandler) handleFollow(w http.ResponseWriter, r *http.Request, activity map[string]interface{}) {
	actorURL, _ := activity["actor"].(string)
	if actorURL == "" {
		http.Error(w, "Missing actor", http.StatusBadRequest)
		return
	}

	// Fetch the actor to get their inbox URL
	actorInfo, err := h.fetchActor(actorURL)
	if err != nil {
		h.logger.Error("fetching follow actor", "actor", actorURL, "error", err)
		http.Error(w, "Failed to fetch actor", http.StatusBadRequest)
		return
	}

	inboxURL, _ := actorInfo["inbox"].(string)
	name, _ := actorInfo["preferredUsername"].(string)
	if name == "" {
		name, _ = actorInfo["name"].(string)
	}
	sharedInbox := ""
	if endpoints, ok := actorInfo["endpoints"].(map[string]interface{}); ok {
		sharedInbox, _ = endpoints["sharedInbox"].(string)
	}

	if err := h.store.AddActivityPubFollower(r.Context(), actorURL, inboxURL, sharedInbox, name); err != nil {
		h.logger.Error("storing follower", "actor", actorURL, "error", err)
	}

	// Send Accept activity back
	site := h.cfg.GetSite()
	myActor := site.BaseURL + "/activitypub/actor"
	accept := map[string]interface{}{
		"@context": activityStreamsContext,
		"id":       myActor + "#accept-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"type":     "Accept",
		"actor":    myActor,
		"object":   activity,
	}

	go h.deliverActivity(inboxURL, accept)

	h.logger.Info("accepted follow", "actor", actorURL, "name", name)
	w.WriteHeader(http.StatusAccepted)
}

func (h *BlogHandler) handleUndo(w http.ResponseWriter, r *http.Request, activity map[string]interface{}) {
	object, ok := activity["object"].(map[string]interface{})
	if !ok {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	objectType, _ := object["type"].(string)
	if objectType == "Follow" {
		actorURL, _ := activity["actor"].(string)
		if actorURL != "" {
			_ = h.store.RemoveActivityPubFollower(r.Context(), actorURL)
			h.logger.Info("unfollowed", "actor", actorURL)
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *BlogHandler) handleCreate(w http.ResponseWriter, r *http.Request, activity map[string]interface{}) {
	// Handle incoming replies — store as webmentions
	object, ok := activity["object"].(map[string]interface{})
	if !ok {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	objectType, _ := object["type"].(string)
	if objectType != "Note" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Check if this is a reply to one of our posts
	inReplyTo, _ := object["inReplyTo"].(string)
	if inReplyTo == "" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	site := h.cfg.GetSite()
	if !strings.HasPrefix(inReplyTo, site.BaseURL+"/") {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Extract slug from URL
	slug := strings.TrimPrefix(inReplyTo, site.BaseURL+"/")
	slug = strings.Split(slug, "?")[0]
	slug = strings.Split(slug, "#")[0]

	// Extract author info
	actorURL, _ := activity["actor"].(string)
	contentHTML, _ := object["content"].(string)
	// SECURITY: Sanitize HTML from remote AP servers to prevent stored XSS
	contentHTML = sanitizeUntrustedHTML(contentHTML)
	authorName := ""

	// Try to get author info from attributedTo
	if attributed, ok := object["attributedTo"].(string); ok {
		actorURL = attributed
	}
	if actorInfo, err := h.fetchActor(actorURL); err == nil {
		authorName, _ = actorInfo["preferredUsername"].(string)
		if authorName == "" {
			authorName, _ = actorInfo["name"].(string)
		}
	}
	if authorName == "" {
		authorName = "Fediverse User"
	}

	source, _ := object["url"].(string)
	if source == "" {
		source, _ = object["id"].(string)
	}

	wm := &store.Webmention{
		Source:      source,
		Target:      inReplyTo,
		PostSlug:    slug,
		AuthorName:  authorName,
		AuthorURL:   actorURL,
		Content:     contentHTML,
		MentionType: "reply",
		Verified:    true, // ActivityPub delivery = verified
	}

	if err := h.store.SaveWebmention(r.Context(), wm); err != nil {
		h.logger.Error("saving AP reply as webmention", "error", err)
	} else {
		h.logger.Info("received AP reply", "from", authorName, "post", slug)
	}

	w.WriteHeader(http.StatusAccepted)
}

// --- Outbox ---

// HandleActivityPubOutbox returns the outbox collection (published posts as activities).
func (h *BlogHandler) HandleActivityPubOutbox(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	if !site.ActivityPubEnabled {
		http.NotFound(w, r)
		return
	}

	posts := h.loader.Posts()

	items := make([]map[string]interface{}, 0, len(posts))
	for i := range posts {
		postURL := site.BaseURL + "/" + posts[i].Slug
		items = append(items, map[string]interface{}{
			"type":      "Create",
			"actor":     site.BaseURL + "/activitypub/actor",
			"published": posts[i].Date.Format(time.RFC3339),
			"object": map[string]interface{}{
				"type":         "Note",
				"id":           postURL,
				"url":          postURL,
				"attributedTo": site.BaseURL + "/activitypub/actor",
				"content":      "<p>" + html.EscapeString(posts[i].Description) + "</p><p><a href=\"" + html.EscapeString(postURL) + "\">" + html.EscapeString(posts[i].Title) + "</a></p>",
				"name":         posts[i].Title,
				"published":    posts[i].Date.Format(time.RFC3339),
				"to":           []string{"https://www.w3.org/ns/activitystreams#Public"},
			},
			"to": []string{"https://www.w3.org/ns/activitystreams#Public"},
		})
	}

	outbox := map[string]interface{}{
		"@context":     activityStreamsContext,
		"id":           site.BaseURL + "/activitypub/outbox",
		"type":         "OrderedCollection",
		"totalItems":   len(items),
		"orderedItems": items,
	}

	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	json.NewEncoder(w).Encode(outbox)
}

// --- Followers Collection ---

// HandleActivityPubFollowers returns the followers collection.
func (h *BlogHandler) HandleActivityPubFollowers(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	if !site.ActivityPubEnabled {
		http.NotFound(w, r)
		return
	}

	followers, err := h.store.AllActivityPubFollowers(r.Context())
	if err != nil {
		h.logger.Error("loading AP followers", "error", err)
		followers = nil
	}

	items := make([]string, 0, len(followers))
	for _, f := range followers {
		items = append(items, f.ActorURL)
	}

	collection := map[string]interface{}{
		"@context":     activityStreamsContext,
		"id":           site.BaseURL + "/activitypub/followers",
		"type":         "OrderedCollection",
		"totalItems":   len(items),
		"orderedItems": items,
	}

	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	json.NewEncoder(w).Encode(collection)
}

// --- HTTP Signatures ---

// GenerateActivityPubKeys generates an RSA keypair and stores it in secrets.
func (h *BlogHandler) GenerateActivityPubKeys() error {
	secrets := h.cfg.GetSecrets()
	if secrets.ActivityPubPrivateKey != "" {
		return nil // Already generated
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generating RSA key: %w", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	pubASN1, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("marshaling public key: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})

	secrets.ActivityPubPrivateKey = string(privPEM)
	secrets.ActivityPubPublicKey = string(pubPEM)

	if err := h.cfg.UpdateSecrets(secrets); err != nil {
		return fmt.Errorf("saving AP keys: %w", err)
	}

	h.logger.Info("generated ActivityPub RSA keypair")
	return nil
}

func (h *BlogHandler) getPrivateKey() (*rsa.PrivateKey, error) {
	secrets := h.cfg.GetSecrets()
	block, _ := pem.Decode([]byte(secrets.ActivityPubPrivateKey))
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in ActivityPub private key")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// signRequest signs an HTTP request with HTTP Signatures (draft-cavage).
func (h *BlogHandler) signRequest(req *http.Request, body []byte) error {
	privKey, err := h.getPrivateKey()
	if err != nil {
		return fmt.Errorf("loading private key: %w", err)
	}

	site := h.cfg.GetSite()
	keyID := site.BaseURL + "/activitypub/actor#main-key"

	now := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", now)
	req.Header.Set("Host", req.URL.Host)

	// Compute digest of body
	bodyHash := sha256.Sum256(body)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(bodyHash[:])
	req.Header.Set("Digest", digest)

	// Build string to sign
	headers := []string{"(request-target)", "host", "date", "digest"}
	var sigString strings.Builder
	for i, h := range headers {
		if i > 0 {
			sigString.WriteString("\n")
		}
		switch h {
		case "(request-target)":
			sigString.WriteString("(request-target): ")
			sigString.WriteString(strings.ToLower(req.Method) + " " + req.URL.RequestURI())
		case "host":
			sigString.WriteString("host: " + req.URL.Host)
		case "date":
			sigString.WriteString("date: " + now)
		case "digest":
			sigString.WriteString("digest: " + digest)
		}
	}

	hashed := sha256.Sum256([]byte(sigString.String()))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hashed[:])
	if err != nil {
		return fmt.Errorf("signing request: %w", err)
	}

	//nolint:gocritic // sprintfQuotedString: HTTP Signature spec requires literal double-quote delimiters, not Go %q escaping
	sigHeader := fmt.Sprintf(`keyId="%s",algorithm="rsa-sha256",headers="%s",signature="%s"`,
		keyID, strings.Join(headers, " "), base64.StdEncoding.EncodeToString(signature))
	req.Header.Set("Signature", sigHeader)

	return nil
}

// verifyHTTPSignature validates an incoming HTTP Signature on ActivityPub inbox requests.
func (h *BlogHandler) verifyHTTPSignature(r *http.Request, body []byte) error {
	sigHeader := r.Header.Get("Signature")
	if sigHeader == "" {
		return fmt.Errorf("no Signature header")
	}

	// Parse Signature header fields: keyId, algorithm, headers, signature
	params := make(map[string]string)
	for _, part := range strings.Split(sigHeader, ",") {
		part = strings.TrimSpace(part)
		eqIdx := strings.Index(part, "=")
		if eqIdx < 0 {
			continue
		}
		key := strings.TrimSpace(part[:eqIdx])
		val := strings.Trim(strings.TrimSpace(part[eqIdx+1:]), `"`)
		params[key] = val
	}

	keyID := params["keyId"]
	sigB64 := params["signature"]
	headersStr := params["headers"]

	if keyID == "" || sigB64 == "" {
		return fmt.Errorf("missing keyId or signature in Signature header")
	}

	// Fetch the actor's public key
	// keyId is typically "https://example.com/users/alice#main-key"
	actorURL := keyID
	if hashIdx := strings.Index(keyID, "#"); hashIdx > 0 {
		actorURL = keyID[:hashIdx]
	}

	actor, err := h.fetchActor(actorURL)
	if err != nil {
		return fmt.Errorf("fetching actor for key verification: %w", err)
	}

	// Extract public key PEM from actor
	var publicKeyPEM string
	if pk, ok := actor["publicKey"].(map[string]interface{}); ok {
		publicKeyPEM, _ = pk["publicKeyPem"].(string)
	}
	if publicKeyPEM == "" {
		return fmt.Errorf("no publicKeyPem found in actor %s", actorURL)
	}

	// Parse PEM to public key
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return fmt.Errorf("invalid PEM block in public key")
	}
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parsing public key: %w", err)
	}
	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not RSA")
	}

	// SECURITY: Require 'digest' in the signed headers to prevent body tampering
	signedHeaders := strings.Split(headersStr, " ")
	if len(signedHeaders) == 0 {
		signedHeaders = []string{"date"}
	}

	hasDigest := false
	for _, hdr := range signedHeaders {
		if strings.EqualFold(hdr, "digest") {
			hasDigest = true
			break
		}
	}
	if !hasDigest {
		return fmt.Errorf("signed headers must include 'digest' for body integrity")
	}

	var sigString strings.Builder
	for i, hdr := range signedHeaders {
		if i > 0 {
			sigString.WriteString("\n")
		}
		switch strings.ToLower(hdr) {
		case "(request-target)":
			sigString.WriteString("(request-target): ")
			sigString.WriteString(strings.ToLower(r.Method) + " " + r.URL.RequestURI())
		case "host":
			sigString.WriteString("host: " + r.Host)
		case "date":
			sigString.WriteString("date: " + r.Header.Get("Date"))
		case "digest":
			sigString.WriteString("digest: " + r.Header.Get("Digest"))
		case "content-type":
			sigString.WriteString("content-type: " + r.Header.Get("Content-Type"))
		default:
			sigString.WriteString(strings.ToLower(hdr) + ": " + r.Header.Get(hdr))
		}
	}

	// Verify digest if present
	if digestHeader := r.Header.Get("Digest"); digestHeader != "" {
		bodyHash := sha256.Sum256(body)
		expectedDigest := "SHA-256=" + base64.StdEncoding.EncodeToString(bodyHash[:])
		if digestHeader != expectedDigest {
			return fmt.Errorf("digest mismatch")
		}
	}

	// Verify signature
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("decoding signature: %w", err)
	}

	hashed := sha256.Sum256([]byte(sigString.String()))
	if err := rsa.VerifyPKCS1v15(rsaPubKey, crypto.SHA256, hashed[:], sigBytes); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// deliverActivity sends an activity to a remote inbox with HTTP Signatures.
func (h *BlogHandler) deliverActivity(inboxURL string, activity map[string]interface{}) {
	body, err := json.Marshal(activity)
	if err != nil {
		h.logger.Error("marshaling activity for delivery", "error", err)
		return
	}

	req, err := http.NewRequest("POST", inboxURL, bytes.NewReader(body))
	if err != nil {
		h.logger.Error("creating delivery request", "inbox", inboxURL, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/activity+json")

	if err := h.signRequest(req, body); err != nil {
		h.logger.Error("signing delivery request", "inbox", inboxURL, "error", err)
		return
	}

	client := safeHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("delivering activity", "inbox", inboxURL, "error", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		h.logger.Warn("activity delivery failed", "inbox", inboxURL, "status", resp.StatusCode)
	} else {
		h.logger.Info("activity delivered", "inbox", inboxURL, "status", resp.StatusCode)
	}
}

// fetchActor retrieves a remote ActivityPub actor document.
func (h *BlogHandler) fetchActor(actorURL string) (map[string]interface{}, error) {
	// SECURITY: Validate URL to prevent SSRF to internal/private IPs
	if err := validateExternalURL(actorURL); err != nil {
		return nil, fmt.Errorf("invalid actor URL: %w", err)
	}

	req, err := http.NewRequest("GET", actorURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/activity+json, application/ld+json")

	client := safeHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching actor %s: %w", actorURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("actor %s returned status %d", actorURL, resp.StatusCode)
	}

	var actor map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&actor); err != nil {
		return nil, fmt.Errorf("decoding actor %s: %w", actorURL, err)
	}
	return actor, nil
}

// NotifyFollowersNewPost sends a Create activity to all followers when a new post is published.
func (h *BlogHandler) NotifyFollowersNewPost(post *content.Post) {
	site := h.cfg.GetSite()
	if !site.ActivityPubEnabled {
		return
	}

	secrets := h.cfg.GetSecrets()
	if secrets.ActivityPubPrivateKey == "" {
		return // AP not configured
	}

	if site.BaseURL == "" {
		return
	}

	followers, err := h.store.AllActivityPubFollowers(context.TODO())
	if err != nil || len(followers) == 0 {
		return
	}

	postURL := site.BaseURL + "/" + post.Slug
	actorURL := site.BaseURL + "/activitypub/actor"

	activity := map[string]interface{}{
		"@context":  activityStreamsContext,
		"id":        postURL + "#create-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"type":      "Create",
		"actor":     actorURL,
		"published": post.Date.Format(time.RFC3339),
		"to":        []string{"https://www.w3.org/ns/activitystreams#Public"},
		"cc":        []string{site.BaseURL + "/activitypub/followers"},
		"object": map[string]interface{}{
			"type":         "Note",
			"id":           postURL,
			"url":          postURL,
			"attributedTo": actorURL,
			"content":      "<p><strong>" + html.EscapeString(post.Title) + "</strong></p><p>" + html.EscapeString(post.Description) + "</p><p><a href=\"" + html.EscapeString(postURL) + "\">Read more</a></p>",
			"name":         post.Title,
			"published":    post.Date.Format(time.RFC3339),
			"to":           []string{"https://www.w3.org/ns/activitystreams#Public"},
			"cc":           []string{site.BaseURL + "/activitypub/followers"},
		},
	}

	// Deduplicate by shared inbox
	seen := make(map[string]bool)
	for _, f := range followers {
		inbox := f.InboxURL
		if f.SharedInboxURL != "" {
			inbox = f.SharedInboxURL
		}
		if seen[inbox] {
			continue
		}
		seen[inbox] = true
		go h.deliverActivity(inbox, activity)
	}

	h.logger.Info("notified followers of new post", "slug", post.Slug, "inboxes", len(seen))
}

// activitypub.go helper — reference html package to avoid import cycle
var _ = slog.Default // keep slog import alive
var _ config.SiteConfig
var _ store.ActivityPubFollower

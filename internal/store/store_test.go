package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := New(dir, logger)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- Comments ---

func TestAddAndGetComments(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	c := &Comment{PostSlug: "test-post", Author: "Alice", Email: "a@b.com", Content: "Great post!"}
	if err := s.AddComment(ctx, c); err != nil {
		t.Fatalf("AddComment() error: %v", err)
	}
	if c.ID == 0 {
		t.Error("comment ID should be set after insert")
	}

	// New comments are unapproved by default
	comments, err := s.CommentsByPost(ctx, "test-post")
	if err != nil {
		t.Fatalf("CommentsByPost() error: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("unapproved comments should not be returned, got %d", len(comments))
	}

	// Approve it
	if err := s.ApproveComment(ctx, c.ID); err != nil {
		t.Fatalf("ApproveComment() error: %v", err)
	}
	comments, _ = s.CommentsByPost(ctx, "test-post")
	if len(comments) != 1 {
		t.Errorf("expected 1 approved comment, got %d", len(comments))
	}
	if comments[0].Author != "Alice" {
		t.Errorf("author = %q, want Alice", comments[0].Author)
	}
}

func TestDeleteComment(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	c := &Comment{PostSlug: "x", Author: "Bob", Content: "Test"}
	s.AddComment(ctx, c)
	if err := s.DeleteComment(ctx, c.ID); err != nil {
		t.Fatalf("DeleteComment() error: %v", err)
	}
	// Delete again should fail
	if err := s.DeleteComment(ctx, c.ID); err == nil {
		t.Error("deleting nonexistent comment should fail")
	}
}

func TestUnapproveComment(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	c := &Comment{PostSlug: "x", Author: "Eve", Content: "Hi"}
	s.AddComment(ctx, c)
	s.ApproveComment(ctx, c.ID)
	s.UnapproveComment(ctx, c.ID)

	comments, _ := s.CommentsByPost(ctx, "x")
	if len(comments) != 0 {
		t.Error("unapproved comment should not appear in CommentsByPost")
	}
}

func TestPendingCommentCount(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.AddComment(ctx, &Comment{PostSlug: "a", Author: "A", Content: "1"})
	s.AddComment(ctx, &Comment{PostSlug: "b", Author: "B", Content: "2"})

	count, err := s.PendingCommentCount(ctx)
	if err != nil {
		t.Fatalf("PendingCommentCount() error: %v", err)
	}
	if count != 2 {
		t.Errorf("pending count = %d, want 2", count)
	}
}

func TestAllComments(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.AddComment(ctx, &Comment{PostSlug: "a", Author: "A", Content: "1"})
	s.AddComment(ctx, &Comment{PostSlug: "b", Author: "B", Content: "2"})

	all, err := s.AllComments(ctx)
	if err != nil {
		t.Fatalf("AllComments() error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("AllComments count = %d, want 2", len(all))
	}
}

// --- Page Views ---

func TestTrackView(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Two different visitors
	if err := s.TrackView(ctx, "post-1", "hash-a"); err != nil {
		t.Fatalf("TrackView() error: %v", err)
	}
	if err := s.TrackView(ctx, "post-1", "hash-b"); err != nil {
		t.Fatalf("TrackView() error: %v", err)
	}
	// Same visitor again (should deduplicate)
	s.TrackView(ctx, "post-1", "hash-a")

	views, err := s.ViewsByPost(ctx, "post-1")
	if err != nil {
		t.Fatalf("ViewsByPost() error: %v", err)
	}
	if views != 2 {
		t.Errorf("views = %d, want 2 (deduped)", views)
	}
}

// --- Reactions ---

func TestAddReaction(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	ok, err := s.AddReaction(ctx, "post-1", "like", "fp-1")
	if err != nil {
		t.Fatalf("AddReaction() error: %v", err)
	}
	if !ok {
		t.Error("first reaction should succeed")
	}

	likes, dislikes, err := s.ReactionsByPost(ctx, "post-1")
	if err != nil {
		t.Fatalf("ReactionsByPost() error: %v", err)
	}
	if likes != 1 || dislikes != 0 {
		t.Errorf("likes=%d dislikes=%d, want 1/0", likes, dislikes)
	}
}

// --- Subscribers ---

func TestSubscriberLifecycle(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	if err := s.AddSubscriber(ctx, "test@example.com", "token123"); err != nil {
		t.Fatalf("AddSubscriber() error: %v", err)
	}

	count, _ := s.SubscriberCount(ctx)
	if count != 0 {
		t.Error("unconfirmed subscriber should not be counted")
	}

	s.ConfirmSubscriber(ctx, "token123")
	count, _ = s.SubscriberCount(ctx)
	if count != 1 {
		t.Errorf("confirmed subscriber count = %d, want 1", count)
	}

	subs, _ := s.ConfirmedSubscribers(ctx)
	if len(subs) != 1 || subs[0].Email != "test@example.com" {
		t.Error("ConfirmedSubscribers should return the confirmed subscriber")
	}

	s.RemoveSubscriber(ctx, "token123")
	count, _ = s.SubscriberCount(ctx)
	if count != 0 {
		t.Error("subscriber should be removed")
	}
}

func TestAllSubscribers(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.AddSubscriber(ctx, "a@b.com", "t1")
	s.AddSubscriber(ctx, "c@d.com", "t2")

	subs, err := s.AllSubscribers(ctx)
	if err != nil {
		t.Fatalf("AllSubscribers() error: %v", err)
	}
	if len(subs) != 2 {
		t.Errorf("AllSubscribers count = %d, want 2", len(subs))
	}
}

func TestDeleteSubscriber(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.AddSubscriber(ctx, "del@me.com", "tok")
	subs, _ := s.AllSubscribers(ctx)
	if len(subs) != 1 {
		t.Fatal("setup failed")
	}
	s.DeleteSubscriber(ctx, subs[0].ID)
	subs, _ = s.AllSubscribers(ctx)
	if len(subs) != 0 {
		t.Error("subscriber should be deleted")
	}
}

// --- Newsletters ---

func TestNewsletterCRUD(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	nl, err := s.CreateNewsletter(ctx, "Test Subject", "Hello **world**")
	if err != nil {
		t.Fatalf("CreateNewsletter() error: %v", err)
	}
	if nl.Status != "draft" {
		t.Errorf("status = %q, want draft", nl.Status)
	}

	// Update
	if err := s.UpdateNewsletter(ctx, nl.ID, "Updated Subject", "New body"); err != nil {
		t.Fatalf("UpdateNewsletter() error: %v", err)
	}

	// Get
	got, err := s.GetNewsletter(ctx, nl.ID)
	if err != nil {
		t.Fatalf("GetNewsletter() error: %v", err)
	}
	if got.Subject != "Updated Subject" {
		t.Errorf("subject = %q, want 'Updated Subject'", got.Subject)
	}

	// List
	all, _ := s.AllNewsletters(ctx)
	if len(all) != 1 {
		t.Errorf("AllNewsletters count = %d, want 1", len(all))
	}

	// Delete
	s.DeleteNewsletter(ctx, nl.ID)
	all, _ = s.AllNewsletters(ctx)
	if len(all) != 0 {
		t.Error("newsletter should be deleted")
	}
}

// --- Webhooks ---

func TestWebhookCRUD(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	hook, err := s.AddWebhook(ctx, "post.created", "https://example.com/hook", "secret123")
	if err != nil {
		t.Fatalf("AddWebhook() error: %v", err)
	}
	if hook.Event != "post.created" {
		t.Errorf("event = %q, want post.created", hook.Event)
	}

	hooks, _ := s.AllWebhooks(ctx)
	if len(hooks) != 1 {
		t.Errorf("AllWebhooks count = %d, want 1", len(hooks))
	}

	byEvent, _ := s.WebhooksByEvent(ctx, "post.created")
	if len(byEvent) != 1 {
		t.Errorf("WebhooksByEvent count = %d, want 1", len(byEvent))
	}

	s.DeleteWebhook(ctx, hook.ID)
	hooks, _ = s.AllWebhooks(ctx)
	if len(hooks) != 0 {
		t.Error("webhook should be deleted")
	}
}

// --- API Tokens ---

func TestAPITokenCRUD(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	token, err := s.AddAPIToken(ctx, "Test Token", "hash123", "read,write")
	if err != nil {
		t.Fatalf("AddAPIToken() error: %v", err)
	}
	if token.Name != "Test Token" {
		t.Errorf("name = %q, want 'Test Token'", token.Name)
	}

	// Validate
	validated, err := s.ValidateAPIToken(ctx, "hash123")
	if err != nil {
		t.Fatalf("ValidateAPIToken() error: %v", err)
	}
	if validated.Scopes != "read,write" {
		t.Errorf("scopes = %q, want 'read,write'", validated.Scopes)
	}

	// Invalid token
	_, err = s.ValidateAPIToken(ctx, "wrong-hash")
	if err == nil {
		t.Error("invalid token should fail validation")
	}

	// List
	tokens, _ := s.AllAPITokens(ctx)
	if len(tokens) != 1 {
		t.Errorf("AllAPITokens count = %d, want 1", len(tokens))
	}

	// Delete
	s.DeleteAPIToken(ctx, token.ID)
	tokens, _ = s.AllAPITokens(ctx)
	if len(tokens) != 0 {
		t.Error("token should be deleted")
	}
}

// --- Post Revisions ---

func TestPostRevisions(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	if err := s.SavePostRevision(ctx, "my-post", "First Version", "content v1", `{}`); err != nil {
		t.Fatalf("SavePostRevision() error: %v", err)
	}
	s.SavePostRevision(ctx, "my-post", "Second Version", "content v2", `{"tags":["go"]}`)

	revs, err := s.PostRevisions(ctx, "my-post")
	if err != nil {
		t.Fatalf("PostRevisions() error: %v", err)
	}
	if len(revs) != 2 {
		t.Errorf("revision count = %d, want 2", len(revs))
	}
	// Newest first — actually depends on insert order with same timestamp
	// SQLite CURRENT_TIMESTAMP has second granularity, so both may have same time
	// Just verify we got 2 revisions with the right slugs
	if revs[0].PostSlug != "my-post" || revs[1].PostSlug != "my-post" {
		t.Error("revisions should belong to my-post")
	}

	rev, err := s.GetPostRevision(ctx, revs[0].ID)
	if err != nil {
		t.Fatalf("GetPostRevision() error: %v", err)
	}
	if rev.PostSlug != "my-post" {
		t.Errorf("revision slug = %q, want 'my-post'", rev.PostSlug)
	}
}

// --- Draft Shares ---

func TestDraftShareLifecycle(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	share, err := s.CreateDraftShare(ctx, "draft-post", "token-abc", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("CreateDraftShare() error: %v", err)
	}
	if share.PostSlug != "draft-post" {
		t.Errorf("slug = %q, want draft-post", share.PostSlug)
	}

	got, err := s.GetDraftShare(ctx, "token-abc")
	if err != nil {
		t.Fatalf("GetDraftShare() error: %v", err)
	}
	if got.PostSlug != "draft-post" {
		t.Errorf("retrieved slug = %q, want draft-post", got.PostSlug)
	}

	s.DeleteDraftShare(ctx, "draft-post")
	_, err = s.GetDraftShare(ctx, "token-abc")
	if err == nil {
		t.Error("share should be deleted")
	}
}

// --- ActivityPub Followers ---

func TestActivityPubFollowers(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.AddActivityPubFollower(ctx, "https://mastodon.social/@alice", "https://mastodon.social/inbox", "", "alice")

	count, _ := s.ActivityPubFollowerCount(ctx)
	if count != 1 {
		t.Errorf("follower count = %d, want 1", count)
	}

	followers, _ := s.AllActivityPubFollowers(ctx)
	if len(followers) != 1 || followers[0].Name != "alice" {
		t.Error("follower not found correctly")
	}

	s.RemoveActivityPubFollower(ctx, "https://mastodon.social/@alice")
	count, _ = s.ActivityPubFollowerCount(ctx)
	if count != 0 {
		t.Error("follower should be removed")
	}
}

// --- Webmentions ---

func TestWebmentionSaveAndQuery(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	wm := &Webmention{
		Source:      "https://other.blog/post",
		Target:      "https://my.blog/hello",
		PostSlug:    "hello",
		AuthorName:  "Bob",
		MentionType: "reply",
		Verified:    true,
	}
	if err := s.SaveWebmention(ctx, wm); err != nil {
		t.Fatalf("SaveWebmention() error: %v", err)
	}

	wms, err := s.WebmentionsByPost(ctx, "hello")
	if err != nil {
		t.Fatalf("WebmentionsByPost() error: %v", err)
	}
	if len(wms) != 1 || wms[0].AuthorName != "Bob" {
		t.Error("webmention not retrieved correctly")
	}
}

// --- Dashboard Stats ---

func TestGetDashboardStats(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.AddComment(ctx, &Comment{PostSlug: "a", Author: "A", Content: "Hi"})
	s.TrackView(ctx, "a", "visitor1")

	stats, err := s.GetDashboardStats(ctx)
	if err != nil {
		t.Fatalf("GetDashboardStats() error: %v", err)
	}
	if stats.TotalComments != 1 {
		t.Errorf("TotalComments = %d, want 1", stats.TotalComments)
	}
	if stats.TotalViews != 1 {
		t.Errorf("TotalViews = %d, want 1", stats.TotalViews)
	}
}

// --- Migration Status / Rollback ---

func TestMigrationStatus(t *testing.T) {
	s := setupTestStore(t)

	infos, err := s.MigrationStatus()
	if err != nil {
		t.Fatalf("MigrationStatus() error: %v", err)
	}
	if len(infos) == 0 {
		t.Fatal("expected at least one migration")
	}
	// All migrations should be applied after New()
	for _, info := range infos {
		if !info.Applied {
			t.Errorf("migration %d (%s) should be applied", info.Version, info.Name)
		}
		if info.AppliedAt == "" {
			t.Errorf("migration %d should have applied_at", info.Version)
		}
	}
	// First migration should be version 1
	if infos[0].Version != 1 || infos[0].Name != "initial_schema" {
		t.Errorf("first migration = v%d %q, want v1 initial_schema", infos[0].Version, infos[0].Name)
	}
}

func TestRollbackMigration(t *testing.T) {
	s := setupTestStore(t)

	infos, _ := s.MigrationStatus()
	lastVersion := infos[len(infos)-1].Version

	// Rollback the last migration
	if err := s.RollbackMigration(); err != nil {
		t.Fatalf("RollbackMigration() error: %v", err)
	}

	// Verify it was removed
	infos2, _ := s.MigrationStatus()
	for _, info := range infos2 {
		if info.Version == lastVersion && info.Applied {
			t.Errorf("migration %d should no longer be applied after rollback", lastVersion)
		}
	}

	// Rollback again should succeed (penultimate migration)
	if err := s.RollbackMigration(); err != nil {
		t.Fatalf("second RollbackMigration() error: %v", err)
	}
}

// --- PostStatsForSlug ---

func TestPostStatsForSlug(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	slug := "stats-post"

	// Track views
	s.TrackView(ctx, slug, "v1")
	s.TrackView(ctx, slug, "v2")
	s.TrackView(ctx, slug, "v3")

	// Add reactions
	s.AddReaction(ctx, slug, "like", "fp-1")
	s.AddReaction(ctx, slug, "like", "fp-2")
	s.AddReaction(ctx, slug, "dislike", "fp-3")

	stats, err := s.PostStatsForSlug(ctx, slug)
	if err != nil {
		t.Fatalf("PostStatsForSlug() error: %v", err)
	}
	if stats.Views != 3 {
		t.Errorf("views = %d, want 3", stats.Views)
	}
	if stats.Likes != 2 {
		t.Errorf("likes = %d, want 2", stats.Likes)
	}
	if stats.Dislikes != 1 {
		t.Errorf("dislikes = %d, want 1", stats.Dislikes)
	}
}

func TestPostStatsForSlugEmpty(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	stats, err := s.PostStatsForSlug(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("PostStatsForSlug() error: %v", err)
	}
	if stats.Views != 0 || stats.Likes != 0 || stats.Dislikes != 0 {
		t.Errorf("empty post stats = %+v, want all zeros", stats)
	}
}

// --- Passkey CRUD ---

func TestPasskeyCRUD(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	credID := []byte("cred-id-123")
	pubKey := []byte("public-key-456")
	aaguid := []byte("aaguid-789")

	p := &Passkey{
		Name:            "My Passkey",
		CredentialID:    credID,
		PublicKey:       pubKey,
		AttestationType: "none",
		AAGUID:          aaguid,
		SignCount:       0,
		Transports:      []string{"usb", "nfc"},
	}

	// Add
	if err := s.AddPasskey(ctx, p); err != nil {
		t.Fatalf("AddPasskey() error: %v", err)
	}
	if p.ID == 0 {
		t.Error("passkey ID should be set after insert")
	}

	// List
	passkeys, err := s.AllPasskeys(ctx)
	if err != nil {
		t.Fatalf("AllPasskeys() error: %v", err)
	}
	if len(passkeys) != 1 {
		t.Fatalf("AllPasskeys count = %d, want 1", len(passkeys))
	}
	if passkeys[0].Name != "My Passkey" {
		t.Errorf("name = %q, want 'My Passkey'", passkeys[0].Name)
	}
	if string(passkeys[0].CredentialID) != string(credID) {
		t.Error("credential ID mismatch")
	}
	if string(passkeys[0].PublicKey) != string(pubKey) {
		t.Error("public key mismatch")
	}
	if passkeys[0].AttestationType != "none" {
		t.Errorf("attestation type = %q, want 'none'", passkeys[0].AttestationType)
	}
	if len(passkeys[0].Transports) != 2 || passkeys[0].Transports[0] != "usb" {
		t.Errorf("transports = %v, want [usb nfc]", passkeys[0].Transports)
	}

	// Update sign count
	if err := s.UpdatePasskeySignCount(ctx, credID, 5); err != nil {
		t.Fatalf("UpdatePasskeySignCount() error: %v", err)
	}
	passkeys, _ = s.AllPasskeys(ctx)
	if passkeys[0].SignCount != 5 {
		t.Errorf("sign count = %d, want 5", passkeys[0].SignCount)
	}

	// Delete
	if err := s.DeletePasskey(ctx, p.ID); err != nil {
		t.Fatalf("DeletePasskey() error: %v", err)
	}
	passkeys, _ = s.AllPasskeys(ctx)
	if len(passkeys) != 0 {
		t.Error("passkey should be deleted")
	}

	// Delete nonexistent should error
	if err := s.DeletePasskey(ctx, 9999); err == nil {
		t.Error("deleting nonexistent passkey should fail")
	}
}

// --- Newsletter Lifecycle ---

func TestNewsletterLifecycle(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	nl, err := s.CreateNewsletter(ctx, "Lifecycle Test", "Body content")
	if err != nil {
		t.Fatalf("CreateNewsletter() error: %v", err)
	}
	if nl.Status != "draft" {
		t.Fatalf("initial status = %q, want draft", nl.Status)
	}

	// Schedule
	schedTime := time.Now().Add(1 * time.Hour)
	if err := s.ScheduleNewsletter(ctx, nl.ID, schedTime); err != nil {
		t.Fatalf("ScheduleNewsletter() error: %v", err)
	}
	got, _ := s.GetNewsletter(ctx, nl.ID)
	if got.Status != "scheduled" {
		t.Errorf("status after schedule = %q, want scheduled", got.Status)
	}
	if got.ScheduledAt == nil {
		t.Error("scheduled_at should be set")
	}

	// Unschedule
	if err := s.UnscheduleNewsletter(ctx, nl.ID); err != nil {
		t.Fatalf("UnscheduleNewsletter() error: %v", err)
	}
	got, _ = s.GetNewsletter(ctx, nl.ID)
	if got.Status != "draft" {
		t.Errorf("status after unschedule = %q, want draft", got.Status)
	}
	if got.ScheduledAt != nil {
		t.Error("scheduled_at should be nil after unschedule")
	}

	// MarkSending
	if err := s.MarkNewsletterSending(ctx, nl.ID); err != nil {
		t.Fatalf("MarkNewsletterSending() error: %v", err)
	}
	got, _ = s.GetNewsletter(ctx, nl.ID)
	if got.Status != "sending" {
		t.Errorf("status after MarkSending = %q, want sending", got.Status)
	}

	// Double-send should fail
	if err := s.MarkNewsletterSending(ctx, nl.ID); err == nil {
		t.Error("marking already-sending newsletter as sending should fail")
	}

	// MarkSent
	if err := s.MarkNewsletterSent(ctx, nl.ID, 10, 2); err != nil {
		t.Fatalf("MarkNewsletterSent() error: %v", err)
	}
	got, _ = s.GetNewsletter(ctx, nl.ID)
	if got.Status != "sent" {
		t.Errorf("status after MarkSent = %q, want sent", got.Status)
	}
	if got.SentCount != 10 || got.FailCount != 2 {
		t.Errorf("counts = %d/%d, want 10/2", got.SentCount, got.FailCount)
	}
	if got.SentAt == nil {
		t.Error("sent_at should be set")
	}
}

func TestMarkNewsletterFailed(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	nl, _ := s.CreateNewsletter(ctx, "Fail Test", "body")
	s.MarkNewsletterSending(ctx, nl.ID)
	if err := s.MarkNewsletterFailed(ctx, nl.ID, 3, 7); err != nil {
		t.Fatalf("MarkNewsletterFailed() error: %v", err)
	}
	got, _ := s.GetNewsletter(ctx, nl.ID)
	if got.Status != "failed" {
		t.Errorf("status = %q, want failed", got.Status)
	}
	if got.SentCount != 3 || got.FailCount != 7 {
		t.Errorf("counts = %d/%d, want 3/7", got.SentCount, got.FailCount)
	}
}

// --- Newsletter Send Tracking ---

func TestNewsletterSendTracking(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Setup: create newsletter and subscribers
	nl, _ := s.CreateNewsletter(ctx, "Track Test", "body")
	s.AddSubscriber(ctx, "a@test.com", "tok-a")
	s.ConfirmSubscriber(ctx, "tok-a")
	s.AddSubscriber(ctx, "b@test.com", "tok-b")
	s.ConfirmSubscriber(ctx, "tok-b")
	s.AddSubscriber(ctx, "c@test.com", "tok-c")
	s.ConfirmSubscriber(ctx, "tok-c")

	subs, _ := s.ConfirmedSubscribers(ctx)
	if len(subs) != 3 {
		t.Fatalf("expected 3 confirmed subscribers, got %d", len(subs))
	}

	// All should be pending initially
	pending, err := s.PendingNewsletterRecipients(ctx, nl.ID)
	if err != nil {
		t.Fatalf("PendingNewsletterRecipients() error: %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("pending = %d, want 3", len(pending))
	}

	// Record sends: a=sent, b=failed, c=sent
	s.RecordNewsletterSend(ctx, nl.ID, subs[0].ID, "sent", "")
	s.RecordNewsletterSend(ctx, nl.ID, subs[1].ID, "failed", "SMTP timeout")
	s.RecordNewsletterSend(ctx, nl.ID, subs[2].ID, "sent", "")

	// Check failed
	failed, err := s.FailedNewsletterRecipients(ctx, nl.ID)
	if err != nil {
		t.Fatalf("FailedNewsletterRecipients() error: %v", err)
	}
	if len(failed) != 1 {
		t.Errorf("failed = %d, want 1", len(failed))
	}
	if len(failed) > 0 && failed[0].Email != "b@test.com" {
		t.Errorf("failed email = %q, want b@test.com", failed[0].Email)
	}

	// Check pending (only b should remain since it failed, a and c are sent)
	pending, _ = s.PendingNewsletterRecipients(ctx, nl.ID)
	if len(pending) != 1 {
		t.Errorf("pending after sends = %d, want 1 (the failed one)", len(pending))
	}
}

func TestDueNewsletters(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Newsletter scheduled in the past = due
	nl1, _ := s.CreateNewsletter(ctx, "Past Due", "body1")
	pastTime := time.Now().Add(-1 * time.Hour)
	s.ScheduleNewsletter(ctx, nl1.ID, pastTime)

	// Newsletter scheduled in the future = not due
	nl2, _ := s.CreateNewsletter(ctx, "Future", "body2")
	futureTime := time.Now().Add(24 * time.Hour)
	s.ScheduleNewsletter(ctx, nl2.ID, futureTime)

	// Draft newsletter = not due
	s.CreateNewsletter(ctx, "Draft", "body3")

	due, err := s.DueNewsletters(ctx)
	if err != nil {
		t.Fatalf("DueNewsletters() error: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("due newsletters = %d, want 1", len(due))
	}
	if due[0].Subject != "Past Due" {
		t.Errorf("due newsletter subject = %q, want 'Past Due'", due[0].Subject)
	}
}

// --- Newsletter Analytics ---

func TestNewsletterAnalytics(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	nl, _ := s.CreateNewsletter(ctx, "Analytics Test", "body")
	// Mark as sent so sent_count is available
	s.MarkNewsletterSending(ctx, nl.ID)
	s.MarkNewsletterSent(ctx, nl.ID, 100, 0)

	// Record events
	s.RecordNewsletterEvent(ctx, nl.ID, 1, "open", "")
	s.RecordNewsletterEvent(ctx, nl.ID, 2, "open", "")
	s.RecordNewsletterEvent(ctx, nl.ID, 1, "open", "") // duplicate open from same subscriber
	s.RecordNewsletterEvent(ctx, nl.ID, 1, "click", "https://example.com/link1")
	s.RecordNewsletterEvent(ctx, nl.ID, 2, "click", "https://example.com/link1")
	s.RecordNewsletterEvent(ctx, nl.ID, 1, "click", "https://example.com/link2")

	analytics, err := s.GetNewsletterAnalytics(ctx, nl.ID)
	if err != nil {
		t.Fatalf("GetNewsletterAnalytics() error: %v", err)
	}
	if analytics.TotalSent != 100 {
		t.Errorf("TotalSent = %d, want 100", analytics.TotalSent)
	}
	if analytics.UniqueOpens != 2 {
		t.Errorf("UniqueOpens = %d, want 2", analytics.UniqueOpens)
	}
	if analytics.TotalOpens != 3 {
		t.Errorf("TotalOpens = %d, want 3", analytics.TotalOpens)
	}
	if analytics.UniqueClicks != 2 {
		t.Errorf("UniqueClicks = %d, want 2", analytics.UniqueClicks)
	}
	if analytics.TotalClicks != 3 {
		t.Errorf("TotalClicks = %d, want 3", analytics.TotalClicks)
	}
	if analytics.OpenRate != 2.0 {
		t.Errorf("OpenRate = %f, want 2.0", analytics.OpenRate)
	}
	if len(analytics.TopLinks) != 2 {
		t.Errorf("TopLinks count = %d, want 2", len(analytics.TopLinks))
	}
	if len(analytics.TopLinks) > 0 && analytics.TopLinks[0].URL != "https://example.com/link1" {
		t.Errorf("top link = %q, want https://example.com/link1", analytics.TopLinks[0].URL)
	}
}

// --- Newsletter Variants ---

func TestNewsletterVariants(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	parent, _ := s.CreateNewsletter(ctx, "Original Subject", "Original body")

	variant, err := s.CreateNewsletterVariant(ctx, parent.ID, "B", "Variant Subject", "Variant body")
	if err != nil {
		t.Fatalf("CreateNewsletterVariant() error: %v", err)
	}
	if variant.ID == 0 {
		t.Error("variant ID should be set")
	}
	if variant.Subject != "Variant Subject" {
		t.Errorf("variant subject = %q, want 'Variant Subject'", variant.Subject)
	}

	variants, err := s.NewsletterVariants(ctx, parent.ID)
	if err != nil {
		t.Fatalf("NewsletterVariants() error: %v", err)
	}
	if len(variants) != 2 {
		t.Errorf("variants count = %d, want 2 (parent + 1 variant)", len(variants))
	}
	// Parent should be first (ORDER BY id ASC)
	if len(variants) >= 2 {
		if variants[0].Subject != "Original Subject" {
			t.Errorf("first variant subject = %q, want 'Original Subject'", variants[0].Subject)
		}
		if variants[1].Subject != "Variant Subject" {
			t.Errorf("second variant subject = %q, want 'Variant Subject'", variants[1].Subject)
		}
	}
}

// --- Draft Share by Slug ---

func TestGetDraftShareBySlug(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	expires := time.Now().Add(24 * time.Hour)
	s.CreateDraftShare(ctx, "my-draft", "share-token", expires)

	got, err := s.GetDraftShareBySlug(ctx, "my-draft")
	if err != nil {
		t.Fatalf("GetDraftShareBySlug() error: %v", err)
	}
	if got.PostSlug != "my-draft" {
		t.Errorf("slug = %q, want my-draft", got.PostSlug)
	}
	if got.Token != "share-token" {
		t.Errorf("token = %q, want share-token", got.Token)
	}

	// Nonexistent slug
	_, err = s.GetDraftShareBySlug(ctx, "no-such-draft")
	if err == nil {
		t.Error("GetDraftShareBySlug for nonexistent slug should fail")
	}
}

// --- Webmention Extras ---

func TestWebmentionExtras(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	wm1 := &Webmention{
		Source: "https://a.com/1", Target: "https://my.blog/post1",
		PostSlug: "post1", AuthorName: "Alice", MentionType: "reply", Verified: true,
	}
	wm2 := &Webmention{
		Source: "https://b.com/2", Target: "https://my.blog/post2",
		PostSlug: "post2", AuthorName: "Bob", MentionType: "like", Verified: true,
	}
	wm3 := &Webmention{
		Source: "https://c.com/3", Target: "https://my.blog/post1",
		PostSlug: "post1", AuthorName: "Charlie", MentionType: "mention", Verified: false,
	}
	s.SaveWebmention(ctx, wm1)
	s.SaveWebmention(ctx, wm2)
	s.SaveWebmention(ctx, wm3)

	// AllWebmentions
	all, err := s.AllWebmentions(ctx)
	if err != nil {
		t.Fatalf("AllWebmentions() error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("AllWebmentions count = %d, want 3", len(all))
	}

	// WebmentionCount (only verified)
	count, err := s.WebmentionCount(ctx)
	if err != nil {
		t.Fatalf("WebmentionCount() error: %v", err)
	}
	if count != 2 {
		t.Errorf("WebmentionCount = %d, want 2 (verified only)", count)
	}

	// DeleteWebmention
	if err := s.DeleteWebmention(ctx, all[0].ID); err != nil {
		t.Fatalf("DeleteWebmention() error: %v", err)
	}
	all, _ = s.AllWebmentions(ctx)
	if len(all) != 2 {
		t.Errorf("after delete, count = %d, want 2", len(all))
	}

	// Delete nonexistent
	if err := s.DeleteWebmention(ctx, 9999); err == nil {
		t.Error("deleting nonexistent webmention should fail")
	}
}

// --- AP Follower Extras ---

func TestRemoveActivityPubFollowerByID(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.AddActivityPubFollower(ctx, "https://mastodon.social/@user1", "https://mastodon.social/inbox1", "", "user1")
	s.AddActivityPubFollower(ctx, "https://mastodon.social/@user2", "https://mastodon.social/inbox2", "", "user2")

	followers, _ := s.AllActivityPubFollowers(ctx)
	if len(followers) != 2 {
		t.Fatalf("expected 2 followers, got %d", len(followers))
	}

	// Remove by ID
	if err := s.RemoveActivityPubFollowerByID(ctx, followers[0].ID); err != nil {
		t.Fatalf("RemoveActivityPubFollowerByID() error: %v", err)
	}
	followers, _ = s.AllActivityPubFollowers(ctx)
	if len(followers) != 1 {
		t.Errorf("after remove, count = %d, want 1", len(followers))
	}
	if followers[0].Name != "user2" {
		t.Errorf("remaining follower = %q, want user2", followers[0].Name)
	}

	// Remove nonexistent
	if err := s.RemoveActivityPubFollowerByID(ctx, 9999); err == nil {
		t.Error("removing nonexistent follower should fail")
	}
}

// --- Webhook Extras ---

func TestToggleWebhook(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	hook, _ := s.AddWebhook(ctx, "post.updated", "https://hook.example.com", "secret")
	if !hook.Active {
		t.Fatal("webhook should be active by default")
	}

	// Disable
	if err := s.ToggleWebhook(ctx, hook.ID, false); err != nil {
		t.Fatalf("ToggleWebhook(false) error: %v", err)
	}

	// Should not appear in WebhooksByEvent (only active)
	byEvent, _ := s.WebhooksByEvent(ctx, "post.updated")
	if len(byEvent) != 0 {
		t.Error("disabled webhook should not appear in WebhooksByEvent")
	}

	// Should still appear in AllWebhooks
	all, _ := s.AllWebhooks(ctx)
	if len(all) != 1 {
		t.Fatal("disabled webhook should still appear in AllWebhooks")
	}
	if all[0].Active {
		t.Error("webhook should be inactive after toggle")
	}

	// Re-enable
	s.ToggleWebhook(ctx, hook.ID, true)
	byEvent, _ = s.WebhooksByEvent(ctx, "post.updated")
	if len(byEvent) != 1 {
		t.Error("re-enabled webhook should appear in WebhooksByEvent")
	}
}

// --- Audit Log ---

func TestAuditLog(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Log events
	if err := s.LogAuditEvent(ctx, "login", "admin logged in", "192.168.1.1"); err != nil {
		t.Fatalf("LogAuditEvent() error: %v", err)
	}
	if err := s.LogAuditEvent(ctx, "post.create", "created post hello-world", "192.168.1.1"); err != nil {
		t.Fatalf("LogAuditEvent() error: %v", err)
	}
	if err := s.LogAuditEvent(ctx, "settings.update", "changed theme", "10.0.0.1"); err != nil {
		t.Fatalf("LogAuditEvent() error: %v", err)
	}

	// Retrieve
	entries, err := s.AuditLogs(ctx, 10)
	if err != nil {
		t.Fatalf("AuditLogs() error: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("AuditLogs count = %d, want 3", len(entries))
	}
	// Newest first
	if len(entries) > 0 && entries[0].Event != "settings.update" {
		t.Errorf("newest entry event = %q, want settings.update", entries[0].Event)
	}
	if len(entries) > 0 && entries[0].IP != "10.0.0.1" {
		t.Errorf("newest entry IP = %q, want 10.0.0.1", entries[0].IP)
	}

	// Default limit: 0 gets clamped to 200
	entries2, _ := s.AuditLogs(ctx, 0)
	if len(entries2) != 3 {
		t.Errorf("AuditLogs(0) count = %d, want 3", len(entries2))
	}
}

func TestCleanupAuditLog(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Insert entries with past timestamps directly
	s.db.ExecContext(ctx,
		"INSERT INTO audit_log (event, detail, ip, created_at) VALUES (?, ?, ?, ?)",
		"old.event", "old detail", "1.2.3.4", time.Now().Add(-72*time.Hour),
	)
	s.LogAuditEvent(ctx, "recent.event", "fresh entry", "5.6.7.8")

	// Cleanup entries older than 48 hours
	deleted, err := s.CleanupAuditLog(ctx, 48*time.Hour)
	if err != nil {
		t.Fatalf("CleanupAuditLog() error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	entries, _ := s.AuditLogs(ctx, 100)
	if len(entries) != 1 {
		t.Errorf("remaining entries = %d, want 1", len(entries))
	}
	if len(entries) > 0 && entries[0].Event != "recent.event" {
		t.Errorf("remaining event = %q, want recent.event", entries[0].Event)
	}
}

// --- DB Accessor ---

func TestDBAccessor(t *testing.T) {
	s := setupTestStore(t)
	if s.DB() == nil {
		t.Error("DB() should not return nil")
	}
}

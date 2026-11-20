-- Create "audit_logs" table
CREATE TABLE "audit_logs" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "actor_id" uuid NULL, "action" character varying NOT NULL, "entity_type" character varying NOT NULL, "entity_id" uuid NOT NULL, "before" jsonb NULL, "after" jsonb NULL, "request_id" character varying NULL, PRIMARY KEY ("id"));
-- Create index "auditlog_actor_id" to table: "audit_logs"
CREATE INDEX "auditlog_actor_id" ON "audit_logs" ("actor_id");
-- Create index "auditlog_created_at" to table: "audit_logs"
CREATE INDEX "auditlog_created_at" ON "audit_logs" ("created_at");
-- Create index "auditlog_entity_type_entity_id" to table: "audit_logs"
CREATE INDEX "auditlog_entity_type_entity_id" ON "audit_logs" ("entity_type", "entity_id");
-- Create "venues" table
CREATE TABLE "venues" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "name" character varying NOT NULL, "city" character varying NOT NULL, "address" character varying NULL, "info" text NULL, "timezone" character varying NOT NULL DEFAULT 'Asia/Kolkata', "status" character varying NOT NULL DEFAULT 'active', "version" bigint NOT NULL DEFAULT 1, PRIMARY KEY ("id"));
-- Create index "venue_city" to table: "venues"
CREATE INDEX "venue_city" ON "venues" ("city");
-- Create index "venue_status" to table: "venues"
CREATE INDEX "venue_status" ON "venues" ("status");
-- Create "events" table
CREATE TABLE "events" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "title" character varying NULL, "description" text NULL, "doors_open_at" timestamptz NULL, "scheduled_start_at" timestamptz NULL, "expected_end_at" timestamptz NULL, "ticket_url" character varying NULL, "publish_state" character varying NOT NULL DEFAULT 'draft', "status" character varying NOT NULL DEFAULT 'scheduled', "version" bigint NOT NULL DEFAULT 1, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, "venue_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "events_venues_events" FOREIGN KEY ("venue_id") REFERENCES "venues" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- Create index "event_publish_state_status" to table: "events"
CREATE INDEX "event_publish_state_status" ON "events" ("publish_state", "status");
-- Create index "event_scheduled_start_at" to table: "events"
CREATE INDEX "event_scheduled_start_at" ON "events" ("scheduled_start_at");
-- Create index "event_venue_id" to table: "events"
CREATE INDEX "event_venue_id" ON "events" ("venue_id");
-- Create "artists" table
CREATE TABLE "artists" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "name" character varying NOT NULL, "slug" character varying NOT NULL, "status" character varying NOT NULL DEFAULT 'active', PRIMARY KEY ("id"));
-- Create index "artist_status" to table: "artists"
CREATE INDEX "artist_status" ON "artists" ("status");
-- Create index "artists_slug_key" to table: "artists"
CREATE UNIQUE INDEX "artists_slug_key" ON "artists" ("slug");
-- Create "lineup_entries" table
CREATE TABLE "lineup_entries" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "billing" character varying NOT NULL, "sort_order" bigint NOT NULL DEFAULT 0, "artist_id" uuid NOT NULL, "event_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "lineup_entries_artists_lineup_entries" FOREIGN KEY ("artist_id") REFERENCES "artists" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "lineup_entries_events_lineup" FOREIGN KEY ("event_id") REFERENCES "events" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- Create index "lineupentry_artist_id" to table: "lineup_entries"
CREATE INDEX "lineupentry_artist_id" ON "lineup_entries" ("artist_id");
-- Create index "lineupentry_event_id_artist_id" to table: "lineup_entries"
CREATE UNIQUE INDEX "lineupentry_event_id_artist_id" ON "lineup_entries" ("event_id", "artist_id");
-- Create "users" table
CREATE TABLE "users" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "name" character varying NOT NULL, "email" character varying NOT NULL, "password_hash" character varying NOT NULL, "role" character varying NOT NULL DEFAULT 'admin', "status" character varying NOT NULL DEFAULT 'active', "last_login_at" timestamptz NULL, PRIMARY KEY ("id"));
-- Create index "user_role_status" to table: "users"
CREATE INDEX "user_role_status" ON "users" ("role", "status");
-- Create index "users_email_key" to table: "users"
CREATE UNIQUE INDEX "users_email_key" ON "users" ("email");
-- Create "refresh_tokens" table
CREATE TABLE "refresh_tokens" ("id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "token_hash" character varying NOT NULL, "expires_at" timestamptz NOT NULL, "revoked_at" timestamptz NULL, "user_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "refresh_tokens_users_refresh_tokens" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- Create index "refresh_tokens_token_hash_key" to table: "refresh_tokens"
CREATE UNIQUE INDEX "refresh_tokens_token_hash_key" ON "refresh_tokens" ("token_hash");
-- Create index "refreshtoken_user_id" to table: "refresh_tokens"
CREATE INDEX "refreshtoken_user_id" ON "refresh_tokens" ("user_id");

-- Migration: Add part table to existing OpenCode sample DB for v3 tool call support
-- Apply: sqlite3 testdata/opencode_sample.db < testdata/opencode_part_migration.sql
-- Purpose: The part table stores individual message parts (tool calls, text, reasoning).
-- v3 behavioral heuristics need tool call data to detect loops, re-reads, overlaps, restarts.

CREATE TABLE IF NOT EXISTS "part" (
    "id" text PRIMARY KEY,
    "message_id" text NOT NULL,
    "session_id" text NOT NULL,
    "time_created" integer NOT NULL,
    "time_updated" integer NOT NULL,
    "data" text NOT NULL,
    CONSTRAINT "fk_part_message_id_message_id_fk" FOREIGN KEY ("message_id") REFERENCES "message"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "part_session_idx" ON "part" ("session_id");
CREATE INDEX IF NOT EXISTS "part_message_id_id_idx" ON "part" ("message_id", "id");

-- Insert tool call parts linked to existing messages.
-- Each part references a real message_id from the existing sample DB.
-- The data field follows the OpenCode part JSON format:
--   {"type":"tool","tool":"<name>","callID":"<id>","state":{"input":{...}}}
-- Replace 'msg_<hash>' with actual message IDs from the sample DB.

-- Example entries (message IDs must match existing messages in the sample DB):
-- INSERT INTO part (id, message_id, session_id, time_created, time_updated, data) VALUES
-- ('part_001', '<real_message_id_1>', '<real_session_id>', 1714600000000, 1714600000000, '{"type":"tool","tool":"read","callID":"call_001","state":{"input":{"filePath":"src/main.go"},"status":"completed","time":{"start":1714600000}}}'),
-- ('part_002', '<real_message_id_1>', '<real_session_id>', 1714600001000, 1714600001000, '{"type":"tool","tool":"read","callID":"call_002","state":{"input":{"filePath":"src/types.go"},"status":"completed","time":{"start":1714600001}}}'),
-- ('part_003', '<real_message_id_2>', '<real_session_id>', 1714600003000, 1714600003000, '{"type":"tool","tool":"write","callID":"call_003","state":{"input":{"filePath":"src/result.go","content":"package main\n"},"status":"completed","time":{"start":1714600003}}}');

-- The actual INSERT statements must use real message_id and session_id values from the existing
-- opencode_sample.db. Run this query first to find valid IDs:
--   SELECT m.id, m.session_id FROM message m LIMIT 10;

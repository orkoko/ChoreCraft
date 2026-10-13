-- Enable UUID generation if it doesn't exist
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create the choregroups table if it doesn't exist
CREATE TABLE IF NOT EXISTS choregroups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    cooperative_points INTEGER NOT NULL DEFAULT 0 CHECK (cooperative_points >= 0)
);

-- Create the users table if it doesn't exist
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    choregroup_id UUID NOT NULL REFERENCES choregroups(id) ON DELETE CASCADE,
    username VARCHAR(255) NOT NULL,
    password_hash TEXT NOT NULL,
    role VARCHAR(50) NOT NULL CHECK (role IN ('admin', 'user')),
    points INTEGER NOT NULL DEFAULT 0 CHECK (points >= 0),
    CONSTRAINT users_choregroup_username_key UNIQUE (choregroup_id, username)
);

-- Create the tasks table if it doesn't exist
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    choregroup_id UUID NOT NULL REFERENCES choregroups(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('individual', 'cooperative')),
    points_reward INTEGER NOT NULL,
    assigned_to_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'assigned'
);

-- Create the task_submissions table if it doesn't exist
CREATE TABLE IF NOT EXISTS task_submissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    submitted_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending_approval', 'approved', 'rejected')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create the rewards table if it doesn't exist
CREATE TABLE IF NOT EXISTS rewards (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    choregroup_id UUID NOT NULL REFERENCES choregroups(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    cost INTEGER NOT NULL CHECK (cost > 0),
    type VARCHAR(50) NOT NULL CHECK (type IN ('individual', 'cooperative')),
    assigned_to_user_id UUID REFERENCES users(id) ON DELETE SET NULL
);

-- Create the reward_purchases table if it doesn't exist
CREATE TABLE IF NOT EXISTS reward_purchases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reward_id UUID NOT NULL REFERENCES rewards(id) ON DELETE CASCADE,
    purchased_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending_approval', 'approved', 'fulfilled', 'rejected')),
    approvals JSONB
);

-- Add is_mandatory to tasks
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS is_mandatory BOOLEAN NOT NULL DEFAULT FALSE;

-- Create the icon_mappings table to store AI-generated emojis for keywords
CREATE TABLE IF NOT EXISTS icon_mappings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    keyword VARCHAR(100) NOT NULL UNIQUE,
    emoji VARCHAR(10) NOT NULL
);

-- Add expires_at to tasks for time-limited tasks
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP WITH TIME ZONE;

-- Migration to make username unique per choregroup instead of globally unique
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_choregroup_username_key;
ALTER TABLE users ADD CONSTRAINT users_choregroup_username_key UNIQUE (choregroup_id, username);

-- Migration to support user notifications read synchronization
ALTER TABLE users ADD COLUMN IF NOT EXISTS notifications_viewed BOOLEAN NOT NULL DEFAULT FALSE;

-- Web Push Notification subscriptions
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL UNIQUE,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Migration to support task dependencies
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS parent_task_id UUID REFERENCES tasks(id) ON DELETE SET NULL;

-- Migration to support custom uploaded icon URLs
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS icon_url TEXT;
ALTER TABLE rewards ADD COLUMN IF NOT EXISTS icon_url TEXT;

-- Migration to support household-level AI generated content language and custom instructions
ALTER TABLE choregroups ADD COLUMN IF NOT EXISTS ai_language VARCHAR(10) NOT NULL DEFAULT 'he';
ALTER TABLE choregroups ADD COLUMN IF NOT EXISTS ai_instructions TEXT NOT NULL DEFAULT '';

-- Migration to support daily bonuses and lucky wheel
CREATE TABLE IF NOT EXISTS user_daily_progress (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    choregroup_id UUID NOT NULL REFERENCES choregroups(id) ON DELETE CASCADE,
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    tasks_completed INTEGER NOT NULL DEFAULT 0,
    spins_earned INTEGER NOT NULL DEFAULT 0,
    spins_used INTEGER NOT NULL DEFAULT 0,
    double_coins_active BOOLEAN NOT NULL DEFAULT FALSE,
    double_coins_used BOOLEAN NOT NULL DEFAULT FALSE,
    surprise_claimed BOOLEAN NOT NULL DEFAULT FALSE,
    CONSTRAINT user_daily_progress_user_date_key UNIQUE (user_id, date)
);

ALTER TABLE user_daily_progress ADD COLUMN IF NOT EXISTS surprise_claimed BOOLEAN NOT NULL DEFAULT FALSE;

-- Migration to track first-time login welcome wheel spin for kids
ALTER TABLE users ADD COLUMN IF NOT EXISTS has_claimed_welcome_spin BOOLEAN NOT NULL DEFAULT FALSE;

-- Migration to support user profile images and avatars
ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_image_url TEXT;

-- Migration to support custom points background themes and lucky wheel surprise unlocks
ALTER TABLE users ADD COLUMN IF NOT EXISTS points_background VARCHAR(100) NOT NULL DEFAULT 'default';
ALTER TABLE users ADD COLUMN IF NOT EXISTS unlocked_backgrounds JSONB NOT NULL DEFAULT '["default", "autumn_leaves", "golden_stars"]'::jsonb;

-- Migration to support customizable avatar frames
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_frame VARCHAR(100) NOT NULL DEFAULT 'none';
ALTER TABLE users ADD COLUMN IF NOT EXISTS unlocked_avatar_frames JSONB NOT NULL DEFAULT '["none"]'::jsonb;

-- Ensure all existing and future admin accounts have full access to all cosmetic skins
UPDATE users
SET unlocked_avatar_frames = '["none", "golden_sparkle", "fire_flame", "cosmic_nebula", "emerald_nature", "rainbow_wave", "royal_crown", "neon_cyber"]'::jsonb,
    unlocked_backgrounds = '["default", "autumn_leaves", "golden_stars", "cherry_blossom", "cosmic_galaxy", "emerald_nature", "candy_party", "fire_dragon"]'::jsonb
WHERE role = 'admin';


-- Activity History & Audit Trail for Households
CREATE TABLE IF NOT EXISTS activity_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    choregroup_id UUID NOT NULL REFERENCES choregroups(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    user_username VARCHAR(255) NOT NULL DEFAULT '',
    target_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    target_username VARCHAR(255) NOT NULL DEFAULT '',
    activity_type VARCHAR(100) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_activity_history_group_created ON activity_history(choregroup_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_history_user ON activity_history(choregroup_id, user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_history_target ON activity_history(choregroup_id, target_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_history_type ON activity_history(choregroup_id, activity_type, created_at DESC);

-- Uploads binary storage table (PostgreSQL BYTEA repository)
CREATE TABLE IF NOT EXISTS uploads (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    data BYTEA NOT NULL,
    size_bytes BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_uploads_filename ON uploads(filename);






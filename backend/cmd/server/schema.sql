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

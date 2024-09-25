#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER"  <<-EOSQL
    CREATE DATABASE quiz_sessions;
    CREATE DATABASE quiz_scores;
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "quiz_sessions" <<-EOSQL
    CREATE TABLE sessions (
      session_id UUID PRIMARY KEY,
      quiz_master TEXT NOT NULL,
      start_time TIMESTAMP,
      end_time TIMESTAMP
    );

    CREATE TABLE sessions_users (
      session_id UUID NOT NULL,
      username TEXT NOT NULL,
      create_time TIMESTAMP NOT NULL,
      PRIMARY KEY (session_id, username),
      FOREIGN KEY (session_id) REFERENCES sessions(session_id)
    );

    CREATE TABLE sessions_questions (
      session_id UUID NOT NULL,
      question_id TEXT NOT NULL,
      start_time TIMESTAMP,
      end_time TIMESTAMP,
      expire_time TIMESTAMP,
      PRIMARY KEY (session_id, question_id),
      FOREIGN KEY (session_id) REFERENCES sessions(session_id)
    );
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "quiz_scores" <<-EOSQL
    CREATE TABLE scores (
      session_id TEXT NOT NULL,
      username TEXT NOT NULL,
      question_id TEXT NOT NULL,
      score NUMERIC NOT NULL,
      create_time TIMESTAMP NOT NULL,
      PRIMARY KEY (session_id, username, question_id)
    );
EOSQL

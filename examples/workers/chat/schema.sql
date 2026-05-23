CREATE TABLE IF NOT EXISTS defaultdb.public.ratel_chat_messages (
  actor_id STRING NOT NULL,
  timestamp INT8 NOT NULL,
  name STRING NOT NULL,
  message STRING NOT NULL,
  CONSTRAINT "primary" PRIMARY KEY (actor_id, timestamp)
);
